package schema

import (
	"fmt"
	"os"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	"gopkg.in/yaml.v3"
)

const querySchemaBlockingStandardRef = "7"

func Load(path string) (LoadedSchema, *domainerrors.AppError) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return LoadedSchema{}, newSchemaError(
			domainerrors.CodeSchemaNotFound,
			"schema file is not readable",
			map[string]any{"reason": err.Error()},
		)
	}

	if len(strings.TrimSpace(string(raw))) == 0 {
		return LoadedSchema{}, newSchemaError(
			domainerrors.CodeSchemaParseError,
			"schema file is empty",
			nil,
		)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return LoadedSchema{}, newSchemaError(
			domainerrors.CodeSchemaParseError,
			"failed to parse schema yaml/json",
			map[string]any{"reason": err.Error()},
		)
	}

	doc := support.FirstContentNode(&root)
	if doc == nil || doc.Kind != yaml.MappingNode {
		return LoadedSchema{}, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			"schema root must be a mapping object",
			nil,
		)
	}

	if duplicateKey, ok := support.FindDuplicateMappingKey(doc); ok {
		return LoadedSchema{}, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			"schema contains duplicate keys",
			map[string]any{"key": duplicateKey},
		)
	}

	decoded := map[string]any{}
	if err := doc.Decode(&decoded); err != nil {
		return LoadedSchema{}, newSchemaError(
			domainerrors.CodeSchemaParseError,
			"failed to decode schema mapping",
			map[string]any{"reason": err.Error()},
		)
	}

	if topLevelErr := validateTopLevelKeys(decoded); topLevelErr != nil {
		return LoadedSchema{}, topLevelErr
	}

	entityNode, ok := support.ToStringMap(decoded["entity"])
	if !ok || len(entityNode) == 0 {
		return LoadedSchema{}, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			"schema.entity must be a non-empty mapping",
			nil,
		)
	}

	entityTypes := make(map[string]EntityType, len(entityNode))
	for _, entityTypeName := range support.SortedMapKeys(entityNode) {
		rawType, ok := support.ToStringMap(entityNode[entityTypeName])
		if !ok {
			return LoadedSchema{}, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s must be a mapping", entityTypeName),
				nil,
			)
		}

		parsedType, parseErr := parseEntityType(entityTypeName, rawType)
		if parseErr != nil {
			return LoadedSchema{}, parseErr
		}
		entityTypes[entityTypeName] = parsedType
	}

	return LoadedSchema{
		RawText:     string(raw),
		EntityTypes: entityTypes,
	}, nil
}

func validateTopLevelKeys(values map[string]any) *domainerrors.AppError {
	for key := range values {
		switch key {
		case "version", "entity", "description":
			continue
		default:
			return newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema has unsupported top-level key '%s'", key),
				nil,
			)
		}
	}
	return nil
}

func parseEntityType(name string, rawType map[string]any) (EntityType, *domainerrors.AppError) {
	metadataFields := map[string]Field{}
	entityRefFields := map[string]Field{}

	rawMeta, hasMeta := rawType["meta"]
	if hasMeta {
		metaNode, ok := support.ToStringMap(rawMeta)
		if !ok {
			return EntityType{}, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta must be a mapping", name),
				nil,
			)
		}

		rawFields, hasFields := metaNode["fields"]
		if hasFields {
			fieldsNode, ok := support.ToStringMap(rawFields)
			if !ok {
				return EntityType{}, newSchemaError(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("schema.entity.%s.meta.fields must be a mapping", name),
					nil,
				)
			}

			for _, metadataFieldName := range support.SortedMapKeys(fieldsNode) {
				rawField, ok := support.ToStringMap(fieldsNode[metadataFieldName])
				if !ok {
					return EntityType{}, newSchemaError(
						domainerrors.CodeSchemaInvalid,
						fmt.Sprintf("schema.entity.%s.meta.fields.%s must be a mapping", name, metadataFieldName),
						nil,
					)
				}

				field, parseErr := parseMetadataField(name, metadataFieldName, rawField)
				if parseErr != nil {
					return EntityType{}, parseErr
				}
				metadataFields[metadataFieldName] = field
				if field.IsEntityRef {
					entityRefFields[metadataFieldName] = field
				}
			}
		}
	}

	contentSections := map[string]struct{}{}
	if rawContent, hasContent := rawType["content"]; hasContent {
		contentNode, ok := support.ToStringMap(rawContent)
		if !ok {
			return EntityType{}, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.content must be a mapping", name),
				nil,
			)
		}

		rawSections, hasSections := contentNode["sections"]
		if hasSections {
			sectionsNode, ok := support.ToStringMap(rawSections)
			if !ok {
				return EntityType{}, newSchemaError(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("schema.entity.%s.content.sections must be a mapping", name),
					nil,
				)
			}
			for _, sectionName := range support.SortedMapKeys(sectionsNode) {
				contentSections[sectionName] = struct{}{}
			}
		}
	}

	return EntityType{
		Name:            name,
		MetadataFields:  metadataFields,
		EntityRefFields: entityRefFields,
		ContentSections: contentSections,
	}, nil
}

