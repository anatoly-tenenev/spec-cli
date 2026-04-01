package engine

import (
	"fmt"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

var defaultSelects = []string{
	"type",
	"id",
	"slug",
	"meta",
	"refs",
}

func BuildPlan(opts model.Options, index model.QuerySchemaIndex) (model.QueryPlan, *domainerrors.AppError) {
	if err := validateTypeFilters(opts.TypeFilters, index); err != nil {
		return model.QueryPlan{}, err
	}
	activeTypeSet := resolveActiveTypeSet(opts.TypeFilters, index)

	selects := opts.Selects
	if len(selects) == 0 {
		selects = append([]string(nil), defaultSelects...)
	}

	selectTree, selectErr := buildSelectTree(selects, index, activeTypeSet)
	if selectErr != nil {
		return model.QueryPlan{}, selectErr
	}

	effectiveSort, sortErr := buildEffectiveSort(opts.Sorts, index, activeTypeSet)
	if sortErr != nil {
		return model.QueryPlan{}, sortErr
	}

	var wherePlan *model.WherePlan
	if opts.WhereExpr != "" {
		compiled, compileErr := compileWhereExpression(opts.WhereExpr, index, activeTypeSet)
		if compileErr != nil {
			return model.QueryPlan{}, compileErr
		}
		wherePlan = compiled
	}

	return model.QueryPlan{
		SelectTree:        selectTree,
		Where:             wherePlan,
		EffectiveSort:     effectiveSort,
		ActiveTypeSet:     append([]string(nil), activeTypeSet...),
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

func resolveActiveTypeSet(typeFilters []string, index model.QuerySchemaIndex) []string {
	if len(typeFilters) == 0 {
		return support.SortedMapKeys(index.EntityTypes)
	}

	set := map[string]struct{}{}
	for _, typeName := range typeFilters {
		set[typeName] = struct{}{}
	}
	return support.SortedMapKeys(set)
}
