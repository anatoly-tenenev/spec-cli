package execution

import (
	"fmt"
	"sort"
	"strings"

	bindingdiagnostics "github.com/anatoly-tenenev/spec-cli/internal/application/graphql/binding/internal/diagnostics"
	gqlmodel "github.com/anatoly-tenenev/spec-cli/internal/application/graphql/model"
	readmodel "github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/ordering"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func Execute(roots []gqlmodel.RootPlan, entities []readmodel.EntityView) (map[string]any, *domainerrors.AppError) {
	data := map[string]any{}
	for _, root := range roots {
		matched := make([]readmodel.EntityView, 0)
		for _, entity := range entities {
			if entity.Type != root.EntityType {
				continue
			}
			if root.Where != nil && !root.Where(entity) {
				continue
			}
			matched = append(matched, entity)
		}
		ordering.SortEntities(matched, root.Sort)
		total := len(matched)
		paged, returned := paginate(matched, root.Offset, root.Limit)
		rootPayload := map[string]any{}
		if root.Selection.Items != nil {
			items := make([]any, 0, len(paged))
			for _, entity := range paged {
				item, err := projectEntity(entity, root.Selection.Items, root.NonNullPaths)
				if err != nil {
					return nil, err
				}
				items = append(items, item)
			}
			rootPayload["items"] = items
		}
		if root.Selection.TotalCount {
			rootPayload["totalCount"] = total
		}
		if root.Selection.PageInfo != nil {
			hasMore := total > root.Offset+returned
			var nextOffset any
			if hasMore {
				nextOffset = root.Offset + returned
			}
			page := map[string]any{"limit": root.Limit, "offset": root.Offset, "returned": returned, "hasMore": hasMore, "nextOffset": nextOffset}
			rootPayload["pageInfo"] = projectMap(page, root.Selection.PageInfo)
		}
		data[root.ResponseKey] = rootPayload
	}
	return data, nil
}

func paginate(entities []readmodel.EntityView, offset int, limit int) ([]readmodel.EntityView, int) {
	if limit == 0 || offset >= len(entities) {
		return []readmodel.EntityView{}, 0
	}
	end := offset + limit
	if end > len(entities) {
		end = len(entities)
	}
	paged := entities[offset:end]
	return paged, len(paged)
}

func projectEntity(entity readmodel.EntityView, selection *gqlmodel.SelectionNode, nonNullPaths []string) (map[string]any, *domainerrors.AppError) {
	projected := projectMap(entity.View, selection)
	if err := enforceNonNull(projected, entity, nonNullPaths); err != nil {
		return nil, err
	}
	return projected, nil
}

func projectMap(source map[string]any, selection *gqlmodel.SelectionNode) map[string]any {
	out := map[string]any{}
	keys := make([]string, 0, len(selection.Children))
	for key := range selection.Children {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		child := selection.Children[key]
		sourceKey := child.SourceName
		if sourceKey == "" {
			sourceKey = key
		}
		value, exists := source[sourceKey]
		if !exists {
			out[key] = nil
			continue
		}
		if child.Terminal {
			out[key] = value
			continue
		}
		if nested, ok := value.(map[string]any); ok {
			out[key] = projectMap(nested, child)
		} else if nestedItems, ok := value.([]any); ok {
			items := make([]any, 0, len(nestedItems))
			for _, item := range nestedItems {
				if itemMap, ok := item.(map[string]any); ok {
					items = append(items, projectMap(itemMap, child))
				} else {
					items = append(items, nil)
				}
			}
			out[key] = items
		} else {
			out[key] = nil
		}
	}
	return out
}

func enforceNonNull(projected map[string]any, entity readmodel.EntityView, paths []string) *domainerrors.AppError {
	for _, path := range paths {
		if valueMissingAtPath(projected, strings.Split(path, ".")) {
			return bindingdiagnostics.InvalidResult("selected non-null field is missing", path, entity)
		}
	}
	return nil
}

func valueMissingAtPath(value any, parts []string) bool {
	if len(parts) == 0 {
		return value == nil
	}
	switch typed := value.(type) {
	case map[string]any:
		next, exists := typed[parts[0]]
		if !exists || next == nil {
			return true
		}
		return valueMissingAtPath(next, parts[1:])
	case []any:
		for _, item := range typed {
			if item == nil || valueMissingAtPath(item, parts) {
				return true
			}
		}
		return false
	default:
		return value == nil || (len(parts) > 0 && strings.TrimSpace(fmt.Sprintf("%v", value)) == "")
	}
}
