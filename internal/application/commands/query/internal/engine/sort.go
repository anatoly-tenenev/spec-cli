package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

var defaultSort = []model.SortTerm{
	{Path: "type", Direction: model.SortDirectionAsc},
	{Path: "id", Direction: model.SortDirectionAsc},
}

var builtinSortKinds = map[string]model.SchemaFieldKind{
	"type":        model.FieldKindString,
	"id":          model.FieldKindString,
	"slug":        model.FieldKindString,
	"revision":    model.FieldKindString,
	"createdDate": model.FieldKindDate,
	"updatedDate": model.FieldKindDate,
	"content.raw": model.FieldKindString,
}

func buildEffectiveSort(requested []model.SortTerm, index model.QuerySchemaIndex, activeTypeSet []string) ([]model.SortTerm, *domainerrors.AppError) {
	terms := requested
	if len(terms) == 0 {
		terms = append([]model.SortTerm(nil), defaultSort...)
	}

	for _, term := range terms {
		if err := validateSortPath(term.Path, index, activeTypeSet); err != nil {
			return nil, err
		}
	}

	effective := append([]model.SortTerm(nil), terms...)
	if len(effective) < 2 ||
		effective[len(effective)-2].Path != "type" ||
		effective[len(effective)-2].Direction != model.SortDirectionAsc ||
		effective[len(effective)-1].Path != "id" ||
		effective[len(effective)-1].Direction != model.SortDirectionAsc {
		effective = append(effective,
			model.SortTerm{Path: "type", Direction: model.SortDirectionAsc},
			model.SortTerm{Path: "id", Direction: model.SortDirectionAsc},
		)
	}
	return effective, nil
}

func validateSortPath(path string, index model.QuerySchemaIndex, activeTypeSet []string) *domainerrors.AppError {
	if kind, builtin := builtinSortKinds[path]; builtin {
		if !isOrderableKind(kind) {
			return domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("invalid filter-namespace sort field '%s'", path),
				nil,
			)
		}
		return nil
	}

	parts := strings.Split(path, ".")
	if len(parts) == 2 && parts[0] == "meta" {
		if hasRefFieldAcrossActiveSet(parts[1], index, activeTypeSet) {
			return domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("sort field '%s' is forbidden for entityRef field; use refs.%s", path, parts[1]),
				nil,
			)
		}
		kinds := gatherMetaSortKinds(parts[1], index, activeTypeSet)
		return validateSortKinds(path, kinds)
	}
	if len(parts) == 3 && parts[0] == "content" && parts[1] == "sections" {
		kinds := gatherSectionSortKinds(parts[2], index, activeTypeSet)
		return validateSortKinds(path, kinds)
	}
	if len(parts) == 3 && parts[0] == "refs" {
		leaf := parts[2]
		if leaf != "id" && leaf != "resolved" && leaf != "type" && leaf != "slug" && leaf != "reason" {
			return domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("invalid filter-namespace sort field '%s'", path),
				nil,
			)
		}
		kinds, compatErr := gatherRefSortKinds(parts[1], leaf, index, activeTypeSet)
		if compatErr != nil {
			return compatErr
		}
		return validateSortKinds(path, kinds)
	}

	return domainerrors.New(
		domainerrors.CodeInvalidArgs,
		fmt.Sprintf("invalid filter-namespace sort field '%s'", path),
		nil,
	)
}

func gatherMetaSortKinds(field string, index model.QuerySchemaIndex, activeTypeSet []string) map[model.SchemaFieldKind]struct{} {
	kinds := map[model.SchemaFieldKind]struct{}{}
	for _, typeName := range activeTypeSet {
		entityType := index.EntityTypes[typeName]
		if _, isRef := entityType.RefFields[field]; isRef {
			continue
		}
		metaSpec, exists := entityType.MetaFields[field]
		if !exists {
			continue
		}
		kinds[metaSpec.Kind] = struct{}{}
	}
	return kinds
}

func gatherSectionSortKinds(section string, index model.QuerySchemaIndex, activeTypeSet []string) map[model.SchemaFieldKind]struct{} {
	kinds := map[model.SchemaFieldKind]struct{}{}
	for _, typeName := range activeTypeSet {
		entityType := index.EntityTypes[typeName]
		if _, exists := entityType.SectionFields[section]; !exists {
			continue
		}
		kinds[model.FieldKindString] = struct{}{}
	}
	return kinds
}

