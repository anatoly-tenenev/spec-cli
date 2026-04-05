package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/support"
	schemacapread "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/read"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

var builtinSelectors = map[string]struct{}{
	"type":             {},
	"id":               {},
	"slug":             {},
	"revision":         {},
	"createdDate":      {},
	"updatedDate":      {},
	"meta":             {},
	"refs":             {},
	"content.raw":      {},
	"content.sections": {},
}

var refLeafSelectors = map[string]struct{}{
	"id":       {},
	"resolved": {},
	"type":     {},
	"slug":     {},
	"reason":   {},
}

func buildSelectTree(selects []string, capability schemacapread.Capability, activeTypeSet []string) (*model.SelectNode, *domainerrors.AppError) {
	root := &model.SelectNode{Children: map[string]*model.SelectNode{}}
	for _, selector := range selects {
		normalized := strings.TrimSpace(selector)
		if normalized == "" {
			return nil, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				"selector cannot be empty",
				nil,
			)
		}
		if err := validateSelector(normalized, capability, activeTypeSet); err != nil {
			return nil, err
		}
		insertSelector(root, strings.Split(normalized, "."))
	}
	return root, nil
}

func validateSelector(selector string, capability schemacapread.Capability, activeTypeSet []string) *domainerrors.AppError {
	if _, builtin := builtinSelectors[selector]; builtin {
		return nil
	}

	parts := strings.Split(selector, ".")
	if len(parts) == 2 && parts[0] == "meta" {
		field := parts[1]
		hasMeta := false
		for _, typeName := range activeTypeSet {
			entityType := capability.EntityTypes[typeName]
			if _, isRef := entityType.RefFields[field]; isRef {
				return domainerrors.New(
					domainerrors.CodeInvalidArgs,
					fmt.Sprintf("selector '%s' is forbidden for entityRef field", selector),
					nil,
				)
			}
			if _, exists := entityType.MetaFields[field]; exists {
				hasMeta = true
			}
		}
		if hasMeta {
			return nil
		}
		return domainerrors.New(
			domainerrors.CodeInvalidArgs,
			fmt.Sprintf("unknown projection-namespace selector '%s'", selector),
			nil,
		)
	}

	if len(parts) == 2 && parts[0] == "refs" {
		if hasRefFieldAcrossActiveSet(parts[1], capability, activeTypeSet) {
			return nil
		}
		return domainerrors.New(
			domainerrors.CodeInvalidArgs,
			fmt.Sprintf("unknown projection-namespace selector '%s'", selector),
			nil,
		)
	}

	if len(parts) == 3 && parts[0] == "refs" {
		refField := parts[1]
		leaf := parts[2]
		if _, ok := refLeafSelectors[leaf]; !ok {
			return domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unknown projection-namespace selector '%s'", selector),
				nil,
			)
		}
		compat, exists := refLeafCompatibility(refField, capability, activeTypeSet)
		if !exists {
			return domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unknown projection-namespace selector '%s'", selector),
				nil,
			)
		}
		if !compat {
			return domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("selector '%s' is forbidden: path-based ref leaf requires scalar ref in active type set", selector),
				nil,
			)
		}
		return nil
	}

	if len(parts) == 3 && parts[0] == "content" && parts[1] == "sections" {
		sectionName := parts[2]
		for _, typeName := range activeTypeSet {
			entityType := capability.EntityTypes[typeName]
			if _, exists := entityType.Sections[sectionName]; exists {
				return nil
			}
		}
		return domainerrors.New(
			domainerrors.CodeInvalidArgs,
			fmt.Sprintf("unknown projection-namespace selector '%s'", selector),
			nil,
		)
	}

	return domainerrors.New(
		domainerrors.CodeInvalidArgs,
		fmt.Sprintf("unknown projection-namespace selector '%s'", selector),
		nil,
	)
}

func hasRefFieldAcrossActiveSet(refField string, capability schemacapread.Capability, activeTypeSet []string) bool {
	for _, typeName := range activeTypeSet {
		entityType := capability.EntityTypes[typeName]
		if _, exists := entityType.RefFields[refField]; exists {
			return true
		}
	}
	return false
}

func refLeafCompatibility(refField string, capability schemacapread.Capability, activeTypeSet []string) (compatible bool, exists bool) {
	hasScalar := false
	for _, typeName := range activeTypeSet {
		entityType := capability.EntityTypes[typeName]
		refSpec, present := entityType.RefFields[refField]
		if !present {
			continue
		}
		exists = true
		if refSpec.Cardinality == schemacapread.RefCardinalityArray {
			return false, true
		}
		hasScalar = true
	}
	return hasScalar, exists
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
