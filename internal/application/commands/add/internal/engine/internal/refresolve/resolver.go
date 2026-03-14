package refresolve

import (
	"fmt"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/engine/internal/issues"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/model"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

func Resolve(
	typeSpec model.EntityTypeSpec,
	candidate *model.Candidate,
	snapshot model.Snapshot,
) (map[string]model.ResolvedRef, []domainvalidation.Issue) {
	resolved := map[string]model.ResolvedRef{}
	refIssues := make([]domainvalidation.Issue, 0)

	for _, fieldName := range typeSpec.MetaFieldOrder {
		fieldSpec := typeSpec.MetaFields[fieldName]
		if !fieldSpec.IsEntityRef {
			continue
		}

		rawValue, exists := candidate.Frontmatter[fieldName]
		if !exists {
			continue
		}

		targetID, ok := rawValue.(string)
		if !ok {
			refIssues = append(refIssues, issues.New(
				"meta.required_type_mismatch",
				fmt.Sprintf("field '%s' must be string target id", fieldName),
				"11.5",
				"frontmatter."+fieldName,
				candidate,
			))
			continue
		}

		targetID = strings.TrimSpace(targetID)
		if targetID == "" {
			continue
		}
		candidate.RefIDs[fieldName] = targetID

		targets := snapshot.EntitiesByID[targetID]
		if len(targets) == 0 {
			refIssues = append(refIssues, issues.New(
				"meta.entity_ref_target_missing",
				fmt.Sprintf("entity_ref '%s' points to missing id '%s'", fieldName, targetID),
				"11.5",
				"frontmatter."+fieldName,
				candidate,
			))
			continue
		}
		if len(targets) > 1 {
			refIssues = append(refIssues, issues.New(
				"meta.entity_ref_target_ambiguous",
				fmt.Sprintf("entity_ref '%s' points to ambiguous id '%s'", fieldName, targetID),
				"11.5",
				"frontmatter."+fieldName,
				candidate,
			))
			continue
		}

		target := targets[0]
		if len(fieldSpec.RefTypes) > 0 && !contains(fieldSpec.RefTypes, target.Type) {
			refIssues = append(refIssues, issues.New(
				"meta.entity_ref_type_mismatch",
				fmt.Sprintf("entity_ref '%s' points to type '%s' outside refTypes", fieldName, target.Type),
				"11.5",
				"frontmatter."+fieldName,
				candidate,
			))
			continue
		}

		resolved[fieldName] = model.ResolvedRef{
			Type:    target.Type,
			ID:      target.ID,
			Slug:    target.Slug,
			DirPath: target.DirPath,
			Meta:    target.Meta,
		}
	}

	return resolved, refIssues
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
