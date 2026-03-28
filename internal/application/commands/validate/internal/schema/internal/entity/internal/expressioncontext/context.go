package expressioncontext

import (
	"strings"

	jmespath "github.com/anatoly-tenenev/go-jmespath"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/domain/reservedkeys"
)

type MetaFieldConstraints struct {
	HasConst bool
	Const    any
	Enum     []any
}

func IsBuiltinMetaField(fieldName string) bool {
	switch fieldName {
	case reservedkeys.BuiltinType,
		reservedkeys.BuiltinID,
		reservedkeys.BuiltinSlug,
		reservedkeys.BuiltinCreatedDate,
		reservedkeys.BuiltinUpdatedDate:
		return true
	default:
		return false
	}
}

func BuildEntityExpressionSchema(
	requiredFields []model.RequiredFieldRule,
	constraintsByField map[string]MetaFieldConstraints,
) jmespath.JSONSchema {
	topLevelProps := map[string]any{
		reservedkeys.BuiltinType:        map[string]any{"type": "string"},
		reservedkeys.BuiltinID:          map[string]any{"type": "string"},
		reservedkeys.BuiltinSlug:        map[string]any{"type": "string"},
		reservedkeys.BuiltinCreatedDate: map[string]any{"type": "string"},
		reservedkeys.BuiltinUpdatedDate: map[string]any{"type": "string"},
	}

	metaProps := map[string]any{}
	metaRequired := make([]any, 0, 8)
	refsProps := map[string]any{}
	for _, rule := range requiredFields {
		if IsBuiltinMetaField(rule.Name) {
			continue
		}

		if rule.Type == reservedkeys.SchemaTypeEntityRef {
			refsProps[rule.Name] = map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":      map[string]any{"type": "string"},
					"type":    map[string]any{"type": "string"},
					"slug":    map[string]any{"type": "string"},
					"dirPath": map[string]any{"type": "string"},
				},
				"required":             []any{"id", "type", "slug", "dirPath"},
				"additionalProperties": false,
			}
			continue
		}

		fieldSchema := buildMetaFieldSchema(rule, constraintsByField[rule.Name])
		if fieldSchema != nil {
			metaProps[rule.Name] = fieldSchema
		}

		if rule.Required && rule.RequiredExpr == nil {
			metaRequired = append(metaRequired, rule.Name)
		}
	}

	topLevelProps["meta"] = map[string]any{
		"type":                 "object",
		"properties":           metaProps,
		"required":             metaRequired,
		"additionalProperties": false,
	}
	topLevelProps["refs"] = map[string]any{
		"type":                 "object",
		"properties":           refsProps,
		"additionalProperties": false,
	}

	return jmespath.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			reservedkeys.BuiltinType:        topLevelProps[reservedkeys.BuiltinType],
			reservedkeys.BuiltinID:          topLevelProps[reservedkeys.BuiltinID],
			reservedkeys.BuiltinSlug:        topLevelProps[reservedkeys.BuiltinSlug],
			reservedkeys.BuiltinCreatedDate: topLevelProps[reservedkeys.BuiltinCreatedDate],
			reservedkeys.BuiltinUpdatedDate: topLevelProps[reservedkeys.BuiltinUpdatedDate],
			"meta":                          topLevelProps["meta"],
			"refs":                          topLevelProps["refs"],
		},
		"required": []any{
			reservedkeys.BuiltinType,
			reservedkeys.BuiltinID,
			reservedkeys.BuiltinSlug,
			reservedkeys.BuiltinCreatedDate,
			reservedkeys.BuiltinUpdatedDate,
			"meta",
			"refs",
		},
		"additionalProperties": false,
	}
}

func IsPathGuaranteedBySchema(path string, fieldsByName map[string]model.RequiredFieldRule) bool {
	segments := pathSegments(path)
	if len(segments) == 0 {
		return false
	}

	switch segments[0] {
	case reservedkeys.BuiltinType,
		reservedkeys.BuiltinID,
		reservedkeys.BuiltinSlug,
		reservedkeys.BuiltinCreatedDate,
		reservedkeys.BuiltinUpdatedDate:
		return true
	case "meta":
		if len(segments) == 1 {
			return true
		}
		fieldName := segments[1]
		rule, exists := fieldsByName[fieldName]
		if !exists {
			return false
		}
		if rule.Type == reservedkeys.SchemaTypeEntityRef {
			return false
		}
		return rule.Required && rule.RequiredExpr == nil
	case "refs":
		if len(segments) == 1 {
			return true
		}
		return false
	default:
		return false
	}
}

func GuardRootForPath(path string, fieldsByName map[string]model.RequiredFieldRule) (string, bool) {
	segments := pathSegments(path)
	if len(segments) < 2 {
		return "", false
	}

	switch segments[0] {
	case "refs":
		fieldName := segments[1]
		rule, exists := fieldsByName[fieldName]
		if !exists || rule.Type != reservedkeys.SchemaTypeEntityRef {
			return "", false
		}
		return "refs." + fieldName, true
	case "meta":
		fieldName := segments[1]
		if IsBuiltinMetaField(fieldName) {
			return "", false
		}
		rule, exists := fieldsByName[fieldName]
		if !exists {
			return "", false
		}
		if rule.Type == reservedkeys.SchemaTypeEntityRef {
			return "", false
		}
		if rule.Required && rule.RequiredExpr == nil {
			return "", false
		}
		return "meta." + fieldName, true
	default:
		return "", false
	}
}

func buildMetaFieldSchema(rule model.RequiredFieldRule, constraints MetaFieldConstraints) map[string]any {
	jsonType, ok := mapRuleTypeToJSONSchemaType(rule.Type)
	if !ok {
		return nil
	}

	schema := map[string]any{"type": jsonType}
	if rule.Type == "array" && rule.HasItemType {
		if itemType, itemOK := mapRuleTypeToJSONSchemaType(rule.ItemType); itemOK {
			schema["items"] = map[string]any{"type": itemType}
		}
	}

	if isScalarType(jsonType) {
		if constraints.HasConst {
			schema["const"] = constraints.Const
		}
		if len(constraints.Enum) > 0 {
			schema["enum"] = constraints.Enum
		}
	}

	return schema
}

func mapRuleTypeToJSONSchemaType(ruleType string) (string, bool) {
	switch ruleType {
	case "string", reservedkeys.SchemaTypeEntityRef:
		return "string", true
	case "integer", "number":
		return "number", true
	case "boolean":
		return "boolean", true
	case "null":
		return "null", true
	case "array":
		return "array", true
	default:
		return "", false
	}
}

func isScalarType(jsonType string) bool {
	switch jsonType {
	case "string", "number", "boolean", "null":
		return true
	default:
		return false
	}
}

func pathSegments(path string) []string {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	parts := strings.Split(path, ".")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		segments = append(segments, part)
	}
	return segments
}
