package expressioncontext

import (
	"strings"

	jmespath "github.com/anatoly-tenenev/go-jmespath"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
	"github.com/anatoly-tenenev/spec-cli/internal/domain/reservedkeys"
)

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

func BuildEntityExpressionSchema(entity model.EntityType) jmespath.JSONSchema {
	metaProps := map[string]any{}
	metaRequired := make([]any, 0, len(entity.MetaFields))
	refsProps := map[string]any{}

	for fieldName, field := range entity.MetaFields {
		if IsBuiltinMetaField(fieldName) {
			continue
		}

		if field.Value.Kind == model.ValueKindEntityRef {
			refsProps[fieldName] = map[string]any{
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

		if schema := buildMetaFieldSchema(field.Value); schema != nil {
			metaProps[fieldName] = schema
		}

		if field.Required.Always && field.Required.Expr == nil {
			metaRequired = append(metaRequired, fieldName)
		}
	}

	return jmespath.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			reservedkeys.BuiltinType:        map[string]any{"type": "string"},
			reservedkeys.BuiltinID:          map[string]any{"type": "string"},
			reservedkeys.BuiltinSlug:        map[string]any{"type": "string"},
			reservedkeys.BuiltinCreatedDate: map[string]any{"type": "string"},
			reservedkeys.BuiltinUpdatedDate: map[string]any{"type": "string"},
			"meta": map[string]any{
				"type":                 "object",
				"properties":           metaProps,
				"required":             metaRequired,
				"additionalProperties": false,
			},
			"refs": map[string]any{
				"type":                 "object",
				"properties":           refsProps,
				"additionalProperties": false,
			},
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

func IsPathGuaranteedBySchema(path string, fieldsByName map[string]model.MetaField) bool {
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
		field, exists := fieldsByName[fieldName]
		if !exists || field.Value.Kind == model.ValueKindEntityRef {
			return false
		}
		return field.Required.Always && field.Required.Expr == nil
	case "refs":
		if len(segments) == 1 {
			return true
		}
		return false
	default:
		return false
	}
}

func GuardRootForPath(path string, fieldsByName map[string]model.MetaField) (string, bool) {
	segments := pathSegments(path)
	if len(segments) < 2 {
		return "", false
	}

	switch segments[0] {
	case "refs":
		fieldName := segments[1]
		field, exists := fieldsByName[fieldName]
		if !exists || field.Value.Kind != model.ValueKindEntityRef {
			return "", false
		}
		return "refs." + fieldName, true
	case "meta":
		fieldName := segments[1]
		if IsBuiltinMetaField(fieldName) {
			return "", false
		}
		field, exists := fieldsByName[fieldName]
		if !exists || field.Value.Kind == model.ValueKindEntityRef {
			return "", false
		}
		if field.Required.Always && field.Required.Expr == nil {
			return "", false
		}
		return "meta." + fieldName, true
	default:
		return "", false
	}
}

func buildMetaFieldSchema(value model.ValueSpec) map[string]any {
	jsonType, ok := mapValueKindToJSONSchemaType(value.Kind)
	if !ok {
		return nil
	}

	schema := map[string]any{"type": jsonType}
	if value.Kind == model.ValueKindArray && value.Items != nil {
		if itemType, itemOK := mapValueKindToJSONSchemaType(value.Items.Kind); itemOK {
			schema["items"] = map[string]any{"type": itemType}
		}
	}

	if isScalarType(jsonType) {
		if value.Const != nil && value.Const.Template == nil {
			schema["const"] = value.Const.Value
		}
		if len(value.Enum) > 0 {
			enumValues := make([]any, 0, len(value.Enum))
			for _, enumValue := range value.Enum {
				if enumValue.Template != nil {
					enumValues = nil
					break
				}
				enumValues = append(enumValues, enumValue.Value)
			}
			if len(enumValues) > 0 {
				schema["enum"] = enumValues
			}
		}
	}

	return schema
}

func mapValueKindToJSONSchemaType(kind model.ValueKind) (string, bool) {
	switch kind {
	case model.ValueKindString, model.ValueKindEntityRef:
		return "string", true
	case model.ValueKindInteger, model.ValueKindNumber:
		return "number", true
	case model.ValueKindBoolean:
		return "boolean", true
	case model.ValueKindArray:
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
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		segments = append(segments, trimmed)
	}
	return segments
}
