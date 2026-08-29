// Package references resolves the entityRef fields of a document against the
// rest of the workspace. A reference that cannot be resolved is returned as an
// unresolved object naming why - missing, ambiguous or type_mismatch - instead
// of failing the read: the tool reports what the workspace says rather than
// refusing to show a document that points at a gap.
//
// It produces two shapes of the same reference: the public one a response may
// contain, and the one a --where expression filters on.
package references

import (
	"fmt"

	"github.com/anatoly-tenenev/spec-cli/internal/application/entityrefs"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/internal/ordered"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/workspace/internal/diagnostics"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/workspace/internal/documents"
	schemacapread "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/read"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func BuildIDIndex(entities []documents.Entity) map[string][]entityrefs.Identity {
	idIndex := map[string][]entityrefs.Identity{}
	for _, entity := range entities {
		idIndex[entity.ID] = append(idIndex[entity.ID], entityrefs.Identity{
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
	idIndex map[string][]entityrefs.Identity,
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
	idIndex map[string][]entityrefs.Identity,
	refField string,
) (public any, where any, includeInWhere bool, err *domainerrors.AppError) {
	if rawTarget == nil {
		return nil, nil, false, nil
	}

	targetID, ok := entityrefs.ReadID(rawTarget)
	if !ok {
		return nil, nil, false, invalidRefReadError(refField)
	}

	resolved := entityrefs.Classify(targetID, refSpec, idIndex)
	return entityrefs.ToPublicObject(resolved), toWhereRefObject(resolved), true, nil
}

func resolveArrayRef(
	rawTarget any,
	refSpec schemacapread.RefField,
	idIndex map[string][]entityrefs.Identity,
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
		targetID, ok := entityrefs.ReadID(item)
		if !ok {
			return nil, nil, invalidRefReadError(refField)
		}
		resolved := entityrefs.Classify(targetID, refSpec, idIndex)
		publicItems = append(publicItems, entityrefs.ToPublicObject(resolved))
		whereItems = append(whereItems, toWhereRefObject(resolved))
	}

	return publicItems, whereItems, nil
}

func toWhereRefObject(ref entityrefs.Ref) map[string]any {
	return map[string]any{
		"resolved": ref.Resolved,
		"id":       ref.ID,
		"type":     ref.Type,
		"slug":     ref.Slug,
		"reason":   ref.Reason,
	}
}

func invalidRefReadError(refField string) *domainerrors.AppError {
	return diagnostics.NewReadError(
		"failed to compute refs",
		fmt.Sprintf("refs field '%s' has invalid value in frontmatter", refField),
		diagnostics.RefsStandardRef,
		map[string]any{"field": refField},
	)
}
