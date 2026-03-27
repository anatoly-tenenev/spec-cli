package validation

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/engine/internal/expr"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/engine/internal/issues"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/engine/internal/lookup"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/support"
	updateworkspace "github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/workspace"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func Validate(
	typeSpec model.EntityTypeSpec,
	candidate *model.Candidate,
	snapshot model.Snapshot,
	sourcePath string,
	pathIssues []domainvalidation.Issue,
	refIssues []domainvalidation.Issue,
) []domainvalidation.Issue {
	validationIssues := make([]domainvalidation.Issue, 0, len(pathIssues)+len(refIssues)+16)
	validationIssues = append(validationIssues, pathIssues...)
	validationIssues = append(validationIssues, refIssues...)

	if !slugPattern.MatchString(candidate.Slug) {
		validationIssues = append(validationIssues, issues.New(
			"builtin.slug_format_invalid",
			"slug must match ^[a-z0-9]+(?:-[a-z0-9]+)*$",
			"11.2",
			"frontmatter.slug",
			candidate,
		))
	}

	if _, ok := parseIDSuffix(candidate.ID, typeSpec.IDPrefix); !ok {
		validationIssues = append(validationIssues, issues.New(
			"builtin.id_format_invalid",
			fmt.Sprintf("id must match prefix '%s-<number>'", typeSpec.IDPrefix),
			"11.1",
			"frontmatter.id",
			candidate,
		))
	}

	if _, err := time.Parse("2006-01-02", candidate.CreatedDate); err != nil {
		validationIssues = append(validationIssues, issues.New(
			"builtin.date_format_invalid",
			"field 'createdDate' must be in YYYY-MM-DD format",
			"11.3",
			"frontmatter.createdDate",
			candidate,
		))
	}
	if _, err := time.Parse("2006-01-02", candidate.UpdatedDate); err != nil {
		validationIssues = append(validationIssues, issues.New(
			"builtin.date_format_invalid",
			"field 'updatedDate' must be in YYYY-MM-DD format",
			"11.4",
			"frontmatter.updatedDate",
			candidate,
		))
	}

	if hasIDConflict(snapshot.EntitiesByID[candidate.ID], sourcePath) {
		validationIssues = append(validationIssues, issues.New(
			"global.id_duplicate",
			fmt.Sprintf("id '%s' is duplicated", candidate.ID),
			"11.1",
			"frontmatter.id",
			candidate,
		))
	}
	if byType, exists := snapshot.SlugsByType[candidate.Type]; exists {
		if hasSlugConflict(byType[candidate.Slug], sourcePath) {
			validationIssues = append(validationIssues, issues.New(
				"global.slug_duplicate_by_type",
				fmt.Sprintf("slug '%s' is duplicated for type '%s'", candidate.Slug, candidate.Type),
				"11.2",
				"frontmatter.slug",
				candidate,
			))
		}
	}

	allowedFrontmatterKeys := map[string]struct{}{
		"type": {}, "id": {}, "slug": {}, "createdDate": {}, "updatedDate": {},
	}
	for _, fieldName := range typeSpec.MetaFieldOrder {
		allowedFrontmatterKeys[fieldName] = struct{}{}
	}
	for _, key := range support.SortedMapKeys(candidate.Frontmatter) {
		if _, ok := allowedFrontmatterKeys[key]; ok {
			continue
		}
		validationIssues = append(validationIssues, issues.New(
			"frontmatter.field_not_allowed",
			fmt.Sprintf("field '%s' is not allowed by schema", key),
			"12.3",
			"frontmatter."+key,
			candidate,
		))
	}

	lookupValues := lookup.Candidate{Candidate: candidate}
	for _, fieldName := range typeSpec.MetaFieldOrder {
		fieldSpec := typeSpec.MetaFields[fieldName]
		value, exists := candidate.Frontmatter[fieldName]

		required := fieldSpec.Required
		if fieldSpec.HasRequiredWhen {
			matches, evalErr := expr.Evaluate(fieldSpec.RequiredWhen, lookupValues)
			if evalErr != nil {
				validationIssues = append(validationIssues, issues.New(
					"meta.required_when_evaluation_failed",
					fmt.Sprintf("failed to evaluate required_when for field '%s'", fieldName),
					"11.5",
					"schema.meta.fields."+fieldName+".required_when",
					candidate,
				))
			} else if matches {
				required = true
			}
		}

		if required && !exists {
			validationIssues = append(validationIssues, issues.New(
				"meta.required_missing",
				fmt.Sprintf("required field '%s' is missing", fieldName),
				"11.5",
				"frontmatter."+fieldName,
				candidate,
			))
			continue
		}

		if !exists {
			continue
		}

		validationIssues = append(validationIssues, validateMetaFieldValue(fieldSpec, value, candidate)...)
	}

	sections, duplicateLabels := updateworkspace.ExtractSections(candidate.Body)
	candidate.Sections = map[string]string{}
	for label, content := range sections {
		candidate.Sections[label] = content.Body
	}
	for _, label := range duplicateLabels {
		validationIssues = append(validationIssues, issues.New(
			"content.section_label_duplicate",
			fmt.Sprintf("section label '%s' is duplicated", label),
			"12.2",
			"content.sections."+label,
			candidate,
		))
	}

	for _, sectionName := range typeSpec.SectionOrder {
		sectionSpec := typeSpec.Sections[sectionName]
		sectionContent, exists := sections[sectionName]

		required := sectionSpec.Required
		if sectionSpec.HasRequiredWhen {
			matches, evalErr := expr.Evaluate(sectionSpec.RequiredWhen, lookupValues)
			if evalErr != nil {
				validationIssues = append(validationIssues, issues.New(
					"content.required_when_evaluation_failed",
					fmt.Sprintf("failed to evaluate required_when for section '%s'", sectionName),
					"12.2",
					"schema.content.sections."+sectionName+".required_when",
					candidate,
				))
			} else if matches {
				required = true
			}
		}

		if required && !exists {
			validationIssues = append(validationIssues, issues.New(
				"content.required_missing",
				fmt.Sprintf("required content section '%s' is missing", sectionName),
				"12.2",
				"content.sections."+sectionName,
				candidate,
			))
			continue
		}

		if !exists {
			continue
		}

		if len(sectionSpec.Titles) > 0 && !contains(sectionSpec.Titles, sectionContent.Title) {
			validationIssues = append(validationIssues, issues.New(
				"content.section_title_mismatch",
				fmt.Sprintf("section '%s' title is not allowed by schema", sectionName),
				"12.2",
				"content.sections."+sectionName,
				candidate,
			))
		}
	}

	sort.SliceStable(validationIssues, func(i, j int) bool {
		if validationIssues[i].Code != validationIssues[j].Code {
			return validationIssues[i].Code < validationIssues[j].Code
		}
		if validationIssues[i].Field != validationIssues[j].Field {
			return validationIssues[i].Field < validationIssues[j].Field
		}
		return validationIssues[i].Message < validationIssues[j].Message
	})

	return validationIssues
}

