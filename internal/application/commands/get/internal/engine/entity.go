package engine

import (
	"fmt"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/get/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/get/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

const (
	getEntityTypeStandardRef     = "5.3"
	getEntityRefStandardRef      = "6"
	getEntitySectionsStandardRef = "8.3"
)

func BuildEntityView(
	target model.ParsedTarget,
	readModel model.ReadModel,
	identityIndex map[string][]model.EntityIdentity,
	plan model.SelectorPlan,
) (map[string]any, *domainerrors.AppError) {
	entityType, exists := readModel.EntityTypes[target.Type]
	if !exists {
		return nil, newReadError(
			"failed to determine entity type",
			fmt.Sprintf("entity type '%s' is not declared in schema.entity", target.Type),
			getEntityTypeStandardRef,
			nil,
		)
	}

	meta := buildMeta(target.Frontmatter, entityType.MetaFields)
	refs := map[string]any{}
	if plan.RequiresRefs {
		requestedFields := buildRequestedRefFields(entityType.RefFields, plan)
		resolvedRefs, refErr := resolveRefs(
			target.Frontmatter,
			entityType.RefTypeHints,
			identityIndex,
			requestedFields,
		)
		if refErr != nil {
			return nil, refErr
		}
		refs = resolvedRefs
	}

	if plan.RequiresSections {
		if sectionErr := validateRequestedSections(target.DuplicateSectionLabels, plan); sectionErr != nil {
			return nil, sectionErr
		}
	}

	view := map[string]any{
		"type":     target.Type,
		"id":       target.ID,
		"revision": target.Revision,
		"meta":     meta,
	}

	if target.Slug != "" {
		view["slug"] = target.Slug
	}
	if target.CreatedDate != "" {
		view["created_date"] = target.CreatedDate
	}
	if target.UpdatedDate != "" {
		view["updated_date"] = target.UpdatedDate
	}
	if plan.RequiresRefs {
		view["refs"] = refs
	}
	if plan.RequiresContent {
		content := map[string]any{}
		if plan.RequiresContentRaw {
			content["raw"] = target.RawBody
		}
		if plan.RequiresSections {
			content["sections"] = sectionsToAnyMap(target.Sections)
		}
		view["content"] = content
	}

	return view, nil
}

func buildMeta(frontmatter map[string]any, allowedFields map[string]struct{}) map[string]any {
	meta := map[string]any{}
	for _, field := range support.SortedMapKeys(allowedFields) {
		value, exists := frontmatter[field]
		if !exists {
			continue
		}
		meta[field] = support.DeepCopy(value)
	}
	return meta
}

func resolveRefs(
	frontmatter map[string]any,
	refTypeHints map[string]string,
	identityIndex map[string][]model.EntityIdentity,
	requestedFields map[string]struct{},
) (map[string]any, *domainerrors.AppError) {
	refs := map[string]any{}
	for _, refField := range support.SortedMapKeys(requestedFields) {
		rawTarget, exists := frontmatter[refField]
		if !exists {
			refs[refField] = nil
			continue
		}
		targetID, ok := readRefID(rawTarget)
		if !ok {
			return nil, newReadError(
				"failed to compute requested refs field",
				fmt.Sprintf("requested refs field '%s' cannot be computed deterministically", refField),
				getEntityRefStandardRef,
				map[string]any{"field": refField},
			)
		}

		targets := identityIndex[targetID]
		hintedType := refTypeHints[refField]
		compatibleTargets := filterTargetsByHint(targets, hintedType)
		refValue := map[string]any{
			"id":       targetID,
			"resolved": false,
			"type":     nil,
			"slug":     nil,
		}

		if len(targets) == 1 && isResolvedRefTarget(targets[0], hintedType) {
			target := targets[0]
			refValue["resolved"] = true
			refValue["type"] = target.Type
			refValue["slug"] = target.Slug
			refs[refField] = refValue
			continue
		}

		if deterministicType := deterministicRefType(compatibleTargets, hintedType); deterministicType != "" {
			refValue["type"] = deterministicType
		}
		if deterministicSlug := deterministicRefSlug(compatibleTargets); deterministicSlug != "" {
			refValue["slug"] = deterministicSlug
		}

		refs[refField] = refValue
	}
	return refs, nil
}

func buildRequestedRefFields(refFields map[string]struct{}, plan model.SelectorPlan) map[string]struct{} {
	requested := map[string]struct{}{}
	if plan.RequiresAllRefFields {
		for field := range refFields {
			requested[field] = struct{}{}
		}
		return requested
	}
	for field := range plan.RequiredRefFields {
		requested[field] = struct{}{}
	}
	return requested
}

func readRefID(rawTarget any) (string, bool) {
	switch typed := rawTarget.(type) {
	case string:
		return normalizeRefID(typed)
	case map[string]any:
		rawID, ok := typed["id"]
		if !ok {
			return "", false
		}
		targetID, ok := rawID.(string)
		if !ok {
			return "", false
		}
		return normalizeRefID(targetID)
	default:
		return "", false
	}
}

func normalizeRefID(raw string) (string, bool) {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return "", false
	}
	return normalized, true
}

