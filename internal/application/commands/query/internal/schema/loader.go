package schema

import (
	"fmt"
	"os"
	"sort"
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

	typeNames := support.SortedMapKeys(entityNode)
	typeSet := make(map[string]struct{}, len(typeNames))
	for _, typeName := range typeNames {
		typeSet[typeName] = struct{}{}
	}

	entityTypes := make(map[string]EntityType, len(entityNode))
	for _, entityTypeName := range typeNames {
		rawType, ok := support.ToStringMap(entityNode[entityTypeName])
		if !ok {
			return LoadedSchema{}, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s must be a mapping", entityTypeName),
				nil,
			)
		}

		parsedType, parseErr := parseEntityType(entityTypeName, rawType, typeSet)
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

func parseEntityType(name string, rawType map[string]any, typeSet map[string]struct{}) (EntityType, *domainerrors.AppError) {
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

				field, parseErr := parseMetadataField(name, metadataFieldName, rawField, typeSet)
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

	contentSections := map[string]SectionField{}
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
				rawSection, ok := support.ToStringMap(sectionsNode[sectionName])
				if !ok {
					return EntityType{}, newSchemaError(
						domainerrors.CodeSchemaInvalid,
						fmt.Sprintf("schema.entity.%s.content.sections.%s must be a mapping", name, sectionName),
						nil,
					)
				}
				required, requiredErr := parseRequiredFlag(rawSection, fmt.Sprintf("schema.entity.%s.content.sections.%s.required", name, sectionName))
				if requiredErr != nil {
					return EntityType{}, requiredErr
				}
				contentSections[sectionName] = SectionField{
					Name:     sectionName,
					Required: required,
				}
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

func parseMetadataField(
	entityTypeName string,
	fieldName string,
	rawField map[string]any,
	typeSet map[string]struct{},
) (Field, *domainerrors.AppError) {
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

	kind, itemKind, kindErr := schemaTypeToFieldKind(entityTypeName, fieldName, rawType, schemaNode)
	if kindErr != nil {
		return Field{}, kindErr
	}

	required, requiredErr := parseRequiredFlag(rawField, fmt.Sprintf("schema.entity.%s.meta.fields.%s.required", entityTypeName, fieldName))
	if requiredErr != nil {
		return Field{}, requiredErr
	}

	isEntityRef, isArrayRef, refTypes, refErr := parseEntityRefMetadataField(entityTypeName, fieldName, rawType, schemaNode, typeSet)
	if refErr != nil {
		return Field{}, refErr
	}

	field := Field{
		Name:        fieldName,
		Kind:        kind,
		ItemKind:    itemKind,
		EnumValues:  []any{},
		Required:    required,
		IsEntityRef: isEntityRef,
		IsArrayRef:  isArrayRef,
		RefTypes:    append([]string(nil), refTypes...),
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

	if rawConst, hasConst := schemaNode["const"]; hasConst {
		field.HasConst = true
		field.ConstValue = rawConst
	}

	return field, nil
}

func parseRequiredFlag(rawRule map[string]any, path string) (bool, *domainerrors.AppError) {
	required := true
	rawRequired, exists := rawRule["required"]
	if !exists {
		return required, nil
	}
	switch typed := rawRequired.(type) {
	case bool:
		return typed, nil
	case string:
		if err := support.ValidateSingleInterpolation(typed); err != nil {
			return false, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s has invalid expression in required context: %s", path, err.Error()),
				nil,
			)
		}
		return false, nil
	default:
		return false, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("%s must be boolean or string interpolation ${expr}", path),
			nil,
		)
	}
}

func parseEntityRefMetadataField(
	entityTypeName string,
	fieldName string,
	rawType string,
	schemaNode map[string]any,
	typeSet map[string]struct{},
) (bool, bool, []string, *domainerrors.AppError) {
	normalizedType := strings.TrimSpace(rawType)
	if normalizedType == "entityRef" {
		refTypes, err := extractRefTypes(
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.refTypes", entityTypeName, fieldName),
			schemaNode,
			typeSet,
		)
		if err != nil {
			return false, false, nil, err
		}
		return true, false, refTypes, nil
	}
	if normalizedType != "array" {
		return false, false, nil, nil
	}

	rawItems, hasItems := schemaNode["items"]
	if !hasItems {
		return false, false, nil, nil
	}
	itemsNode, ok := support.ToStringMap(rawItems)
	if !ok {
		return false, false, nil, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.items must be a mapping", entityTypeName, fieldName),
			nil,
		)
	}

	rawItemType, ok := itemsNode["type"].(string)
	if !ok || strings.TrimSpace(rawItemType) == "" {
		return false, false, nil, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.items.type must be a non-empty string", entityTypeName, fieldName),
			nil,
		)
	}
	if strings.TrimSpace(rawItemType) != "entityRef" {
		return false, false, nil, nil
	}

	refTypes, err := extractRefTypes(
		fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.items.refTypes", entityTypeName, fieldName),
		itemsNode,
		typeSet,
	)
	if err != nil {
		return false, false, nil, err
	}
	return true, true, refTypes, nil
}