func AsAppError(issuesList []domainvalidation.Issue) *domainerrors.AppError {
	return domainerrors.New(
		domainerrors.CodeValidationFailed,
		"updated entity failed validation",
		map[string]any{
			"validation": map[string]any{
				"issues": issuesList,
			},
		},
	)
}

func validateMetaFieldValue(fieldSpec model.MetaField, rawValue any, candidate *model.Candidate) []domainvalidation.Issue {
	issuesList := make([]domainvalidation.Issue, 0)
	value := support.NormalizeValue(rawValue)

	typeMismatch := func(expected string) {
		issuesList = append(issuesList, issues.New(
			"meta.required_type_mismatch",
			fmt.Sprintf("field '%s' must be %s", fieldSpec.Name, expected),
			"11.5",
			"frontmatter."+fieldSpec.Name,
			candidate,
		))
	}

	switch fieldSpec.Type {
	case "string":
		if _, ok := value.(string); !ok {
			typeMismatch("string")
		}
	case "integer":
		number, ok := support.NumberToFloat64(value)
		if !ok || number != float64(int(number)) {
			typeMismatch("integer")
		}
	case "number":
		if _, ok := support.NumberToFloat64(value); !ok {
			typeMismatch("number")
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			typeMismatch("boolean")
		}
	case "null":
		if value != nil {
			typeMismatch("null")
		}
	case "entityRef":
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			typeMismatch("non-empty string")
		}
	case "array":
		arr, ok := value.([]any)
		if !ok {
			typeMismatch("array")
			break
		}
		if fieldSpec.HasMinItems && len(arr) < fieldSpec.MinItems {
			issuesList = append(issuesList, issues.New(
				"meta.required_array_min_items",
				fmt.Sprintf("field '%s' requires at least %d items", fieldSpec.Name, fieldSpec.MinItems),
				"11.5",
				"frontmatter."+fieldSpec.Name,
				candidate,
			))
		}
		if fieldSpec.HasMaxItems && len(arr) > fieldSpec.MaxItems {
			issuesList = append(issuesList, issues.New(
				"meta.required_array_max_items",
				fmt.Sprintf("field '%s' allows at most %d items", fieldSpec.Name, fieldSpec.MaxItems),
				"11.5",
				"frontmatter."+fieldSpec.Name,
				candidate,
			))
		}
		if fieldSpec.UniqueItems {
			for i := 0; i < len(arr); i++ {
				for j := i + 1; j < len(arr); j++ {
					if support.LiteralEqual(arr[i], arr[j]) {
						issuesList = append(issuesList, issues.New(
							"meta.required_array_unique_items",
							fmt.Sprintf("field '%s' requires unique items", fieldSpec.Name),
							"11.5",
							"frontmatter."+fieldSpec.Name,
							candidate,
						))
						break
					}
				}
			}
		}
		if fieldSpec.HasItems {
			for _, item := range arr {
				if !isValueOfType(item, fieldSpec.ItemType) {
					issuesList = append(issuesList, issues.New(
						"meta.required_array_items_mismatch",
						fmt.Sprintf("field '%s' contains item with unsupported type", fieldSpec.Name),
						"11.5",
						"frontmatter."+fieldSpec.Name,
						candidate,
					))
					break
				}
			}
		}
	}

	if len(fieldSpec.Enum) > 0 {
		matched := false
		for _, enumValue := range fieldSpec.Enum {
			if support.LiteralEqual(enumValue, value) {
				matched = true
				break
			}
		}
		if !matched {
			issuesList = append(issuesList, issues.New(
				"meta.required_enum_mismatch",
				fmt.Sprintf("field '%s' value is outside enum", fieldSpec.Name),
				"11.5",
				"frontmatter."+fieldSpec.Name,
				candidate,
			))
		}
	}

	if fieldSpec.HasConst && !support.LiteralEqual(fieldSpec.Const, value) {
		issuesList = append(issuesList, issues.New(
			"meta.required_value_mismatch",
			fmt.Sprintf("field '%s' must match schema const", fieldSpec.Name),
			"11.5",
			"frontmatter."+fieldSpec.Name,
			candidate,
		))
	}

	return issuesList
}

