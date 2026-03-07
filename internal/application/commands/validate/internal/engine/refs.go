package engine

import (
	"fmt"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

func resolveEntityReferences(
	issues *[]domainvalidation.Issue,
	entity *model.CheckedEntity,
	frontmatter map[string]any,
	typeSpec model.SchemaEntityType,
	idIndex map[string][]resolvedEntityRef,
) map[string]resolvedEntityRef {
	resolved := make(map[string]resolvedEntityRef)

	for _, fieldRule := range typeSpec.RequiredFields {
		if fieldRule.Type != "entity_ref" {
			continue
		}

		rawValue, exists := frontmatter[fieldRule.Name]
		if !exists {
			continue
		}

		referenceID, ok := rawValue.(string)
		if !ok {
			continue
		}
		referenceID = strings.TrimSpace(referenceID)
		if referenceID == "" {
			continue
		}

		targets := idIndex[referenceID]
		switch len(targets) {
		case 0:
			addIssue(issues, entity, domainvalidation.Issue{
				Code:        "meta.entity_ref_target_missing",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     fmt.Sprintf("entity_ref field '%s' points to unknown id '%s'", fieldRule.Name, referenceID),
				StandardRef: "12.3",
				Field:       fmt.Sprintf("frontmatter.%s", fieldRule.Name),
			})
			continue
		case 1:
			// continue below
		default:
			addIssue(issues, entity, domainvalidation.Issue{
				Code:        "meta.entity_ref_target_ambiguous",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     fmt.Sprintf("entity_ref field '%s' points to ambiguous id '%s'", fieldRule.Name, referenceID),
				StandardRef: "12.3",
				Field:       fmt.Sprintf("frontmatter.%s", fieldRule.Name),
			})
			continue
		}

		target := targets[0]
		if len(fieldRule.RefTypes) > 0 && !containsString(fieldRule.RefTypes, target.Type) {
			addIssue(issues, entity, domainvalidation.Issue{
				Code:        "meta.entity_ref_type_mismatch",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     fmt.Sprintf("entity_ref field '%s' cannot reference entity type '%s'", fieldRule.Name, target.Type),
				StandardRef: "12.3",
				Field:       fmt.Sprintf("frontmatter.%s", fieldRule.Name),
			})
			continue
		}

		resolved[fieldRule.Name] = target
	}

	return resolved
}

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
