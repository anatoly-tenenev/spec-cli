// Package entitycheck judges a finished write candidate against its entity
// type: the builtin formats, uniqueness of id and slug in the workspace, the
// frontmatter fields the schema allows, the required fields and sections
// including conditional ones, and the section titles.
//
// add and update both run it over the whole document rather than over the
// part the caller touched, because both must refuse to leave the workspace
// non-conforming. That makes it one judgement, not two: a document add would
// create must be one update could arrive at, and the other way round.
//
// The one thing the two commands genuinely disagree on is what counts as a
// conflict. update is editing a document that already holds its own id and
// slug, so the document at OwnPath is not competition; add has no such
// document, and leaves OwnPath empty.
//
// Every issue is collected and the result is ordered, so one rejected write
// tells the caller everything that has to change, in a stable order.
package entitycheck

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/application/collections"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/entityids"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/issues"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/schemarules"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/writemodel"
	"github.com/anatoly-tenenev/spec-cli/internal/application/entitydoc"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Workspace is what the uniqueness checks are answered against: the documents
// already holding each id and each slug per type, and the path of the document
// being written, if it already exists.
type Workspace struct {
	EntitiesByID map[string][]writemodel.WorkspaceEntity
	SlugsByType  map[string]map[string][]writemodel.WorkspaceEntity
	OwnPath      string
}

// Check reports every issue the candidate has. pathIssues and refIssues come
// from the checks that ran earlier in the pipeline and are carried through so
// the caller reports one ordered set rather than several.
//
// It fills candidate.Sections as a side effect: the sections are parsed here
// to be checked, and the caller needs the same parse to write the document.
func Check(
	typeSpec writemodel.EntityTypeSpec,
	candidate *writemodel.Candidate,
	workspace Workspace,
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

	if _, ok := entityids.ParseSuffix(candidate.ID, typeSpec.IDPrefix); !ok {
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

	if workspace.hasConflict(workspace.EntitiesByID[candidate.ID]) {
		validationIssues = append(validationIssues, issues.New(
			"global.id_duplicate",
			fmt.Sprintf("id '%s' is duplicated", candidate.ID),
			"11.1",
			"frontmatter.id",
			candidate,
		))
	}
	if byType, exists := workspace.SlugsByType[candidate.Type]; exists {
		if workspace.hasConflict(byType[candidate.Slug]) {
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

	sections, duplicateLabels := entitydoc.ExtractSections(candidate.Body)
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

// hasConflict reports whether some document other than the one being written
// already holds the value. With OwnPath empty - the add case, where the
// document does not exist yet - any holder is a conflict.
func (w Workspace) hasConflict(holders []writemodel.WorkspaceEntity) bool {
	for _, entity := range holders {
		if w.OwnPath != "" && entity.PathAbs == w.OwnPath {
			continue
		}
		return true
	}
	return false
}
