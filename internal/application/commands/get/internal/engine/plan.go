package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/get/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/get/internal/support"
	schemacapread "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/read"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

var defaultSelectors = []string{
	"type",
	"id",
	"slug",
	"meta",
	"refs",
}

var builtinSelectors = []string{
	"type",
	"id",
	"slug",
	"createdDate",
	"updatedDate",
	"revision",
	"meta",
	"refs",
	"content.raw",
	"content.sections",
}

var refLeafSelectors = map[string]struct{}{
	"id":       {},
	"resolved": {},
	"type":     {},
	"slug":     {},
	"reason":   {},
}

func BuildSelectorPlan(rawSelectors []string, readCapability schemacapread.Capability) (model.SelectorPlan, *domainerrors.AppError) {
	selectors := rawSelectors
	if len(selectors) == 0 {
		selectors = append([]string(nil), defaultSelectors...)
	}

	activeTypeSet := support.SortedMapKeys(readCapability.EntityTypes)
	allowedSelectors := buildAllowedSelectors(readCapability, activeTypeSet)

	root := &model.SelectNode{Children: map[string]*model.SelectNode{}}
	for _, selector := range selectors {
		normalized := strings.TrimSpace(selector)
		if err := validateSelector(normalized, allowedSelectors, readCapability, activeTypeSet); err != nil {
			return model.SelectorPlan{}, err
		}
		insertSelector(root, strings.Split(normalized, "."))
	}

	effectiveSelectors := collectTerminalSelectors(root)
	nullIfMissing := map[string]struct{}{}
	requiredRefFields := map[string]struct{}{}
	requiresAllRefFields := false
	requiredSectionNames := map[string]struct{}{}

	requiresRefs := false
	requiresSections := false
	requiresAllSections := false
	requiresContentRaw := false
	requiresContent := false

	for _, selector := range effectiveSelectors {
		parts := strings.Split(selector, ".")
		if len(parts) == 0 {
			continue
		}

		switch parts[0] {
		case "meta":
			if len(parts) == 2 {
				nullIfMissing[selector] = struct{}{}
			}
		case "refs":
			requiresRefs = true
			if len(parts) == 1 {
				requiresAllRefFields = true
			}
			if len(parts) >= 2 {
				requiredRefFields[parts[1]] = struct{}{}
			}
			if len(parts) == 2 {
				nullIfMissing[selector] = struct{}{}
			}
			if len(parts) == 3 {
				nullIfMissing["refs."+parts[1]] = struct{}{}
			}
		case "content":
			requiresContent = true
			if len(parts) >= 2 && parts[1] == "raw" {
				requiresContentRaw = true
			}
			if len(parts) >= 2 && parts[1] == "sections" {
				requiresSections = true
				if len(parts) == 2 {
					requiresAllSections = true
				}
				if len(parts) == 3 {
					requiredSectionNames[parts[2]] = struct{}{}
					nullIfMissing[selector] = struct{}{}
				}
			}
		}
	}

	return model.SelectorPlan{
		Tree:                 root,
		EffectiveSelectors:   effectiveSelectors,
		NullIfMissingPaths:   nullIfMissing,
		RequiredRefFields:    requiredRefFields,
		RequiresAllRefFields: requiresAllRefFields,
		RequiredSectionNames: requiredSectionNames,
		RequiresRefs:         requiresRefs,
		RequiresSections:     requiresSections,
		RequiresAllSections:  requiresAllSections,
		RequiresContent:      requiresContent,
		RequiresContentRaw:   requiresContentRaw,
	}, nil
}

