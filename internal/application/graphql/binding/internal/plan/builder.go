package plan

import (
	bindingdiagnostics "github.com/anatoly-tenenev/spec-cli/internal/application/graphql/binding/internal/diagnostics"
	"github.com/anatoly-tenenev/spec-cli/internal/application/graphql/binding/internal/predicate"
	gqlmodel "github.com/anatoly-tenenev/spec-cli/internal/application/graphql/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/graphql/sdl"
	readmodel "github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/validator/rules"
)

const maxLimit = 1000

func Build(proj *gqlmodel.Projection, query string, variables map[string]any, operationName string) ([]gqlmodel.RootPlan, *domainerrors.AppError) {
	schema, schemaErr := gqlparser.LoadSchema(&ast.Source{Name: "spec-cli.graphql", Input: sdl.Render(proj, nil)})
	if schemaErr != nil {
		return nil, domainerrors.New(domainerrors.CodeGraphQLProjectionError, "failed to load generated GraphQL SDL", map[string]any{"reason": schemaErr.Error()})
	}
	doc, validationErrs := gqlparser.LoadQueryWithRules(schema, query, rules.NewDefaultRules())
	if len(validationErrs) > 0 {
		return nil, bindingdiagnostics.InvalidQuery("invalid GraphQL query", "validation", validationErrs)
	}
	operation, err := selectOperation(doc, operationName)
	if err != nil {
		return nil, err
	}
	if operation.Operation != ast.Query {
		return nil, domainerrors.New(domainerrors.CodeInvalidQuery, "only GraphQL query operations are supported", bindingdiagnostics.GraphQLDetails("binding", operation.Position))
	}
	expandedSelections := expandSelectionSet(operation.SelectionSet, doc.Fragments, variables)
	if err := checkDirectives(expandedSelections); err != nil {
		return nil, err
	}
	roots := []gqlmodel.RootPlan{}
	rootIndexes := map[string]int{}
	for _, selection := range expandedSelections {
		field, ok := selection.(*ast.Field)
		if !ok {
			continue
		}
		if field.Name == "__schema" || field.Name == "__type" {
			return nil, domainerrors.New(domainerrors.CodeInvalidQuery, "GraphQL introspection is not supported", bindingdiagnostics.GraphQLDetails("binding", field.Position))
		}
		entity, exists := proj.Entities[field.Name]
		if !exists {
			return nil, domainerrors.New(domainerrors.CodeInvalidQuery, "unknown GraphQL root field", bindingdiagnostics.GraphQLDetails("binding", field.Position))
		}
		if !includeByDirectives(field.Directives, variables) {
			continue
		}
		args := field.ArgumentMap(variables)
		field.SelectionSet = expandSelectionSet(field.SelectionSet, doc.Fragments, variables)
		root, bindErr := bindRoot(entity, field, args, variables)
		if bindErr != nil {
			return nil, bindErr
		}
		if existingIndex, exists := rootIndexes[root.ResponseKey]; exists {
			roots[existingIndex] = mergeRootPlan(roots[existingIndex], root)
			continue
		}
		rootIndexes[root.ResponseKey] = len(roots)
		roots = append(roots, root)
	}
	return roots, nil
}

func selectOperation(doc *ast.QueryDocument, operationName string) (*ast.OperationDefinition, *domainerrors.AppError) {
	if operationName != "" {
		for _, operation := range doc.Operations {
			if operation.Name == operationName {
				return operation, nil
			}
		}
		return nil, domainerrors.New(domainerrors.CodeInvalidQuery, "GraphQL operation not found", map[string]any{"graphql": map[string]any{"phase": "binding", "operation_name": operationName}})
	}
	if len(doc.Operations) != 1 {
		return nil, domainerrors.New(domainerrors.CodeInvalidQuery, "GraphQL operation name is required when document contains multiple operations", map[string]any{"graphql": map[string]any{"phase": "binding"}})
	}
	return doc.Operations[0], nil
}

func bindRoot(entity gqlmodel.Entity, field *ast.Field, args map[string]any, variables map[string]any) (gqlmodel.RootPlan, *domainerrors.AppError) {
	responseKey := field.Name
	if field.Alias != "" {
		responseKey = field.Alias
	}
	limit := intArg(args, "limit", 100)
	offset := intArg(args, "offset", 0)
	if limit < 0 || offset < 0 {
		return gqlmodel.RootPlan{}, domainerrors.New(domainerrors.CodeInvalidQuery, "limit and offset must be non-negative", bindingdiagnostics.GraphQLDetails("binding", field.Position))
	}
	if limit > maxLimit {
		return gqlmodel.RootPlan{}, domainerrors.New(domainerrors.CodeInvalidQuery, "limit must not exceed 1000", bindingdiagnostics.GraphQLDetails("binding", field.Position))
	}
	sortTerms, sortErr := bindSort(entity, args["sort"])
	if sortErr != nil {
		return gqlmodel.RootPlan{}, sortErr
	}
	resultPredicate := predicate.Build(args["where"])
	selection, selectionErr := bindResultSelection(field.SelectionSet, variables)
	if selectionErr != nil {
		return gqlmodel.RootPlan{}, selectionErr
	}
	return gqlmodel.RootPlan{
		ResponseKey:  responseKey,
		EntityType:   entity.Name,
		Limit:        limit,
		Offset:       offset,
		Sort:         sortTerms,
		Where:        resultPredicate,
		Selection:    selection,
		NonNullPaths: collectNonNullPaths(entity, selection),
	}, nil
}

