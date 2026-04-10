package shared

import (
	"fmt"
	"sort"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/derivedschema"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/diagnostics"
	schemaexpressions "github.com/anatoly-tenenev/spec-cli/internal/application/schema/expressions"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
	"gopkg.in/yaml.v3"
)

func ParseValueSpec(
	node *yaml.Node,
	path string,
	typeSet map[string]struct{},
	allowArray bool,
	issues *[]diagnostics.Issue,
) model.ValueSpec {
	values, ok := MappingValues(node, path, issues)
	if !ok {
		return model.ValueSpec{Kind: model.ValueKindUnknown}
	}
	AppendUnsupportedKeys(
		values,
		path,
		SetOf("type", "format", "enum", "const", "refType", "refTypes", "items", "uniqueItems", "minItems", "maxItems", "description"),
		issues,
	)

	typeNode, hasType := values["type"]
	if !hasType {
		AddError(issues, "schema.value.type_required", "schema.type is required", path+".type")
		return model.ValueSpec{Kind: model.ValueKindUnknown}
	}
	typeName, typeValid := ScalarString(typeNode, path+".type", true, issues)
	if !typeValid {
		return model.ValueSpec{Kind: model.ValueKindUnknown}
	}

	kind := kindFromTypeName(typeName)
	if kind == model.ValueKindUnknown {
		AddError(
			issues,
			"schema.value.type_unsupported",
			fmt.Sprintf("unsupported schema type '%s'", typeName),
			path+".type",
		)
	}

	if _, exists := values["refTypes"]; exists {
		AddError(
			issues,
			"schema.value.ref_type_legacy",
			"schema.refTypes is not supported; use schema.refType",
			path+".refTypes",
		)
	}

	spec := model.ValueSpec{Kind: kind}
	if formatNode, exists := values["format"]; exists {
		if format, isValid := ScalarString(formatNode, path+".format", true, issues); isValid {
			spec.Format = format
		}
	}

	if constNode, exists := values["const"]; exists {
		if value, isValid := LiteralValue(constNode, path+".const", issues); isValid {
			spec.Const = &model.Literal{Value: value}
		}
	}

	if enumNode, exists := values["enum"]; exists {
		spec.Enum = parseEnum(enumNode, path+".enum", issues)
	}

	if spec.Const != nil && !derivedschema.LiteralMatchesKind(spec.Const.Value, kind) {
		AddError(
			issues,
			"schema.value.const_type_mismatch",
			"schema.const value type is incompatible with schema.type",
			path+".const",
		)
	}
	for index, enumValue := range spec.Enum {
		if derivedschema.LiteralMatchesKind(enumValue.Value, kind) {
			continue
		}
		AddError(
			issues,
			"schema.value.enum_type_mismatch",
			"schema.enum value type is incompatible with schema.type",
			fmt.Sprintf("%s[%d]", path+".enum", index),
		)
	}

	if kind == model.ValueKindEntityRef {
		refTypes := parseRefType(values["refType"], path+".refType", typeSet, issues)
		spec.Ref = &model.RefSpec{
			Cardinality:  model.RefCardinalityScalar,
			AllowedTypes: refTypes,
		}
		assertArrayOnlyKeysUnused(values, path, issues)
		return spec
	}

	if _, exists := values["refType"]; exists {
		AddError(
			issues,
			"schema.value.ref_type_unexpected",
			"schema.refType is allowed only for schema.type=entityRef",
			path+".refType",
		)
	}

	if kind != model.ValueKindArray {
		assertArrayOnlyKeysUnused(values, path, issues)
		return spec
	}

	if !allowArray {
		AddError(issues, "schema.value.array_nested_unsupported", "nested array schema is not supported", path)
		return spec
	}

	itemsNode, hasItems := values["items"]
	if !hasItems {
		AddError(issues, "schema.value.items_required", "schema.items is required for schema.type=array", path+".items")
	} else {
		itemSpec := ParseValueSpec(itemsNode, path+".items", typeSet, false, issues)
		spec.Items = &itemSpec
	}

	if uniqueItemsNode, exists := values["uniqueItems"]; exists {
		if value, isValid := ScalarBool(uniqueItemsNode, path+".uniqueItems", issues); isValid {
			spec.UniqueItems = value
		}
	}
	if minItemsNode, exists := values["minItems"]; exists {
		if minItems, isValid := ScalarNonNegativeInt(minItemsNode, path+".minItems", issues); isValid {
			spec.MinItems = &minItems
		}
	}
	if maxItemsNode, exists := values["maxItems"]; exists {
		if maxItems, isValid := ScalarNonNegativeInt(maxItemsNode, path+".maxItems", issues); isValid {
			spec.MaxItems = &maxItems
		}
	}
	if spec.MinItems != nil && spec.MaxItems != nil && *spec.MinItems > *spec.MaxItems {
		AddError(
			issues,
			"schema.value.min_max_items_invalid",
			"schema.minItems must be less than or equal to schema.maxItems",
			path,
		)
	}

	return spec
}

