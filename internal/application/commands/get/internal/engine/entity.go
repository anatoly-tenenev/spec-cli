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

type resolvedRef struct {
	ID       string
	Resolved bool
	Type     any
	Slug     any
	Reason   any
}

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
		view["createdDate"] = target.CreatedDate
	}
	if target.UpdatedDate != "" {
		view["updatedDate"] = target.UpdatedDate
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
	identityIndex map[string][]model.EntityIdentity,
	requestedFields map[string]model.RefFieldSpec,
) (map[string]any, *domainerrors.AppError) {
	refs := map[string]any{}
	for _, refField := range support.SortedMapKeys(requestedFields) {
		refSpec := requestedFields[refField]
		rawTarget, exists := frontmatter[refField]
		if !exists {
			refs[refField] = nil
			continue
		}

		if refSpec.Cardinality == model.RefCardinalityArray {
			refValue, refErr := resolveArrayRefValue(rawTarget, refSpec, identityIndex, refField)
			if refErr != nil {
				return nil, refErr
			}
			refs[refField] = refValue
			continue
		}

		refValue, refErr := resolveScalarRefValue(rawTarget, refSpec, identityIndex, refField)
		if refErr != nil {
			return nil, refErr
		}
		refs[refField] = refValue
	}
	return refs, nil
}

func resolveScalarRefValue(
	rawTarget any,
	refSpec model.RefFieldSpec,
	identityIndex map[string][]model.EntityIdentity,
	refField string,
) (any, *domainerrors.AppError) {
	if rawTarget == nil {
		return nil, nil
	}

	targetID, ok := readRefID(rawTarget)
	if !ok {
		return nil, invalidRefReadError(refField)
	}
	resolved := classifyResolvedRef(targetID, refSpec, identityIndex)
	return toPublicRefObject(resolved), nil
}

func resolveArrayRefValue(
	rawTarget any,
	refSpec model.RefFieldSpec,
	identityIndex map[string][]model.EntityIdentity,
	refField string,
) (any, *domainerrors.AppError) {
	if rawTarget == nil {
		return nil, nil
	}

	items, ok := rawTarget.([]any)
	if !ok {
		return nil, invalidRefReadError(refField)
	}

	resolvedItems := make([]any, 0, len(items))
	for _, item := range items {
		if item == nil {
			resolvedItems = append(resolvedItems, nil)
			continue
		}
		targetID, ok := readRefID(item)
		if !ok {
			return nil, invalidRefReadError(refField)
		}
		resolved := classifyResolvedRef(targetID, refSpec, identityIndex)
		resolvedItems = append(resolvedItems, toPublicRefObject(resolved))
	}
	return resolvedItems, nil
}

func classifyResolvedRef(targetID string, refSpec model.RefFieldSpec, identityIndex map[string][]model.EntityIdentity) resolvedRef {
	targets := identityIndex[targetID]
	compatibleTargets := filterTargetsByRefTypes(targets, refSpec.RefTypes)

	if len(compatibleTargets) == 1 {
		target := compatibleTargets[0]
		return resolvedRef{
			ID:       targetID,
			Resolved: true,
			Type:     target.Type,
			Slug:     target.Slug,
			Reason:   nil,
		}
	}

	reason := "ambiguous"
	switch {
	case len(targets) == 0:
		reason = "missing"
	case len(compatibleTargets) == 0:
		reason = "type_mismatch"
	}

	return resolvedRef{
		ID:       targetID,
		Resolved: false,
		Type:     deterministicRefTypeHint(compatibleTargets, refSpec.RefTypes),
		Slug:     nil,
		Reason:   reason,
	}
}

func toPublicRefObject(ref resolvedRef) map[string]any {
	value := map[string]any{
		"resolved": ref.Resolved,
		"id":       ref.ID,
		"type":     ref.Type,
		"slug":     ref.Slug,
	}
	if !ref.Resolved {
		value["reason"] = ref.Reason
	}
	return value
}

func buildRequestedRefFields(refFields map[string]model.RefFieldSpec, plan model.SelectorPlan) map[string]model.RefFieldSpec {
	requested := map[string]model.RefFieldSpec{}
	if plan.RequiresAllRefFields {
		for field, spec := range refFields {
			requested[field] = spec
		}
		return requested
	}
	for field := range plan.RequiredRefFields {
		spec, exists := refFields[field]
		if !exists {
			continue
		}
		requested[field] = spec
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

func filterTargetsByRefTypes(targets []model.EntityIdentity, refTypes []string) []model.EntityIdentity {
	if len(refTypes) == 0 {
		return targets
	}
	allowed := map[string]struct{}{}
	for _, refType := range refTypes {
		allowed[refType] = struct{}{}
	}
	filtered := make([]model.EntityIdentity, 0, len(targets))
	for _, target := range targets {
		if _, ok := allowed[target.Type]; ok {
			filtered = append(filtered, target)
		}
	}
	return filtered
}

func deterministicRefTypeHint(targets []model.EntityIdentity, refTypes []string) any {
	if len(refTypes) == 1 {
		return refTypes[0]
	}
	if len(targets) == 0 {
		return nil
	}
	candidate := targets[0].Type
	for idx := 1; idx < len(targets); idx++ {
		if targets[idx].Type != candidate {
			return nil
		}
	}
	return candidate
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

func invalidRefReadError(refField string) *domainerrors.AppError {
	return newReadError(
		"failed to compute requested refs field",
		fmt.Sprintf("requested refs field '%s' has invalid value in frontmatter", refField),
		getEntityRefStandardRef,
		map[string]any{"field": refField},
	)
}

func newReadError(message string, issueMessage string, standardRef string, details map[string]any) *domainerrors.AppError {
	issue := support.ValidationIssue("error", "InstanceError", issueMessage, standardRef)
	return domainerrors.New(domainerrors.CodeReadFailed, message, support.WithValidationIssues(details, issue))
}