func bindResultSelection(selections ast.SelectionSet, variables map[string]any) (gqlmodel.ResultSelection, *domainerrors.AppError) {
	result := gqlmodel.ResultSelection{}
	for _, selection := range selections {
		field, ok := selection.(*ast.Field)
		if !ok {
			continue
		}
		if !includeByDirectives(field.Directives, variables) {
			continue
		}
		switch field.Name {
		case "items":
			field.SelectionSet = expandSelectionSet(field.SelectionSet, nil, variables)
			result.Items = mergeSelectionNode(result.Items, bindSelectionNode(field.SelectionSet, variables))
		case "totalCount":
			result.TotalCount = true
		case "pageInfo":
			field.SelectionSet = expandSelectionSet(field.SelectionSet, nil, variables)
			result.PageInfo = mergeSelectionNode(result.PageInfo, bindSelectionNode(field.SelectionSet, variables))
		default:
			return gqlmodel.ResultSelection{}, domainerrors.New(domainerrors.CodeInvalidQuery, "unsupported root result selection", bindingdiagnostics.GraphQLDetails("binding", field.Position))
		}
	}
	return result, nil
}

func bindSelectionNode(selections ast.SelectionSet, variables map[string]any) *gqlmodel.SelectionNode {
	node := &gqlmodel.SelectionNode{Children: map[string]*gqlmodel.SelectionNode{}}
	for _, selection := range selections {
		field, ok := selection.(*ast.Field)
		if !ok {
			continue
		}
		if !includeByDirectives(field.Directives, variables) {
			continue
		}
		key := field.Name
		if field.Alias != "" {
			key = field.Alias
		}
		child := bindSelectionNode(expandSelectionSet(field.SelectionSet, nil, variables), variables)
		child.SourceName = field.Name
		if len(field.SelectionSet) == 0 {
			child.Terminal = true
		}
		node.Children[key] = mergeSelectionNode(node.Children[key], child)
	}
	return node
}

func mergeRootPlan(dst gqlmodel.RootPlan, src gqlmodel.RootPlan) gqlmodel.RootPlan {
	dst.Selection = mergeResultSelection(dst.Selection, src.Selection)
	dst.NonNullPaths = mergeStringSet(dst.NonNullPaths, src.NonNullPaths)
	return dst
}

func mergeResultSelection(dst gqlmodel.ResultSelection, src gqlmodel.ResultSelection) gqlmodel.ResultSelection {
	dst.Items = mergeSelectionNode(dst.Items, src.Items)
	dst.TotalCount = dst.TotalCount || src.TotalCount
	dst.PageInfo = mergeSelectionNode(dst.PageInfo, src.PageInfo)
	return dst
}

func mergeSelectionNode(dst *gqlmodel.SelectionNode, src *gqlmodel.SelectionNode) *gqlmodel.SelectionNode {
	if src == nil {
		return dst
	}
	if dst == nil {
		return src
	}
	if dst.SourceName == "" {
		dst.SourceName = src.SourceName
	}
	if dst.Children == nil {
		dst.Children = map[string]*gqlmodel.SelectionNode{}
	}
	if len(src.Children) > 0 {
		dst.Terminal = false
	} else if len(dst.Children) == 0 && src.Terminal {
		dst.Terminal = true
	}
	for key, child := range src.Children {
		dst.Children[key] = mergeSelectionNode(dst.Children[key], child)
	}
	if len(dst.Children) > 0 {
		dst.Terminal = false
	}
	return dst
}

func mergeStringSet(dst []string, src []string) []string {
	seen := map[string]struct{}{}
	for _, value := range dst {
		seen[value] = struct{}{}
	}
	for _, value := range src {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		dst = append(dst, value)
	}
	return dst
}

func expandSelectionSet(selections ast.SelectionSet, fragments ast.FragmentDefinitionList, variables map[string]any) ast.SelectionSet {
	expanded := ast.SelectionSet{}
	for _, selection := range selections {
		switch typed := selection.(type) {
		case *ast.Field:
			copyField := *typed
			copyField.SelectionSet = expandSelectionSet(typed.SelectionSet, fragments, variables)
			expanded = append(expanded, &copyField)
		case *ast.InlineFragment:
			if includeByDirectives(typed.Directives, variables) {
				expanded = append(expanded, expandSelectionSet(typed.SelectionSet, fragments, variables)...)
			}
		case *ast.FragmentSpread:
			for _, fragment := range fragments {
				if fragment.Name == typed.Name && includeByDirectives(typed.Directives, variables) {
					expanded = append(expanded, expandSelectionSet(fragment.SelectionSet, fragments, variables)...)
					break
				}
			}
		}
	}
	return expanded
}

