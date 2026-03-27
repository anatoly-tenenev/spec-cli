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
		if fieldRule.Type != "entityRef" {
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

		resolution := resolveEntityReferenceValue(
			fieldRule.Name,
			fmt.Sprintf("frontmatter.%s", fieldRule.Name),
			referenceID,
			fieldRule.RefTypes,
			idIndex,
		)
		if resolution.Issue != nil {
			addIssue(issues, entity, *resolution.Issue)
			continue
		}

		resolved[fieldRule.Name] = resolution.Target
	}

	return resolved
}

type entityRefResolution struct {
	Target resolvedEntityRef
	Issue  *domainvalidation.Issue
}

func resolveEntityReferenceValue(
	fieldLabel string,
	fieldPath string,
	referenceID string,
	refTypes []string,
	idIndex map[string][]resolvedEntityRef,
) entityRefResolution {
	targets := idIndex[referenceID]
	switch len(targets) {
	case 0:
		issue := domainvalidation.Issue{
			Code:        "meta.entityRef_target_missing",
			Level:       domainvalidation.LevelError,
			Class:       "InstanceError",
			Message:     fmt.Sprintf("entityRef field '%s' points to unknown id '%s'", fieldLabel, referenceID),
			StandardRef: "12.3",
			Field:       fieldPath,
		}
		return entityRefResolution{Issue: &issue}
	case 1:
		// continue below
	default:
		issue := domainvalidation.Issue{
			Code:        "meta.entityRef_target_ambiguous",
			Level:       domainvalidation.LevelError,
			Class:       "InstanceError",
			Message:     fmt.Sprintf("entityRef field '%s' points to ambiguous id '%s'", fieldLabel, referenceID),
			StandardRef: "12.3",
			Field:       fieldPath,
		}
		return entityRefResolution{Issue: &issue}
	}

	target := targets[0]
	if len(refTypes) > 0 && !containsString(refTypes, target.Type) {
		issue := domainvalidation.Issue{
			Code:        "meta.entityRef_type_mismatch",
			Level:       domainvalidation.LevelError,
			Class:       "InstanceError",
			Message:     fmt.Sprintf("entityRef field '%s' cannot reference entity type '%s'", fieldLabel, target.Type),
			StandardRef: "12.3",
			Field:       fieldPath,
		}
		return entityRefResolution{Issue: &issue}
	}

	return entityRefResolution{Target: target}
}

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