func isValueOfType(value any, typeName string) bool {
	typeName = strings.TrimSpace(typeName)
	value = support.NormalizeValue(value)

	switch typeName {
	case "string":
		_, ok := value.(string)
		return ok
	case "integer":
		number, ok := support.NumberToFloat64(value)
		return ok && number == float64(int(number))
	case "number":
		_, ok := support.NumberToFloat64(value)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "null":
		return value == nil
	case "entityRef":
		text, ok := value.(string)
		return ok && strings.TrimSpace(text) != ""
	default:
		return false
	}
}

func parseIDSuffix(id string, prefix string) (int, bool) {
	expectedPrefix := prefix + "-"
	if !strings.HasPrefix(id, expectedPrefix) {
		return 0, false
	}

	rawSuffix := strings.TrimPrefix(id, expectedPrefix)
	if rawSuffix == "" {
		return 0, false
	}
	for _, ch := range rawSuffix {
		if ch < '0' || ch > '9' {
			return 0, false
		}
	}

	value := 0
	for _, ch := range rawSuffix {
		value = value*10 + int(ch-'0')
	}
	return value, true
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func hasIDConflict(existing []model.WorkspaceEntity, sourcePath string) bool {
	for _, entity := range existing {
		if entity.PathAbs == sourcePath {
			continue
		}
		return true
	}
	return false
}

func hasSlugConflict(existing []model.WorkspaceEntity, sourcePath string) bool {
	for _, entity := range existing {
		if entity.PathAbs == sourcePath {
			continue
		}
		return true
	}
	return false
}
