package engine

import (
	"fmt"
	"sort"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
	schemacapread "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/read"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

var defaultSelects = []string{
	"type",
	"id",
	"slug",
	"meta",
	"refs",
}

func BuildPlan(opts model.Options, capability schemacapread.Capability) (model.QueryPlan, *domainerrors.AppError) {
	if err := validateTypeFilters(opts.TypeFilters, capability); err != nil {
		return model.QueryPlan{}, err
	}
	activeTypeSet := resolveActiveTypeSet(opts.TypeFilters, capability)

	selects := opts.Selects
	if len(selects) == 0 {
		selects = append([]string(nil), defaultSelects...)
	}

	selectTree, selectErr := buildSelectTree(selects, capability, activeTypeSet)
	if selectErr != nil {
		return model.QueryPlan{}, selectErr
	}

	effectiveSort, sortErr := buildEffectiveSort(opts.Sorts, capability, activeTypeSet)
	if sortErr != nil {
		return model.QueryPlan{}, sortErr
	}

	var wherePlan *model.WherePlan
	if opts.WhereExpr != "" {
		compiled, compileErr := compileWhereExpression(opts.WhereExpr, capability, activeTypeSet)
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

func validateTypeFilters(typeFilters []string, capability schemacapread.Capability) *domainerrors.AppError {
	for _, typeName := range typeFilters {
		if _, exists := capability.EntityTypes[typeName]; exists {
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

func resolveActiveTypeSet(typeFilters []string, capability schemacapread.Capability) []string {
	if len(typeFilters) == 0 {
		if len(capability.EntityOrder) > 0 {
			return append([]string(nil), capability.EntityOrder...)
		}
		return sortedEntityTypes(capability.EntityTypes)
	}

	set := map[string]struct{}{}
	ordered := make([]string, 0, len(typeFilters))
	for _, typeName := range typeFilters {
		if _, exists := set[typeName]; exists {
			continue
		}
		set[typeName] = struct{}{}
		ordered = append(ordered, typeName)
	}
	return ordered
}

func sortedEntityTypes(entityTypes map[string]schemacapread.EntityReadModel) []string {
	names := make([]string, 0, len(entityTypes))
	for typeName := range entityTypes {
		names = append(names, typeName)
	}
	sort.Strings(names)
	return names
}