func validateSelector(
	selector string,
	allowedSelectors map[string]struct{},
	readCapability schemacapread.Capability,
	activeTypeSet []string,
) *domainerrors.AppError {
	if _, exists := allowedSelectors[selector]; !exists {
		return domainerrors.New(
			domainerrors.CodeInvalidArgs,
			fmt.Sprintf("unknown selector '%s'", selector),
			nil,
		)
	}

	parts := strings.Split(selector, ".")
	if len(parts) != 3 || parts[0] != "refs" {
		return nil
	}

	if _, ok := refLeafSelectors[parts[2]]; !ok {
		return nil
	}

	compatible, exists := refLeafCompatibility(parts[1], readCapability, activeTypeSet)
	if !exists {
		return domainerrors.New(
			domainerrors.CodeInvalidArgs,
			fmt.Sprintf("unknown selector '%s'", selector),
			nil,
		)
	}
	if compatible {
		return nil
	}

	return domainerrors.New(
		domainerrors.CodeInvalidArgs,
		fmt.Sprintf("selector '%s' is forbidden: path-based ref leaf requires scalar ref across schema read model", selector),
		nil,
	)
}

func buildAllowedSelectors(
	readCapability schemacapread.Capability,
	activeTypeSet []string,
) map[string]struct{} {
	allowedSelectors := map[string]struct{}{}
	for _, selector := range builtinSelectors {
		allowedSelectors[selector] = struct{}{}
	}

	for _, typeName := range activeTypeSet {
		entityType := readCapability.EntityTypes[typeName]
		for field := range entityType.MetaFields {
			allowedSelectors["meta."+field] = struct{}{}
		}
		for field, refSpec := range entityType.RefFields {
			allowedSelectors["refs."+field] = struct{}{}
			if refSpec.Cardinality == schemacapread.RefCardinalityArray {
				continue
			}
			for leaf := range refLeafSelectors {
				allowedSelectors["refs."+field+"."+leaf] = struct{}{}
			}
		}
		for section := range entityType.Sections {
			allowedSelectors["content.sections."+section] = struct{}{}
		}
	}

	return allowedSelectors
}

func refLeafCompatibility(
	refField string,
	readCapability schemacapread.Capability,
	activeTypeSet []string,
) (compatible bool, exists bool) {
	hasScalar := false
	for _, typeName := range activeTypeSet {
		entityType := readCapability.EntityTypes[typeName]
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

func collectTerminalSelectors(root *model.SelectNode) []string {
	if root == nil {
		return nil
	}

	selectors := make([]string, 0)
	collectSelectors(root, "", &selectors)
	sort.Strings(selectors)
	return selectors
}

func collectSelectors(node *model.SelectNode, path string, out *[]string) {
	keys := make([]string, 0, len(node.Children))
	for key := range node.Children {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		child := node.Children[key]
		nextPath := key
		if path != "" {
			nextPath = path + "." + key
		}
		if child.Terminal {
			*out = append(*out, nextPath)
			continue
		}
		collectSelectors(child, nextPath, out)
	}
}

func ProjectEntity(entity map[string]any, tree *model.SelectNode, nullIfMissing map[string]struct{}) map[string]any {
	projected, ok := projectMap(entity, tree, "", nullIfMissing)
	if !ok {
		return map[string]any{}
	}
	return projected
}

func projectMap(source map[string]any, node *model.SelectNode, prefix string, nullIfMissing map[string]struct{}) (map[string]any, bool) {
	if node == nil {
		return map[string]any{}, false
	}

	keys := make([]string, 0, len(node.Children))
	for key := range node.Children {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := map[string]any{}
	for _, key := range keys {
		child := node.Children[key]
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		value, exists := source[key]
		if !exists {
			if _, shouldMaterializeNull := nullIfMissing[path]; shouldMaterializeNull {
				out[key] = nil
			}
			continue
		}

		projected, include := projectValue(value, child, path, nullIfMissing)
		if include {
			out[key] = projected
			continue
		}
		if _, shouldMaterializeNull := nullIfMissing[path]; shouldMaterializeNull {
			out[key] = nil
		}
	}

	return out, len(out) > 0
}

func projectValue(value any, node *model.SelectNode, prefix string, nullIfMissing map[string]struct{}) (any, bool) {
	if node == nil {
		return nil, false
	}
	if node.Terminal {
		return support.DeepCopy(value), true
	}

	typed, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	return projectMap(typed, node, prefix, nullIfMissing)
}