func bindSort(entity gqlmodel.Entity, raw any) ([]readmodel.SortTerm, *domainerrors.AppError) {
	fieldPaths := map[string]string{}
	for _, field := range entity.SortFields {
		fieldPaths[field.Name] = field.Path
	}
	if raw == nil {
		return []readmodel.SortTerm{{Path: "id", Direction: readmodel.SortDirectionAsc}}, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, domainerrors.New(domainerrors.CodeInvalidQuery, "sort must be a list", map[string]any{"graphql": map[string]any{"phase": "binding"}})
	}
	terms := make([]readmodel.SortTerm, 0, len(list)+1)
	for _, item := range list {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, domainerrors.New(domainerrors.CodeInvalidQuery, "sort item must be an object", map[string]any{"graphql": map[string]any{"phase": "binding"}})
		}
		fieldName, _ := obj["field"].(string)
		path, exists := fieldPaths[fieldName]
		if !exists {
			return nil, domainerrors.New(domainerrors.CodeInvalidQuery, "unknown sort field", map[string]any{"graphql": map[string]any{"phase": "binding", "field": fieldName}})
		}
		direction := readmodel.SortDirectionAsc
		if rawDirection, ok := obj["direction"].(string); ok && rawDirection == "desc" {
			direction = readmodel.SortDirectionDesc
		}
		terms = append(terms, readmodel.SortTerm{Path: path, Direction: direction})
	}
	if len(terms) == 0 || terms[len(terms)-1].Path != "id" || terms[len(terms)-1].Direction != readmodel.SortDirectionAsc {
		terms = append(terms, readmodel.SortTerm{Path: "id", Direction: readmodel.SortDirectionAsc})
	}
	return terms, nil
}

func checkDirectives(selections ast.SelectionSet) *domainerrors.AppError {
	for _, selection := range selections {
		switch typed := selection.(type) {
		case *ast.Field:
			for _, directive := range typed.Directives {
				if directive.Name != "include" && directive.Name != "skip" {
					return domainerrors.New(domainerrors.CodeInvalidQuery, "unsupported GraphQL directive", bindingdiagnostics.GraphQLDetails("binding", directive.Position))
				}
			}
			if err := checkDirectives(typed.SelectionSet); err != nil {
				return err
			}
		case *ast.InlineFragment:
			if err := checkDirectives(typed.SelectionSet); err != nil {
				return err
			}
		}
	}
	return nil
}

func includeByDirectives(directives ast.DirectiveList, variables map[string]any) bool {
	include := true
	for _, directive := range directives {
		args := directive.ArgumentMap(variables)
		value, _ := args["if"].(bool)
		if directive.Name == "skip" && value {
			include = false
		}
		if directive.Name == "include" && !value {
			include = false
		}
	}
	return include
}

func intArg(args map[string]any, name string, fallback int) int {
	raw, exists := args[name]
	if !exists || raw == nil {
		return fallback
	}
	switch typed := raw.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return fallback
	}
}

func collectNonNullPaths(entity gqlmodel.Entity, selection gqlmodel.ResultSelection) []string {
	if selection.Items == nil {
		return nil
	}
	paths := []string{}
	for _, field := range []string{"type", "id", "slug", "revision", "createdDate", "updatedDate"} {
		if _, selected := selection.Items.Children[field]; selected {
			paths = append(paths, field)
		}
	}
	if metaNode, selected := selection.Items.Children["meta"]; selected {
		required := map[string]struct{}{}
		for _, field := range entity.MetaFields {
			if field.Required {
				required[field.Name] = struct{}{}
			}
		}
		for responseKey, child := range metaNode.Children {
			name := sourceName(responseKey, child)
			if _, ok := required[name]; ok {
				paths = append(paths, "meta."+responseKey)
			}
		}
	}
	if refsNode, selected := selection.Items.Children["refs"]; selected {
		required := map[string]bool{}
		for _, field := range entity.RefFields {
			required[field.Name] = field.Required
		}
		for responseKey, child := range refsNode.Children {
			name := sourceName(responseKey, child)
			if required[name] {
				paths = append(paths, "refs."+responseKey)
				for leafKey := range child.Children {
					paths = append(paths, "refs."+responseKey+"."+leafKey)
				}
			}
		}
	}
	if contentNode, selected := selection.Items.Children["content"]; selected {
		if sectionsNode, ok := contentNode.Children["sections"]; ok {
			required := map[string]struct{}{}
			for _, section := range entity.Sections {
				if section.Required {
					required[section.Name] = struct{}{}
				}
			}
			for responseKey, child := range sectionsNode.Children {
				name := sourceName(responseKey, child)
				if _, ok := required[name]; ok {
					paths = append(paths, "content.sections."+responseKey)
				}
			}
		}
	}
	return paths
}

func sourceName(responseKey string, node *gqlmodel.SelectionNode) string {
	if node != nil && node.SourceName != "" {
		return node.SourceName
	}
	return responseKey
}
