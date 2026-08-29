// Package validation checks the whole updated entity, not just the fields the
// patch touched: builtin formats, required fields including conditional ones,
// field types and constraints, and uniqueness across the workspace. Checking
// everything is what keeps an update from completing a document that was
// already non-conforming into a state no one validated.
//
// Uniqueness ignores the document being updated, so an entity does not
// collide with its own id or slug.
package validation

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/collections"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/issues"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/schemarules"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/model"
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
	evaluationContext map[string]any,
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
	for _, key := range collections.SortedMapKeys(candidate.Frontmatter) {
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

	for _, fieldName := range typeSpec.MetaFieldOrder {
		fieldSpec := typeSpec.MetaFields[fieldName]
		value, exists := candidate.Frontmatter[fieldName]

		required, requiredErr := schemarules.EvaluateRequired(fieldSpec.Required, fieldSpec.RequiredExpr, evaluationContext)
		if requiredErr != nil {
			validationIssues = append(validationIssues, issues.New(
				"meta.required_expression_evaluation_failed",
				fmt.Sprintf("failed to evaluate required for field '%s'", fieldName),
				"11.6",
				issues.PathOrDefault(fieldSpec.RequiredPath, "schema.meta.fields."+fieldName+".required"),
				candidate,
			))
			required = false
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

		validationIssues = append(validationIssues, schemarules.Check(fieldSpec, value, candidate, evaluationContext)...)
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

		required, requiredErr := schemarules.EvaluateRequired(sectionSpec.Required, sectionSpec.RequiredExpr, evaluationContext)
		if requiredErr != nil {
			validationIssues = append(validationIssues, issues.New(
				"content.required_expression_evaluation_failed",
				fmt.Sprintf("failed to evaluate required for section '%s'", sectionName),
				"11.6",
				issues.PathOrDefault(sectionSpec.RequiredPath, "schema.content.sections."+sectionName+".required"),
				candidate,
			))
			required = false
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

		if strings.TrimSpace(sectionSpec.Title) != "" && sectionContent.Title != sectionSpec.Title {
			validationIssues = append(validationIssues, issues.New(
				"content.section_title_mismatch",
				fmt.Sprintf("section '%s' title must exactly match '%s'", sectionName, sectionSpec.Title),
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
