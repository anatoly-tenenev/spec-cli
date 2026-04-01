package engine

import (
	"reflect"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func Execute(plan model.QueryPlan, entities []model.EntityView) (model.QueryResponse, *domainerrors.AppError) {
	matchedEntities := make([]model.EntityView, 0, len(entities))
	for _, entity := range entities {
		if plan.Where != nil {
			whereValue, whereErr := plan.Where.Query.Search(entity.WhereContext)
			if whereErr != nil {
				return model.QueryResponse{}, domainerrors.New(
					domainerrors.CodeReadFailed,
					"failed to evaluate --where expression",
					map[string]any{"reason": whereErr.Error()},
				)
			}
			if !isTruthy(whereValue) {
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
	}, nil
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

func isTruthy(value any) bool {
	return !isFalse(value)
}

func isFalse(value any) bool {
	switch typed := value.(type) {
	case bool:
		return !typed
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	case string:
		return len(typed) == 0
	case nil:
		return true
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Struct:
		return false
	case reflect.Slice, reflect.Map:
		return rv.Len() == 0
	case reflect.Ptr:
		if rv.IsNil() {
			return true
		}
		return isFalse(rv.Elem().Interface())
	}

	return false
}