func ParseRequirement(node *yaml.Node, path string, defaultValue bool, issues *[]diagnostics.Issue) model.Requirement {
	if node == nil {
		return model.Requirement{Always: defaultValue, Path: path}
	}
	if node.Kind != yaml.ScalarNode {
		AddError(issues, "schema.requirement.invalid", "required must be boolean or ${expr}", path)
		return model.Requirement{Always: false, Path: path}
	}

	var decoded any
	if err := node.Decode(&decoded); err != nil {
		AddError(issues, "schema.requirement.invalid", "failed to parse required value", path)
		return model.Requirement{Always: false, Path: path}
	}

	switch typed := decoded.(type) {
	case bool:
		return model.Requirement{Always: typed, Path: path}
	case string:
		expr := compileSingleExpression(typed, path, issues)
		return model.Requirement{Always: false, Expr: expr, Path: path}
	default:
		AddError(issues, "schema.requirement.invalid", "required must be boolean or ${expr}", path)
		return model.Requirement{Always: false, Path: path}
	}
}

func CompileTemplate(raw string, path string, issues *[]diagnostics.Issue) *schemaexpressions.CompiledTemplate {
	parts := make([]schemaexpressions.TemplatePart, 0)
	rest := raw
	for {
		idx := strings.Index(rest, "${")
		if idx < 0 {
			if rest != "" {
				parts = append(parts, schemaexpressions.TemplatePart{Literal: rest})
			}
			break
		}
		if idx > 0 {
			parts = append(parts, schemaexpressions.TemplatePart{Literal: rest[:idx]})
		}
		rest = rest[idx+2:]
		end := strings.Index(rest, "}")
		if end < 0 {
			AddError(issues, "schema.template.invalid", "unterminated interpolation in template", path)
			return nil
		}
		expr := strings.TrimSpace(rest[:end])
		if expr == "" {
			AddError(issues, "schema.template.empty_expression", "template interpolation expression must not be empty", path)
			return nil
		}
		parts = append(parts, schemaexpressions.TemplatePart{
			Expression: &schemaexpressions.CompiledExpression{
				Source: expr,
				Mode:   schemaexpressions.CompileModeTemplatePart,
			},
		})
		rest = rest[end+1:]
	}
	if len(parts) == 0 {
		return nil
	}
	for _, part := range parts {
		if part.Expression != nil {
			return &schemaexpressions.CompiledTemplate{Raw: raw, Parts: parts}
		}
	}
	return nil
}

func LiteralValue(node *yaml.Node, path string, issues *[]diagnostics.Issue) (any, bool) {
	var decoded any
	if err := node.Decode(&decoded); err != nil {
		AddError(issues, "schema.value.literal_invalid", "failed to decode literal value", path)
		return nil, false
	}
	return decoded, true
}

func ScalarString(node *yaml.Node, path string, nonEmpty bool, issues *[]diagnostics.Issue) (string, bool) {
	if node == nil || node.Kind != yaml.ScalarNode {
		AddError(issues, "schema.value.string_invalid", "value must be a string", path)
		return "", false
	}
	var value string
	if err := node.Decode(&value); err != nil {
		AddError(issues, "schema.value.string_invalid", "value must be a string", path)
		return "", false
	}
	if nonEmpty && strings.TrimSpace(value) == "" {
		AddError(issues, "schema.value.string_empty", "value must be a non-empty string", path)
		return "", false
	}
	return value, true
}

func ScalarBool(node *yaml.Node, path string, issues *[]diagnostics.Issue) (bool, bool) {
	if node == nil || node.Kind != yaml.ScalarNode {
		AddError(issues, "schema.value.bool_invalid", "value must be a boolean", path)
		return false, false
	}
	var value bool
	if err := node.Decode(&value); err != nil {
		AddError(issues, "schema.value.bool_invalid", "value must be a boolean", path)
		return false, false
	}
	return value, true
}

