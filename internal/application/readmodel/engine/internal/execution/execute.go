package execution

import (
	"fmt"
	"reflect"

	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/engine/internal/selection"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/ordering"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func Execute(plan model.QueryPlan, entities []model.EntityView) (model.QueryResponse, *domainerrors.AppError) {
	roots := make([]model.QueryRootField, 0, len(plan.RootPlans))
	for _, rootPlan := range plan.RootPlans {
		matchedEntities := make([]model.EntityView, 0, len(entities))
		for _, entity := range entities {
			if entity.Type != rootPlan.EntityType {
				continue
			}
			// A repeated section label carries no value. Filtering on it would
			// silently drop the document, so refuse before evaluating.
			if label, ambiguous := ambiguousSection(entity, plan.WhereSections); ambiguous {
				return model.QueryResponse{}, duplicateSectionError(entity, label, "--where")
			}

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

		ordering.SortEntities(matchedEntities, rootPlan.EffectiveSort)

		matchedCount := len(matchedEntities)
		pagedEntities, returned := paginateEntities(matchedEntities, rootPlan.Offset, rootPlan.Limit)

		items := make([]map[string]any, 0, len(pagedEntities))
		for _, entity := range pagedEntities {
			// Returning the document without a section it repeats would read as
			// "this section is absent", which is not what the document says.
			if label, ambiguous := ambiguousSection(entity, plan.SelectSections); ambiguous {
				return model.QueryResponse{}, duplicateSectionError(entity, label, "--select")
			}
			items = append(items, selection.ProjectEntity(entity.View, plan.SelectTree))
		}

		hasMore := matchedCount > rootPlan.Offset+returned
		var nextOffset any
		if hasMore {
			nextOffset = rootPlan.Offset + returned
		}

		roots = append(roots, model.QueryRootField{
			EntityType: rootPlan.EntityType,
			Items:      items,
			TotalCount: matchedCount,
			PageInfo: model.PageInfo{
				Mode:          "offset",
				Limit:         rootPlan.Limit,
				Offset:        rootPlan.Offset,
				Returned:      returned,
				HasMore:       hasMore,
				NextOffset:    nextOffset,
				EffectiveSort: sortTermsToStrings(rootPlan.EffectiveSort),
			},
		})
	}

	return model.QueryResponse{
		ResultState: "valid",
		RootFields:  roots,
	}, nil
}

// ambiguousSection returns the first repeated label the access set reaches.
func ambiguousSection(entity model.EntityView, access model.SectionAccess) (string, bool) {
	for _, label := range entity.DuplicateSectionLabels {
		if access.Touches(label) {
			return label, true
		}
	}
	return "", false
}

func duplicateSectionError(entity model.EntityView, label string, via string) *domainerrors.AppError {
	return domainerrors.New(
		domainerrors.CodeReadFailed,
		"failed to read requested content sections",
		map[string]any{
			"reason":  fmt.Sprintf("section label '%s' is duplicated", label),
			"section": label,
			"id":      entity.ID,
			"type":    entity.Type,
			"via":     via,
		},
	)
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
