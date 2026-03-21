package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func buildSelectTree(selects []string, index model.QuerySchemaIndex) (*model.SelectNode, *domainerrors.AppError) {
	root := &model.SelectNode{Children: map[string]*model.SelectNode{}}
	for _, selector := range selects {
		normalized := strings.TrimSpace(selector)
		if _, exists := index.SelectorPaths[normalized]; !exists && !isHiddenRefLeafSelector(normalized, index) {
			return nil, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unknown projection-namespace selector '%s'", normalized),
				nil,
			)
		}
		insertSelector(root, strings.Split(normalized, "."))
	}
	return root, nil
}

func isHiddenRefLeafSelector(path string, index model.QuerySchemaIndex) bool {
	parts := strings.Split(path, ".")
	if len(parts) != 3 || parts[0] != "refs" {
		return false
	}
	switch parts[2] {
	case "id", "resolved", "type", "slug":
	default:
		return false
	}
	_, exists := index.FilterFields[path]
	return exists
}

func insertSelector(root *model.SelectNode, parts []string) {
	current := root
	for idx, part := range parts {
		if current.Terminal {
			return
		}

		child, exists := current.Children[part]
		if !exists {
			child = &model.SelectNode{Children: map[string]*model.SelectNode{}}
			current.Children[part] = child
		}

		if idx == len(parts)-1 {
			child.Terminal = true
			child.Children = map[string]*model.SelectNode{}
			return
		}
		current = child
	}
}

func ProjectEntity(entity map[string]any, tree *model.SelectNode) map[string]any {
	projected := projectMap(entity, tree)
	if projected == nil {
		return map[string]any{}
	}
	return projected
}

func projectMap(source map[string]any, node *model.SelectNode) map[string]any {
	if node == nil {
		return map[string]any{}
	}

	output := map[string]any{}
	keys := make([]string, 0, len(node.Children))
	for key := range node.Children {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		child := node.Children[key]
		value, exists := source[key]
		if !exists {
			output[key] = nil
			continue
		}

		output[key] = projectValue(value, child)
	}
	return output
}

func projectValue(value any, node *model.SelectNode) any {
	if node == nil {
		return nil
	}
	if node.Terminal {
		return support.DeepCopy(value)
	}

	sourceMap, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return projectMap(sourceMap, node)
}