func ScalarNonNegativeInt(node *yaml.Node, path string, issues *[]diagnostics.Issue) (int, bool) {
	if node == nil || node.Kind != yaml.ScalarNode {
		AddError(issues, "schema.value.int_invalid", "value must be an integer >= 0", path)
		return 0, false
	}
	var value int
	if err := node.Decode(&value); err != nil {
		AddError(issues, "schema.value.int_invalid", "value must be an integer >= 0", path)
		return 0, false
	}
	if value < 0 {
		AddError(issues, "schema.value.int_negative", "value must be an integer >= 0", path)
		return 0, false
	}
	return value, true
}

func MappingValues(node *yaml.Node, path string, issues *[]diagnostics.Issue) (map[string]*yaml.Node, bool) {
	if node == nil || node.Kind != yaml.MappingNode {
		AddError(issues, "schema.value.mapping_invalid", "value must be a mapping object", path)
		return nil, false
	}
	values := make(map[string]*yaml.Node, len(node.Content)/2)
	for idx := 0; idx+1 < len(node.Content); idx += 2 {
		keyNode := node.Content[idx]
		valueNode := node.Content[idx+1]
		values[keyNode.Value] = valueNode
	}
	return values, true
}

func AppendUnsupportedKeys(values map[string]*yaml.Node, path string, allowed map[string]struct{}, issues *[]diagnostics.Issue) {
	unsupported := make([]string, 0)
	for key := range values {
		if _, exists := allowed[key]; exists {
			continue
		}
		unsupported = append(unsupported, key)
	}
	sort.Strings(unsupported)
	for _, key := range unsupported {
		AddError(
			issues,
			"schema.keys.unsupported",
			fmt.Sprintf("unsupported key '%s'", key),
			path+"."+key,
		)
	}
}

func SortedKeys(values map[string]*yaml.Node) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func OrderedKeys(values map[string]*yaml.Node, mappingNode *yaml.Node) []string {
	if mappingNode == nil || mappingNode.Kind != yaml.MappingNode {
		return SortedKeys(values)
	}

	keys := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for idx := 0; idx+1 < len(mappingNode.Content); idx += 2 {
		key := mappingNode.Content[idx].Value
		if _, exists := values[key]; !exists {
			continue
		}
		keys = append(keys, key)
		seen[key] = struct{}{}
	}
	if len(seen) == len(values) {
		return keys
	}

	for _, key := range SortedKeys(values) {
		if _, exists := seen[key]; exists {
			continue
		}
		keys = append(keys, key)
	}
	return keys
}

