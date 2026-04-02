package engine

import (
	"fmt"
	"reflect"
	"strings"

	jmespath "github.com/anatoly-tenenev/go-jmespath"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

const (
	astNodeField           = 5
	astNodeIndexExpr       = 10
	astNodeSubexpression   = 20
	astNodeValueProjection = 22
)

var unresolvedReasons = []any{"missing", "ambiguous", "type_mismatch"}

func compileWhereExpression(raw string, index model.QuerySchemaIndex, activeTypeSet []string) (*model.WherePlan, *domainerrors.AppError) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	if err := validateWherePolicy(trimmed, index, activeTypeSet); err != nil {
		return nil, err
	}

	schema := buildWhereItemSchema(index, activeTypeSet)
	compiledSchema, schemaErr := jmespath.CompileSchema(schema)
	if schemaErr != nil {
		return nil, domainerrors.New(
			domainerrors.CodeInvalidQuery,
			fmt.Sprintf("failed to compile where schema context: %s", schemaErr.Error()),
			nil,
		)
	}

	query, compileErr := jmespath.CompileWithCompiledSchema(trimmed, compiledSchema)
	if compileErr != nil {
		return nil, domainerrors.New(
			domainerrors.CodeInvalidQuery,
			compileErr.Error(),
			nil,
		)
	}

	return &model.WherePlan{Source: trimmed, Query: query}, nil
}

func validateWherePolicy(expression string, index model.QuerySchemaIndex, activeTypeSet []string) *domainerrors.AppError {
	refFields := map[string]struct{}{}
	for _, typeName := range activeTypeSet {
		entityType := index.EntityTypes[typeName]
		for refField := range entityType.RefFields {
			refFields[refField] = struct{}{}
		}
	}

	parser := jmespath.NewParser()
	ast, err := parser.Parse(expression)
	if err != nil {
		return domainerrors.New(
			domainerrors.CodeInvalidQuery,
			err.Error(),
			nil,
		)
	}

	fieldChains := collectFieldChains(reflect.ValueOf(ast))
	for _, chain := range fieldChains {
		if len(chain) == 0 {
			continue
		}
		if chain[0] == "content" {
			if len(chain) < 2 {
				return domainerrors.New(
					domainerrors.CodeInvalidQuery,
					"forbidden where path: root 'content' is not allowed; use content.sections...",
					nil,
				)
			}
			if chain[1] != "sections" {
				return domainerrors.New(
					domainerrors.CodeInvalidQuery,
					fmt.Sprintf("forbidden where path: '%s' is not allowed; only content.sections... is supported", strings.Join(chain, ".")),
					nil,
				)
			}
		}
		if chain[0] == "meta" && len(chain) >= 2 {
			if _, isRef := refFields[chain[1]]; isRef {
				return domainerrors.New(
					domainerrors.CodeInvalidQuery,
					fmt.Sprintf("forbidden where path: 'meta.%s' points to entityRef; use refs.%s", chain[1], chain[1]),
					nil,
				)
			}
		}
	}

	return nil
}

func collectFieldChains(node reflect.Value) [][]string {
	if !node.IsValid() {
		return nil
	}
	if node.Kind() == reflect.Interface {
		if node.IsNil() {
			return nil
		}
		return collectFieldChains(node.Elem())
	}
	if node.Kind() != reflect.Struct {
		return nil
	}

	children := node.FieldByName("children")
	nodeType := int(node.FieldByName("nodeType").Int())

	switch nodeType {
	case astNodeField:
		if name, ok := fieldNodeName(node); ok {
			return [][]string{{name}}
		}
		return nil
	case astNodeSubexpression, astNodeValueProjection, astNodeIndexExpr:
		if children.IsValid() && children.Kind() == reflect.Slice && children.Len() >= 2 {
			left := collectFieldChains(children.Index(0))
			right := collectFieldChains(children.Index(1))
			chains := combineFieldChains(left, right)
			if len(chains) == 0 {
				chains = append(chains, left...)
				chains = append(chains, right...)
			}
			for idx := 2; idx < children.Len(); idx++ {
				chains = append(chains, collectFieldChains(children.Index(idx))...)
			}
			return dedupeFieldChains(chains)
		}
	}

	chains := make([][]string, 0, 4)
	if children.IsValid() && children.Kind() == reflect.Slice {
		for idx := 0; idx < children.Len(); idx++ {
			chains = append(chains, collectFieldChains(children.Index(idx))...)
		}
	}
	return dedupeFieldChains(chains)
}

func fieldNodeName(node reflect.Value) (string, bool) {
	value := node.FieldByName("value")
	if !value.IsValid() || value.Kind() != reflect.Interface || value.IsNil() {
		return "", false
	}
	elem := value.Elem()
	if !elem.IsValid() || elem.Kind() != reflect.String {
		return "", false
	}
	name := strings.TrimSpace(elem.String())
	if name == "" {
		return "", false
	}
	return name, true
}

func combineFieldChains(left [][]string, right [][]string) [][]string {
	if len(left) == 0 || len(right) == 0 {
		return nil
	}

	combined := make([][]string, 0, len(left)*len(right))
	for _, leftChain := range left {
		for _, rightChain := range right {
			chain := make([]string, 0, len(leftChain)+len(rightChain))
			chain = append(chain, leftChain...)
			chain = append(chain, rightChain...)
			combined = append(combined, chain)
		}
	}
	return combined
}

func dedupeFieldChains(chains [][]string) [][]string {
	if len(chains) == 0 {
		return nil
	}
	result := make([][]string, 0, len(chains))
	seen := map[string]struct{}{}
	for _, chain := range chains {
		if len(chain) == 0 {
			continue
		}
		key := strings.Join(chain, "\x00")
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, chain)
	}
	return result
}

