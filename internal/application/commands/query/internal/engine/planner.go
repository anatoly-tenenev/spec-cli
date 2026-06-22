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

	var wherePlan *model.WherePlan
	if opts.WhereExpr != "" {
		compiled, compileErr := compileWhereExpression(opts.WhereExpr, capability, activeTypeSet)
		if compileErr != nil {
			return model.QueryPlan{}, compileErr
		}
		wherePlan = compiled
	}

	if err := validateScopedEntityTypes(opts, capability, activeTypeSet); err != nil {
		return model.QueryPlan{}, err
	}

	rootPlans, rootPlanErr := buildRootPlans(opts, capability, activeTypeSet)
	if rootPlanErr != nil {
		return model.QueryPlan{}, rootPlanErr
	}

	return model.QueryPlan{
		SelectTree:        selectTree,
		Where:             wherePlan,
		ActiveTypeSet:     append([]string(nil), activeTypeSet...),
		RootPlans:         rootPlans,
		OriginalSelects:   selects,
		OriginalSortTerms: opts.Sorts,
	}, nil
}

func validateScopedEntityTypes(
	opts model.Options,
	capability schemacapread.Capability,
	activeTypeSet []string,
) *domainerrors.AppError {
	scopes := map[string]struct{}{}
	for scope := range opts.ScopedLimits {
		scopes[scope] = struct{}{}
	}
	for scope := range opts.ScopedOffsets {
		scopes[scope] = struct{}{}
	}
	for scope := range opts.ScopedSorts {
		scopes[scope] = struct{}{}
	}

	active := map[string]struct{}{}
	for _, typeName := range activeTypeSet {
		active[typeName] = struct{}{}
	}

	for _, scope := range sortedScopes(scopes) {
		if _, exists := capability.EntityTypes[scope]; !exists {
			return domainerrors.New(
				domainerrors.CodeEntityTypeUnknown,
				fmt.Sprintf("unknown scoped entity type: %s", scope),
				map[string]any{"entity_type": scope},
			)
		}
		if _, enabled := active[scope]; !enabled {
			return domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("scoped entity type is not active: %s", scope),
				map[string]any{"entity_type": scope},
			)
		}
	}
	return nil
}

func sortedScopes(scopes map[string]struct{}) []string {
	ordered := make([]string, 0, len(scopes))
	for scope := range scopes {
		ordered = append(ordered, scope)
	}
	sort.Strings(ordered)
	return ordered
}

func buildRootPlans(
	opts model.Options,
	capability schemacapread.Capability,
	activeTypeSet []string,
) ([]model.RootPlan, *domainerrors.AppError) {
	rootPlans := make([]model.RootPlan, 0, len(activeTypeSet))
	for _, entityType := range activeTypeSet {
		limit := opts.Limit
		if scopedLimit, exists := opts.ScopedLimits[entityType]; exists {
			limit = scopedLimit
		}

		offset := opts.Offset
		if scopedOffset, exists := opts.ScopedOffsets[entityType]; exists {
			offset = scopedOffset
		}

		sortInput := opts.Sorts
		sortValidationTypeSet := activeTypeSet
		if scopedSorts, exists := opts.ScopedSorts[entityType]; exists {
			sortInput = scopedSorts
			sortValidationTypeSet = []string{entityType}
		}

		effectiveSort, sortErr := buildEffectiveSort(sortInput, capability, sortValidationTypeSet)
		if sortErr != nil {
			return nil, sortErr
		}

		rootPlans = append(rootPlans, model.RootPlan{
			EntityType:    entityType,
			Limit:         limit,
			Offset:        offset,
			EffectiveSort: effectiveSort,
		})
	}
	return rootPlans, nil
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
