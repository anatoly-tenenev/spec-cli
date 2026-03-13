package engine

import (
	"encoding/json"
	"fmt"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

var defaultSelects = []string{"type", "id", "slug"}

func BuildPlan(opts model.Options, index model.QuerySchemaIndex) (model.QueryPlan, *domainerrors.AppError) {
	if err := validateTypeFilters(opts.TypeFilters, index); err != nil {
		return model.QueryPlan{}, err
	}

	selects := opts.Selects
	if len(selects) == 0 {
		selects = append([]string(nil), defaultSelects...)
	}

	selectTree, selectErr := buildSelectTree(selects, index)
	if selectErr != nil {
		return model.QueryPlan{}, selectErr
	}

	effectiveSort, sortErr := buildEffectiveSort(opts.Sorts, index)
	if sortErr != nil {
		return model.QueryPlan{}, sortErr
	}

	var typedFilter *model.FilterNode
	if opts.WhereJSON != "" {
		rawNode, parseErr := parseWhereJSON(opts.WhereJSON)
		if parseErr != nil {
			return model.QueryPlan{}, parseErr
		}
		boundNode, bindErr := bindWhereNode(rawNode, index)
		if bindErr != nil {
			return model.QueryPlan{}, bindErr
		}
		typedFilter = &boundNode
	}

	return model.QueryPlan{
		SelectTree:        selectTree,
		TypedFilter:       typedFilter,
		EffectiveSort:     effectiveSort,
		OriginalSelects:   selects,
		OriginalSortTerms: opts.Sorts,
		Limit:             opts.Limit,
		Offset:            opts.Offset,
	}, nil
}

func validateTypeFilters(typeFilters []string, index model.QuerySchemaIndex) *domainerrors.AppError {
	for _, typeName := range typeFilters {
		if _, exists := index.EntityTypes[typeName]; exists {
			continue
		}
		return domainerrors.New(
			domainerrors.CodeEntityTypeUnknown,
			fmt.Sprintf("unknown entity type: %s", typeName),
			map[string]any{"entity_type": typeName},
		)
	}
	return nil
}

func ParseJSONPayload(raw string) (any, error) {
	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}