func SetOf(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func HasWhitespace(value string) bool {
	for _, symbol := range value {
		if symbol == ' ' || symbol == '\t' || symbol == '\n' || symbol == '\r' {
			return true
		}
	}
	return false
}

func AddError(issues *[]diagnostics.Issue, code string, message string, path string) {
	*issues = append(*issues, diagnostics.NewError(code, message, path))
}

func AddWarning(issues *[]diagnostics.Issue, code string, message string, path string) {
	*issues = append(*issues, diagnostics.NewWarning(code, message, path))
}

func kindFromTypeName(typeName string) model.ValueKind {
	switch strings.TrimSpace(typeName) {
	case string(model.ValueKindString):
		return model.ValueKindString
	case string(model.ValueKindNumber):
		return model.ValueKindNumber
	case string(model.ValueKindInteger):
		return model.ValueKindInteger
	case string(model.ValueKindBoolean):
		return model.ValueKindBoolean
	case string(model.ValueKindArray):
		return model.ValueKindArray
	case string(model.ValueKindEntityRef):
		return model.ValueKindEntityRef
	default:
		return model.ValueKindUnknown
	}
}

func parseEnum(node *yaml.Node, path string, issues *[]diagnostics.Issue) []model.Literal {
	if node.Kind != yaml.SequenceNode {
		AddError(issues, "schema.value.enum_invalid", "schema.enum must be a non-empty array", path)
		return nil
	}
	if len(node.Content) == 0 {
		AddError(issues, "schema.value.enum_empty", "schema.enum must be a non-empty array", path)
		return nil
	}

	values := make([]model.Literal, 0, len(node.Content))
	for index, itemNode := range node.Content {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		value, isValid := LiteralValue(itemNode, itemPath, issues)
		if !isValid {
			continue
		}
		values = append(values, model.Literal{Value: value})
	}
	return values
}

func parseRefType(node *yaml.Node, path string, typeSet map[string]struct{}, issues *[]diagnostics.Issue) []string {
	if node == nil {
		return nil
	}

	seen := map[string]struct{}{}
	values := make([]string, 0)

	switch node.Kind {
	case yaml.ScalarNode:
		value, ok := parseRefTypeItem(node, path, issues)
		if !ok {
			return nil
		}
		seen[value] = struct{}{}
		if _, known := typeSet[value]; !known {
			AddError(
				issues,
				"schema.value.ref_type_unknown",
				fmt.Sprintf("unknown entity type '%s' in refType", value),
				path,
			)
		}
		values = append(values, value)
	case yaml.SequenceNode:
		if len(node.Content) == 0 {
			AddError(issues, "schema.value.ref_type_empty", "schema.refType must be a non-empty array", path)
			return nil
		}

		values = make([]string, 0, len(node.Content))
		seen = make(map[string]struct{}, len(node.Content))
		for index, itemNode := range node.Content {
			itemPath := fmt.Sprintf("%s[%d]", path, index)
			value, ok := parseRefTypeItem(itemNode, itemPath, issues)
			if !ok {
				continue
			}
			if _, exists := seen[value]; exists {
				AddError(
					issues,
					"schema.value.ref_type_duplicate",
					fmt.Sprintf("duplicate ref type '%s'", value),
					itemPath,
				)
				continue
			}
			seen[value] = struct{}{}
			if _, known := typeSet[value]; !known {
				AddError(
					issues,
					"schema.value.ref_type_unknown",
					fmt.Sprintf("unknown entity type '%s' in refType", value),
					itemPath,
				)
			}
			values = append(values, value)
		}
	default:
		AddError(issues, "schema.value.ref_type_invalid", "schema.refType must be a string or non-empty array of strings", path)
		return nil
	}

	sort.Strings(values)
	return values
}

func parseRefTypeItem(node *yaml.Node, path string, issues *[]diagnostics.Issue) (string, bool) {
	if node == nil || node.Kind != yaml.ScalarNode {
		AddError(issues, "schema.value.ref_type_invalid", "schema.refType must contain string values", path)
		return "", false
	}

	var value string
	if err := node.Decode(&value); err != nil {
		AddError(issues, "schema.value.ref_type_invalid", "schema.refType must contain string values", path)
		return "", false
	}
	if strings.TrimSpace(value) == "" {
		AddError(issues, "schema.value.ref_type_empty", "schema.refType values must be non-empty strings", path)
		return "", false
	}
	return value, true
}

func compileSingleExpression(raw string, path string, issues *[]diagnostics.Issue) *schemaexpressions.CompiledExpression {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "${") || !strings.HasSuffix(trimmed, "}") {
		AddError(issues, "schema.expression.invalid", "expression must have form ${expr}", path)
		return nil
	}
	inner := strings.TrimSpace(trimmed[2 : len(trimmed)-1])
	if inner == "" {
		AddError(issues, "schema.expression.empty", "expression must not be empty", path)
		return nil
	}
	return &schemaexpressions.CompiledExpression{
		Source: inner,
		Mode:   schemaexpressions.CompileModeScalar,
	}
}

func assertArrayOnlyKeysUnused(values map[string]*yaml.Node, path string, issues *[]diagnostics.Issue) {
	if _, exists := values["items"]; exists {
		AddError(issues, "schema.value.items_unexpected", "schema.items is allowed only for schema.type=array", path+".items")
	}
	if _, exists := values["uniqueItems"]; exists {
		AddError(issues, "schema.value.unique_items_unexpected", "schema.uniqueItems is allowed only for schema.type=array", path+".uniqueItems")
	}
	if _, exists := values["minItems"]; exists {
		AddError(issues, "schema.value.min_items_unexpected", "schema.minItems is allowed only for schema.type=array", path+".minItems")
	}
	if _, exists := values["maxItems"]; exists {
		AddError(issues, "schema.value.max_items_unexpected", "schema.maxItems is allowed only for schema.type=array", path+".maxItems")
	}
}
