// select.go decides what a read command may ask for and cuts the
// entity down to it. Selectors are checked against the active type set, so
// asking for a field no selected type declares is an error rather than a
// silently empty column.
//
// A selector is valid if any active type declares it: with several root types
// in play, requiring every type to have the field would make cross-type
// queries impossible.

package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/model"
	schemacapread "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/read"
	"github.com/anatoly-tenenev/spec-cli/internal/application/selectors"
	"github.com/anatoly-tenenev/spec-cli/internal/application/values"
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

func buildSelectTree(selects []string, capability schemacapread.Capability, activeTypeSet []string) (*model.SelectNode, *domainerrors.AppError) {
	root := selectors.NewTree()
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
		selectors.Insert(root, strings.Split(normalized, "."))
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
		if selectors.HasRefField(parts[1], capability, activeTypeSet) {
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
		if !selectors.IsRefLeaf(leaf) {
			return domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unknown projection-namespace selector '%s'", selector),
				nil,
			)
		}
		compat, exists := selectors.RefLeafCompatibility(refField, capability, activeTypeSet)
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

func projectEntity(entity map[string]any, tree *model.SelectNode) map[string]any {
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
		return values.DeepCopy(value)
	}

	sourceMap, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return projectMap(sourceMap, node)
}