func buildWhereItemSchema(index model.QuerySchemaIndex, activeTypeSet []string) jmespath.JSONSchema {
	if len(activeTypeSet) == 1 {
		return buildWhereItemShape(index, activeTypeSet, activeTypeSet[0])
	}

	alternatives := make([]any, 0, len(activeTypeSet))
	for _, typeName := range activeTypeSet {
		alternatives = append(alternatives, buildWhereItemShape(index, activeTypeSet, typeName))
	}
	return jmespath.JSONSchema{
		"oneOf": alternatives,
	}
}

func buildWhereItemShape(index model.QuerySchemaIndex, activeTypeSet []string, typeName string) jmespath.JSONSchema {
	entityType := index.EntityTypes[typeName]
	allEntityTypes := support.SortedMapKeys(index.EntityTypes)

	metaProperties := map[string]any{}
	metaRequired := make([]any, 0, len(entityType.MetaFields))
	for _, fieldName := range support.SortedMapKeys(entityType.MetaFields) {
		field := entityType.MetaFields[fieldName]
		metaProperties[fieldName] = buildMetaFieldSchema(field)
		if field.Required {
			metaRequired = append(metaRequired, fieldName)
		}
	}

	refsProperties := map[string]any{}
	refsRequired := make([]any, 0, len(entityType.RefFields))
	for _, refFieldName := range support.SortedMapKeys(entityType.RefFields) {
		refField := entityType.RefFields[refFieldName]
		refTypeEnum := refField.RefTypes
		if len(refTypeEnum) == 0 {
			refTypeEnum = allEntityTypes
		}
		refObject := buildRefObjectSchema(refTypeEnum)
		if refField.Cardinality == model.RefCardinalityArray {
			refsProperties[refFieldName] = map[string]any{
				"type":  "array",
				"items": refObject,
			}
			continue
		}
		refsProperties[refFieldName] = refObject
	}

	sectionProperties := map[string]any{}
	sectionRequired := make([]any, 0, len(entityType.SectionFields))
	for _, sectionName := range support.SortedMapKeys(entityType.SectionFields) {
		section := entityType.SectionFields[sectionName]
		if section.Required {
			sectionProperties[sectionName] = map[string]any{"type": "string"}
			sectionRequired = append(sectionRequired, sectionName)
		} else {
			sectionProperties[sectionName] = map[string]any{"type": "string"}
		}
	}

	return jmespath.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"type": map[string]any{
				"const": typeName,
			},
			"id":       map[string]any{"type": "string"},
			"slug":     map[string]any{"type": "string"},
			"revision": map[string]any{"type": "string"},
			"createdDate": map[string]any{
				"type":   "string",
				"format": "date",
			},
			"updatedDate": map[string]any{
				"type":   "string",
				"format": "date",
			},
			"meta": map[string]any{
				"type":                 "object",
				"properties":           metaProperties,
				"required":             metaRequired,
				"additionalProperties": false,
			},
			"refs": map[string]any{
				"type":                 "object",
				"properties":           refsProperties,
				"required":             refsRequired,
				"additionalProperties": false,
			},
			"content": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"sections": map[string]any{
						"type":                 "object",
						"properties":           sectionProperties,
						"required":             sectionRequired,
						"additionalProperties": false,
					},
				},
				"required":             []any{"sections"},
				"additionalProperties": false,
			},
		},
		"required":             []any{"type", "id", "slug", "revision", "createdDate", "updatedDate", "meta", "refs", "content"},
		"additionalProperties": false,
	}
}

func buildMetaFieldSchema(field model.MetadataFieldSpec) map[string]any {
	schema := map[string]any{}
	switch field.Kind {
	case model.FieldKindString:
		schema["type"] = "string"
	case model.FieldKindDate:
		schema["type"] = "string"
		schema["format"] = "date"
	case model.FieldKindNumber:
		schema["type"] = "number"
	case model.FieldKindBoolean:
		schema["type"] = "boolean"
	case model.FieldKindArray:
		schema["type"] = "array"
		if field.ItemKind != "" {
			schema["items"] = map[string]any{"type": schemaTypeName(field.ItemKind)}
		}
	case model.FieldKindObject:
		schema["type"] = "object"
		schema["additionalProperties"] = false
	case model.FieldKindNull:
		schema["type"] = "null"
	default:
		schema["type"] = "string"
	}

	if len(field.EnumValues) > 0 {
		schema["enum"] = append([]any(nil), field.EnumValues...)
	}
	if field.HasConst {
		schema["const"] = field.ConstValue
	}
	return schema
}

func buildRefObjectSchema(typeEnum []string) map[string]any {
	typeEnumValues := make([]any, 0, len(typeEnum))
	for _, typeName := range typeEnum {
		typeEnumValues = append(typeEnumValues, typeName)
	}

	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"resolved": map[string]any{"type": "boolean"},
			"id":       map[string]any{"type": "string"},
			"type":     map[string]any{"type": "string", "enum": typeEnumValues},
			"slug":     map[string]any{"type": "string"},
			"reason":   map[string]any{"type": "string", "enum": unresolvedReasons},
		},
		"additionalProperties": false,
	}
}

func schemaTypeName(kind model.SchemaFieldKind) string {
	switch kind {
	case model.FieldKindString, model.FieldKindDate:
		return "string"
	case model.FieldKindNumber:
		return "number"
	case model.FieldKindBoolean:
		return "boolean"
	case model.FieldKindArray:
		return "array"
	case model.FieldKindObject:
		return "object"
	case model.FieldKindNull:
		return "null"
	default:
		return "string"
	}
}
