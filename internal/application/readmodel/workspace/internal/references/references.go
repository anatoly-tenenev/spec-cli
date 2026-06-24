package references

import (
	"fmt"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/internal/ordered"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/workspace/internal/diagnostics"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/workspace/internal/documents"
	schemacapread "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/read"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

type entityIdentity struct {
	Type string
	ID   string
	Slug string
}

type resolvedRef struct {
	ID       string
	Resolved bool
	Type     any
	Slug     any
	Reason   any
}

func BuildIDIndex(entities []documents.Entity) map[string][]entityIdentity {
	idIndex := map[string][]entityIdentity{}
	for _, entity := range entities {
		idIndex[entity.ID] = append(idIndex[entity.ID], entityIdentity{
			Type: entity.Type,
			ID:   entity.ID,
			Slug: entity.Slug,
		})
	}
	return idIndex
}

func Resolve(
	frontmatter map[string]any,
	refFields map[string]schemacapread.RefField,
	idIndex map[string][]entityIdentity,
) (map[string]any, map[string]any, *domainerrors.AppError) {
	publicRefs := map[string]any{}
	whereRefs := map[string]any{}

	for _, refField := range ordered.MapKeys(refFields) {
		refSpec := refFields[refField]
		rawTarget, exists := frontmatter[refField]
		if !exists {
			publicRefs[refField] = nil
			continue
		}

		if refSpec.Cardinality == schemacapread.RefCardinalityArray {
			publicValue, whereValue, err := resolveArrayRef(rawTarget, refSpec, idIndex, refField)
			if err != nil {
				return nil, nil, err
			}
			publicRefs[refField] = publicValue
			whereRefs[refField] = whereValue
			continue
		}

		publicValue, whereValue, includeInWhere, err := resolveScalarRef(rawTarget, refSpec, idIndex, refField)
		if err != nil {
			return nil, nil, err
		}
		publicRefs[refField] = publicValue
		if includeInWhere {
			whereRefs[refField] = whereValue
		}
	}

	return publicRefs, whereRefs, nil
}

func resolveScalarRef(
	rawTarget any,
	refSpec schemacapread.RefField,
	idIndex map[string][]entityIdentity,
	refField string,
) (public any, where any, includeInWhere bool, err *domainerrors.AppError) {
	if rawTarget == nil {
		return nil, nil, false, nil
	}

	targetID, ok := readRefID(rawTarget)
	if !ok {
		return nil, nil, false, invalidRefReadError(refField)
	}

	resolved := classifyResolvedRef(targetID, refSpec, idIndex)
	return toPublicRefObject(resolved), toWhereRefObject(resolved), true, nil
}

func resolveArrayRef(
	rawTarget any,
	refSpec schemacapread.RefField,
	idIndex map[string][]entityIdentity,
	refField string,
) (any, any, *domainerrors.AppError) {
	if rawTarget == nil {
		return nil, nil, nil
	}

	items, ok := rawTarget.([]any)
	if !ok {
		return nil, nil, invalidRefReadError(refField)
	}

	publicItems := make([]any, 0, len(items))
	whereItems := make([]any, 0, len(items))
	for _, item := range items {
		if item == nil {
			publicItems = append(publicItems, nil)
			whereItems = append(whereItems, nil)
			continue
		}
		targetID, ok := readRefID(item)
		if !ok {
			return nil, nil, invalidRefReadError(refField)
		}
		resolved := classifyResolvedRef(targetID, refSpec, idIndex)
		publicItems = append(publicItems, toPublicRefObject(resolved))
		whereItems = append(whereItems, toWhereRefObject(resolved))
	}

	return publicItems, whereItems, nil
}

func classifyResolvedRef(targetID string, refSpec schemacapread.RefField, idIndex map[string][]entityIdentity) resolvedRef {
	targets := idIndex[targetID]
	compatibleTargets := filterTargetsByRefTypes(targets, refSpec.AllowedTypes)

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
		Type:     deterministicRefTypeHint(compatibleTargets, refSpec.AllowedTypes),
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

func toWhereRefObject(ref resolvedRef) map[string]any {
	return map[string]any{
		"resolved": ref.Resolved,
		"id":       ref.ID,
		"type":     ref.Type,
		"slug":     ref.Slug,
		"reason":   ref.Reason,
	}
}

func filterTargetsByRefTypes(targets []entityIdentity, refTypes []string) []entityIdentity {
	allowed := map[string]struct{}{}
	for _, refType := range refTypes {
		allowed[refType] = struct{}{}
	}
	filtered := make([]entityIdentity, 0, len(targets))
	for _, target := range targets {
		if _, ok := allowed[target.Type]; ok {
			filtered = append(filtered, target)
		}
	}
	return filtered
}

func deterministicRefTypeHint(targets []entityIdentity, refTypes []string) any {
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
	targetID := strings.TrimSpace(raw)
	if targetID == "" {
		return "", false
	}
	return targetID, true
}

func invalidRefReadError(refField string) *domainerrors.AppError {
	return diagnostics.NewReadError(
		"failed to compute refs",
		fmt.Sprintf("refs field '%s' has invalid value in frontmatter", refField),
		diagnostics.RefsStandardRef,
		map[string]any{"field": refField},
	)
}
