package refresolve

import (
	"fmt"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/issues"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/writemodel"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

func Resolve(
	typeSpec writemodel.EntityTypeSpec,
	candidate *writemodel.Candidate,
	entitiesByID map[string][]writemodel.WorkspaceEntity,
) (map[string]writemodel.ResolvedRef, map[string][]writemodel.ResolvedRef, []domainvalidation.Issue) {
	resolved := map[string]writemodel.ResolvedRef{}
	resolvedArrays := map[string][]writemodel.ResolvedRef{}
	refIssues := make([]domainvalidation.Issue, 0)
	candidate.RefIDs = map[string]string{}
	candidate.RefIDArrays = map[string][]string{}

	for _, fieldName := range typeSpec.MetaFieldOrder {
		fieldSpec := typeSpec.MetaFields[fieldName]
		if !fieldSpec.IsEntityRef && !fieldSpec.IsEntityRefArray {
			continue
		}

		rawValue, exists := candidate.Frontmatter[fieldName]
		if !exists {
			continue
		}
		if fieldSpec.IsEntityRefArray {
			targetIDs, idOK := readRefIDArray(rawValue)
			if !idOK {
				refIssues = append(refIssues, issues.New(
					"meta.required_type_mismatch",
					fmt.Sprintf("field '%s' must be array of non-empty string target ids", fieldName),
					"11.5",
					"frontmatter."+fieldName,
					candidate,
				))
				continue
			}
			candidate.RefIDArrays[fieldName] = targetIDs

			resolvedItems := make([]writemodel.ResolvedRef, 0, len(targetIDs))
			hasRefIssue := false
			for idx, targetID := range targetIDs {
				targets := entitiesByID[targetID]
				itemField := fmt.Sprintf("frontmatter.%s[%d]", fieldName, idx)
				if len(targets) == 0 {
					refIssues = append(refIssues, issues.New(
						"meta.entityRef_target_missing",
						fmt.Sprintf("entityRef '%s[%d]' points to missing id '%s'", fieldName, idx, targetID),
						"11.5",
						itemField,
						candidate,
					))
					hasRefIssue = true
					continue
				}
				if len(targets) > 1 {
					refIssues = append(refIssues, issues.New(
						"meta.entityRef_target_ambiguous",
						fmt.Sprintf("entityRef '%s[%d]' points to ambiguous id '%s'", fieldName, idx, targetID),
						"11.5",
						itemField,
						candidate,
					))
					hasRefIssue = true
					continue
				}

				target := targets[0]
				if len(fieldSpec.ItemRefTypes) > 0 && !contains(fieldSpec.ItemRefTypes, target.Type) {
					refIssues = append(refIssues, issues.New(
						"meta.entityRef_type_mismatch",
						fmt.Sprintf("entityRef '%s[%d]' points to type '%s' outside refType", fieldName, idx, target.Type),
						"11.5",
						itemField,
						candidate,
					))
					hasRefIssue = true
					continue
				}

				resolvedItems = append(resolvedItems, writemodel.ResolvedRef{
					Type:    target.Type,
					ID:      target.ID,
					Slug:    target.Slug,
					DirPath: target.DirPath,
					Meta:    target.Meta,
				})
			}
			if !hasRefIssue && len(resolvedItems) == len(targetIDs) {
				resolvedArrays[fieldName] = resolvedItems
			}
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

		targets := entitiesByID[targetID]
		if len(targets) == 0 {
			refIssues = append(refIssues, issues.New(
				"meta.entityRef_target_missing",
				fmt.Sprintf("entityRef '%s' points to missing id '%s'", fieldName, targetID),
				"11.5",
				"frontmatter."+fieldName,
				candidate,
			))
			continue
		}
		if len(targets) > 1 {
			refIssues = append(refIssues, issues.New(
				"meta.entityRef_target_ambiguous",
				fmt.Sprintf("entityRef '%s' points to ambiguous id '%s'", fieldName, targetID),
				"11.5",
				"frontmatter."+fieldName,
				candidate,
			))
			continue
		}

		target := targets[0]
		if len(fieldSpec.RefTypes) > 0 && !contains(fieldSpec.RefTypes, target.Type) {
			refIssues = append(refIssues, issues.New(
				"meta.entityRef_type_mismatch",
				fmt.Sprintf("entityRef '%s' points to type '%s' outside refType", fieldName, target.Type),
				"11.5",
				"frontmatter."+fieldName,
				candidate,
			))
			continue
		}

		resolved[fieldName] = writemodel.ResolvedRef{
			Type:    target.Type,
			ID:      target.ID,
			Slug:    target.Slug,
			DirPath: target.DirPath,
			Meta:    target.Meta,
		}
	}

	return resolved, resolvedArrays, refIssues
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func readRefIDArray(rawValue any) ([]string, bool) {
	rawItems, ok := rawValue.([]any)
	if !ok {
		return nil, false
	}

	result := make([]string, 0, len(rawItems))
	for _, rawItem := range rawItems {
		itemText, ok := rawItem.(string)
		if !ok {
			return nil, false
		}
		itemText = strings.TrimSpace(itemText)
		if itemText == "" {
			return nil, false
		}
		result = append(result, itemText)
	}
	return result, true
}
