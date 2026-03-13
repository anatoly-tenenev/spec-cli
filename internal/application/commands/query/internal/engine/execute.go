package engine

import "github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"

func Execute(plan model.QueryPlan, entities []model.EntityView) model.QueryResponse {
	matchedEntities := make([]model.EntityView, 0, len(entities))
	for _, entity := range entities {
		if plan.TypedFilter != nil {
			if !EvaluateFilter(*plan.TypedFilter, entity.View) {
				continue
			}
		}
		matchedEntities = append(matchedEntities, entity)
	}

	SortEntities(matchedEntities, plan.EffectiveSort)

	matchedCount := len(matchedEntities)
	pagedEntities, returned := paginateEntities(matchedEntities, plan.Offset, plan.Limit)

	items := make([]map[string]any, 0, len(pagedEntities))
	for _, entity := range pagedEntities {
		items = append(items, ProjectEntity(entity.View, plan.SelectTree))
	}

	hasMore := matchedCount > plan.Offset+returned
	var nextOffset any
	if hasMore {
		nextOffset = plan.Offset + returned
	}

	return model.QueryResponse{
		ResultState: "valid",
		Items:       items,
		Matched:     matchedCount,
		Page: model.PageInfo{
			Mode:          "offset",
			Limit:         plan.Limit,
			Offset:        plan.Offset,
			Returned:      returned,
			HasMore:       hasMore,
			NextOffset:    nextOffset,
			EffectiveSort: sortTermsToStrings(plan.EffectiveSort),
		},
	}
}

func paginateEntities(entities []model.EntityView, offset int, limit int) ([]model.EntityView, int) {
	if limit == 0 {
		return []model.EntityView{}, 0
	}
	if offset >= len(entities) {
		return []model.EntityView{}, 0
	}
	end := offset + limit
	if end > len(entities) {
		end = len(entities)
	}
	paged := entities[offset:end]
	return paged, len(paged)
}

func sortTermsToStrings(terms []model.SortTerm) []string {
	serialized := make([]string, 0, len(terms))
	for _, term := range terms {
		serialized = append(serialized, term.Path+":"+string(term.Direction))
	}
	return serialized
}