func gatherRefSortKinds(
	refField string,
	leaf string,
	index model.QuerySchemaIndex,
	activeTypeSet []string,
) (map[model.SchemaFieldKind]struct{}, *domainerrors.AppError) {
	kinds := map[model.SchemaFieldKind]struct{}{}
	found := false
	for _, typeName := range activeTypeSet {
		entityType := index.EntityTypes[typeName]
		refSpec, exists := entityType.RefFields[refField]
		if !exists {
			continue
		}
		found = true
		if refSpec.Cardinality == model.RefCardinalityArray {
			return nil, domainerrors.New(
				domainerrors.CodeInvalidQuery,
				fmt.Sprintf("invalid sort field '%s': path-based ref leaf is forbidden for array refs in active type set", "refs."+refField+"."+leaf),
				nil,
			)
		}

		if leaf == "resolved" {
			kinds[model.FieldKindBoolean] = struct{}{}
		} else {
			kinds[model.FieldKindString] = struct{}{}
		}
	}
	if !found {
		return map[model.SchemaFieldKind]struct{}{}, nil
	}
	return kinds, nil
}

func validateSortKinds(path string, kinds map[model.SchemaFieldKind]struct{}) *domainerrors.AppError {
	if len(kinds) == 0 {
		return domainerrors.New(
			domainerrors.CodeInvalidArgs,
			fmt.Sprintf("invalid filter-namespace sort field '%s'", path),
			nil,
		)
	}
	if len(kinds) > 1 {
		return domainerrors.New(
			domainerrors.CodeInvalidQuery,
			fmt.Sprintf("invalid sort field '%s': incompatible sort kinds across active type set", path),
			nil,
		)
	}
	for kind := range kinds {
		if !isOrderableKind(kind) {
			return domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("invalid filter-namespace sort field '%s'", path),
				nil,
			)
		}
	}
	return nil
}

func isOrderableKind(kind model.SchemaFieldKind) bool {
	switch kind {
	case model.FieldKindString, model.FieldKindDate, model.FieldKindNumber, model.FieldKindBoolean:
		return true
	default:
		return false
	}
}

func SortEntities(entities []model.EntityView, terms []model.SortTerm) {
	sort.SliceStable(entities, func(leftIdx int, rightIdx int) bool {
		left := entities[leftIdx]
		right := entities[rightIdx]
		for _, term := range terms {
			leftValue, leftPresent := resolveReadValue(left.View, term.Path)
			rightValue, rightPresent := resolveReadValue(right.View, term.Path)

			if !leftPresent && !rightPresent {
				continue
			}
			if !leftPresent && rightPresent {
				return term.Direction == model.SortDirectionAsc
			}
			if leftPresent && !rightPresent {
				return term.Direction == model.SortDirectionDesc
			}

			compared := compareLiteralValues(leftValue, rightValue)
			if compared == 0 {
				continue
			}
			if term.Direction == model.SortDirectionAsc {
				return compared < 0
			}
			return compared > 0
		}
		return false
	})
}

func compareLiteralValues(left any, right any) int {
	if leftNum, leftOK := support.NumberToFloat64(left); leftOK {
		if rightNum, rightOK := support.NumberToFloat64(right); rightOK {
			switch {
			case leftNum < rightNum:
				return -1
			case leftNum > rightNum:
				return 1
			default:
				return 0
			}
		}
	}

	leftString, leftStringOK := left.(string)
	rightString, rightStringOK := right.(string)
	if leftStringOK && rightStringOK {
		switch {
		case leftString < rightString:
			return -1
		case leftString > rightString:
			return 1
		default:
			return 0
		}
	}

	leftBool, leftBoolOK := left.(bool)
	rightBool, rightBoolOK := right.(bool)
	if leftBoolOK && rightBoolOK {
		switch {
		case leftBool == rightBool:
			return 0
		case !leftBool && rightBool:
			return -1
		default:
			return 1
		}
	}

	leftRendered := fmt.Sprintf("%v", left)
	rightRendered := fmt.Sprintf("%v", right)
	switch {
	case leftRendered < rightRendered:
		return -1
	case leftRendered > rightRendered:
		return 1
	default:
		return 0
	}
}