func isResolvedRefTarget(target model.EntityIdentity, hintedType string) bool {
	hintedType = strings.TrimSpace(hintedType)
	return hintedType == "" || target.Type == hintedType
}

func filterTargetsByHint(targets []model.EntityIdentity, hintedType string) []model.EntityIdentity {
	hintedType = strings.TrimSpace(hintedType)
	if hintedType == "" {
		return targets
	}
	filtered := make([]model.EntityIdentity, 0, len(targets))
	for _, target := range targets {
		if target.Type == hintedType {
			filtered = append(filtered, target)
		}
	}
	return filtered
}

func deterministicRefType(targets []model.EntityIdentity, hintedType string) string {
	hintedType = strings.TrimSpace(hintedType)
	if hintedType != "" {
		return hintedType
	}
	if len(targets) == 0 {
		return ""
	}
	deterministic := targets[0].Type
	for idx := 1; idx < len(targets); idx++ {
		if targets[idx].Type != deterministic {
			return ""
		}
	}
	return deterministic
}

func deterministicRefSlug(targets []model.EntityIdentity) string {
	if len(targets) == 0 {
		return ""
	}
	deterministic := targets[0].Slug
	for idx := 1; idx < len(targets); idx++ {
		if targets[idx].Slug != deterministic {
			return ""
		}
	}
	return deterministic
}

func validateRequestedSections(duplicates map[string]int, plan model.SelectorPlan) *domainerrors.AppError {
	if len(duplicates) == 0 {
		return nil
	}

	if plan.RequiresAllSections {
		for _, label := range support.SortedMapKeys(duplicates) {
			return newReadError(
				"failed to compute requested content sections",
				fmt.Sprintf("section label '%s' is duplicated", label),
				getEntitySectionsStandardRef,
				map[string]any{"section": label},
			)
		}
	}

	for section := range plan.RequiredSectionNames {
		if duplicates[section] <= 1 {
			continue
		}
		return newReadError(
			"failed to compute requested content sections",
			fmt.Sprintf("section label '%s' is duplicated", section),
			getEntitySectionsStandardRef,
			map[string]any{"section": section},
		)
	}

	return nil
}

func sectionsToAnyMap(sections map[string]string) map[string]any {
	mapped := make(map[string]any, len(sections))
	for key, value := range sections {
		mapped[key] = value
	}
	return mapped
}

func newReadError(message string, issueMessage string, standardRef string, details map[string]any) *domainerrors.AppError {
	issue := support.ValidationIssue("error", "InstanceError", issueMessage, standardRef)
	return domainerrors.New(domainerrors.CodeReadFailed, message, support.WithValidationIssues(details, issue))
}