func extractRefTypes(path string, schemaNode map[string]any, typeSet map[string]struct{}) ([]string, *domainerrors.AppError) {
	rawRefTypes, ok := schemaNode["refTypes"]
	if !ok {
		return nil, nil
	}

	values, ok := support.ToSlice(rawRefTypes)
	if !ok || len(values) == 0 {
		return nil, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("%s must be a non-empty array", path),
			nil,
		)
	}

	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for idx, item := range values {
		refType, ok := item.(string)
		if !ok || strings.TrimSpace(refType) == "" {
			return nil, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s[%d] must be non-empty string", path, idx),
				nil,
			)
		}
		refType = strings.TrimSpace(refType)
		if _, exists := typeSet[refType]; !exists {
			return nil, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s references unknown entity type '%s'", path, refType),
				nil,
			)
		}
		if _, duplicate := seen[refType]; duplicate {
			return nil, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s contains duplicate '%s'", path, refType),
				nil,
			)
		}
		seen[refType] = struct{}{}
		result = append(result, refType)
	}
	sort.Strings(result)
	return result, nil
}

func schemaTypeToFieldKind(
	entityTypeName string,
	fieldName string,
	rawType string,
	rawFieldSchema map[string]any,
) (model.SchemaFieldKind, model.SchemaFieldKind, *domainerrors.AppError) {
	normalizedType := strings.TrimSpace(rawType)

	switch normalizedType {
	case "integer", "number":
		return model.FieldKindNumber, "", nil
	case "boolean":
		return model.FieldKindBoolean, "", nil
	case "array":
		itemKind, itemErr := parseArrayItemKind(entityTypeName, fieldName, rawFieldSchema)
		if itemErr != nil {
			return "", "", itemErr
		}
		return model.FieldKindArray, itemKind, nil
	case "string":
		if format, ok := rawFieldSchema["format"].(string); ok && strings.TrimSpace(format) == "date" {
			return model.FieldKindDate, "", nil
		}
		return model.FieldKindString, "", nil
	case "null":
		return model.FieldKindNull, "", nil
	case "entityRef":
		// entityRef is exposed under refs namespace as an object.
		return model.FieldKindString, "", nil
	default:
		return "", "", newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.type uses unsupported type", entityTypeName, fieldName),
			map[string]any{"type": normalizedType},
		)
	}
}

func parseArrayItemKind(
	entityTypeName string,
	fieldName string,
	rawFieldSchema map[string]any,
) (model.SchemaFieldKind, *domainerrors.AppError) {
	rawItems, hasItems := rawFieldSchema["items"]
	if !hasItems {
		return "", nil
	}
	itemsNode, ok := support.ToStringMap(rawItems)
	if !ok {
		return "", newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.items must be a mapping", entityTypeName, fieldName),
			nil,
		)
	}
	rawItemType, ok := itemsNode["type"].(string)
	if !ok || strings.TrimSpace(rawItemType) == "" {
		return "", newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.items.type must be a non-empty string", entityTypeName, fieldName),
			nil,
		)
	}

	switch strings.TrimSpace(rawItemType) {
	case "string", "entityRef":
		return model.FieldKindString, nil
	case "integer", "number":
		return model.FieldKindNumber, nil
	case "boolean":
		return model.FieldKindBoolean, nil
	case "null":
		return model.FieldKindNull, nil
	case "object":
		return model.FieldKindObject, nil
	case "array":
		return model.FieldKindArray, nil
	default:
		return "", newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.items.type uses unsupported type", entityTypeName, fieldName),
			map[string]any{"type": strings.TrimSpace(rawItemType)},
		)
	}
}

func newSchemaError(code domainerrors.Code, message string, details map[string]any) *domainerrors.AppError {
	issue := support.ValidationIssue(
		"error",
		"SchemaError",
		message,
		querySchemaBlockingStandardRef,
	)
	return domainerrors.New(code, message, support.WithValidationIssues(details, issue))
}