func parseMetadataField(entityTypeName string, fieldName string, rawField map[string]any) (Field, *domainerrors.AppError) {
	rawSchema, ok := rawField["schema"]
	if !ok {
		return Field{}, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema is required", entityTypeName, fieldName),
			nil,
		)
	}

	schemaNode, ok := support.ToStringMap(rawSchema)
	if !ok {
		return Field{}, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema must be a mapping", entityTypeName, fieldName),
			nil,
		)
	}

	rawType, ok := schemaNode["type"].(string)
	if !ok || strings.TrimSpace(rawType) == "" {
		return Field{}, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.type must be a non-empty string", entityTypeName, fieldName),
			nil,
		)
	}

	fieldKind, kindErr := schemaTypeToFieldKind(entityTypeName, fieldName, rawType, schemaNode)
	if kindErr != nil {
		return Field{}, kindErr
	}
	isEntityRef, refTypeHint, refErr := parseEntityRefMetadataField(entityTypeName, fieldName, rawType, schemaNode)
	if refErr != nil {
		return Field{}, refErr
	}

	field := Field{
		Name:        fieldName,
		Kind:        fieldKind,
		EnumValues:  []any{},
		IsEntityRef: isEntityRef,
		RefTypeHint: refTypeHint,
	}

	rawEnum, hasEnum := schemaNode["enum"]
	if hasEnum {
		enumValues, ok := support.ToSlice(rawEnum)
		if !ok || len(enumValues) == 0 {
			return Field{}, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.enum must be a non-empty array", entityTypeName, fieldName),
				nil,
			)
		}
		field.EnumValues = append(field.EnumValues, enumValues...)
	}

	return field, nil
}

func parseEntityRefMetadataField(
	entityTypeName string,
	fieldName string,
	rawType string,
	schemaNode map[string]any,
) (bool, string, *domainerrors.AppError) {
	normalizedType := strings.TrimSpace(rawType)
	if normalizedType == "entity_ref" {
		return true, extractSingleRefTypeHint(schemaNode), nil
	}
	if normalizedType != "array" {
		return false, "", nil
	}

	rawItems, hasItems := schemaNode["items"]
	if !hasItems {
		return false, "", nil
	}
	itemsNode, ok := support.ToStringMap(rawItems)
	if !ok {
		return false, "", newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.items must be a mapping", entityTypeName, fieldName),
			nil,
		)
	}

	rawItemType, ok := itemsNode["type"].(string)
	if !ok || strings.TrimSpace(rawItemType) == "" {
		return false, "", newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.items.type must be a non-empty string", entityTypeName, fieldName),
			nil,
		)
	}
	if strings.TrimSpace(rawItemType) != "entity_ref" {
		return false, "", nil
	}

	return true, extractSingleRefTypeHint(itemsNode), nil
}

func extractSingleRefTypeHint(schemaNode map[string]any) string {
	rawRefTypes, ok := schemaNode["refTypes"]
	if !ok {
		return ""
	}
	values, ok := support.ToSlice(rawRefTypes)
	if !ok || len(values) != 1 {
		return ""
	}
	refType, ok := values[0].(string)
	if !ok {
		return ""
	}
	refType = strings.TrimSpace(refType)
	if refType == "" {
		return ""
	}
	return refType
}

func schemaTypeToFieldKind(
	entityTypeName string,
	fieldName string,
	rawType string,
	rawFieldSchema map[string]any,
) (model.SchemaFieldKind, *domainerrors.AppError) {
	normalizedType := strings.TrimSpace(rawType)

	switch normalizedType {
	case "integer", "number":
		return model.FieldKindNumber, nil
	case "boolean":
		return model.FieldKindBoolean, nil
	case "array":
		return model.FieldKindArray, nil
	case "string":
		if format, ok := rawFieldSchema["format"].(string); ok && strings.TrimSpace(format) == "date" {
			return model.FieldKindDate, nil
		}
		return model.FieldKindString, nil
	case "entity_ref":
		return model.FieldKindString, nil
	default:
		return "", newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.type uses unsupported type", entityTypeName, fieldName),
			map[string]any{"type": normalizedType},
		)
	}
}

func newSchemaError(code domainerrors.Code, message string, details map[string]any) *domainerrors.AppError {
	issue := support.ValidationIssue(
		support.ValidationIssueLevelError,
		support.ValidationIssueClassSchemaError,
		message,
		querySchemaBlockingStandardRef,
	)
	return domainerrors.New(code, message, support.WithValidationIssues(details, issue))
}
