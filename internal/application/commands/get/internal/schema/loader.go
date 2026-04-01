package schema

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/get/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/get/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	"gopkg.in/yaml.v3"
)

const getSchemaBlockingStandardRef = "7"

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

func LoadReadModel(path string, displayPath string) (model.ReadModel, *domainerrors.AppError) {
	raw, err := os.ReadFile(path)
	if err != nil {
		reason := err.Error()
		if strings.TrimSpace(displayPath) != "" {
			reason = strings.Replace(reason, path, displayPath, 1)
		}
		return model.ReadModel{}, newSchemaError(
			domainerrors.CodeSchemaNotFound,
			"schema file is not readable",
			map[string]any{"reason": reason},
		)
	}

	if len(strings.TrimSpace(string(raw))) == 0 {
		return model.ReadModel{}, newSchemaError(
			domainerrors.CodeSchemaParseError,
			"schema file is empty",
			nil,
		)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return model.ReadModel{}, newSchemaError(
			domainerrors.CodeSchemaParseError,
			"failed to parse schema yaml/json",
			map[string]any{"reason": err.Error()},
		)
	}

	doc := support.FirstContentNode(&root)
	if doc == nil || doc.Kind != yaml.MappingNode {
		return model.ReadModel{}, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			"schema root must be a mapping object",
			nil,
		)
	}

	if duplicateKey, ok := support.FindDuplicateMappingKey(doc); ok {
		return model.ReadModel{}, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			"schema contains duplicate keys",
			map[string]any{"key": duplicateKey},
		)
	}

	decoded := map[string]any{}
	if err := doc.Decode(&decoded); err != nil {
		return model.ReadModel{}, newSchemaError(
			domainerrors.CodeSchemaParseError,
			"failed to decode schema mapping",
			map[string]any{"reason": err.Error()},
		)
	}

	if err := validateTopLevelKeys(decoded); err != nil {
		return model.ReadModel{}, err
	}

	rawEntity, ok := support.ToStringMap(decoded["entity"])
	if !ok || len(rawEntity) == 0 {
		return model.ReadModel{}, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			"schema.entity must be a non-empty mapping",
			nil,
		)
	}

	readModel := model.ReadModel{
		EntityTypes:      make(map[string]model.EntityTypeSpec, len(rawEntity)),
		AllowedSelectors: map[string]struct{}{},
	}
	for _, selector := range builtinSelectors {
		readModel.AllowedSelectors[selector] = struct{}{}
	}

	typeSet := map[string]struct{}{}
	for _, typeName := range support.SortedMapKeys(rawEntity) {
		typeSet[typeName] = struct{}{}
	}

	for _, entityTypeName := range support.SortedMapKeys(rawEntity) {
		rawType, ok := support.ToStringMap(rawEntity[entityTypeName])
		if !ok {
			return model.ReadModel{}, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s must be a mapping", entityTypeName),
				nil,
			)
		}

		typeSpec, parseErr := parseEntityType(entityTypeName, rawType, typeSet)
		if parseErr != nil {
			return model.ReadModel{}, parseErr
		}

		readModel.EntityTypes[entityTypeName] = typeSpec
		for field := range typeSpec.MetaFields {
			readModel.AllowedSelectors["meta."+field] = struct{}{}
		}
		for field, refSpec := range typeSpec.RefFields {
			readModel.AllowedSelectors["refs."+field] = struct{}{}
			if refSpec.Cardinality == model.RefCardinalityArray {
				continue
			}
			for _, leaf := range []string{"id", "resolved", "type", "slug", "reason"} {
				readModel.AllowedSelectors["refs."+field+"."+leaf] = struct{}{}
			}
		}
		for section := range typeSpec.SectionFields {
			readModel.AllowedSelectors["content.sections."+section] = struct{}{}
		}
	}

	if len(readModel.EntityTypes) == 0 {
		return model.ReadModel{}, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			"schema.entity must be non-empty",
			nil,
		)
	}

	return readModel, nil
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

func parseEntityType(name string, rawType map[string]any, typeSet map[string]struct{}) (model.EntityTypeSpec, *domainerrors.AppError) {
	typeSpec := model.EntityTypeSpec{
		Name:          name,
		MetaFields:    map[string]struct{}{},
		RefFields:     map[string]model.RefFieldSpec{},
		SectionFields: map[string]struct{}{},
	}

	rawMeta, hasMeta := rawType["meta"]
	if hasMeta {
		metaNode, ok := support.ToStringMap(rawMeta)
		if !ok {
			return model.EntityTypeSpec{}, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta must be a mapping", name),
				nil,
			)
		}

		rawFields, hasFields := metaNode["fields"]
		if hasFields {
			fieldsNode, ok := support.ToStringMap(rawFields)
			if !ok {
				return model.EntityTypeSpec{}, newSchemaError(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("schema.entity.%s.meta.fields must be a mapping", name),
					nil,
				)
			}

			for _, metadataFieldName := range support.SortedMapKeys(fieldsNode) {
				rawField, ok := support.ToStringMap(fieldsNode[metadataFieldName])
				if !ok {
					return model.EntityTypeSpec{}, newSchemaError(
						domainerrors.CodeSchemaInvalid,
						fmt.Sprintf("schema.entity.%s.meta.fields.%s must be a mapping", name, metadataFieldName),
						nil,
					)
				}

				parsedField, parseErr := parseMetadataField(name, metadataFieldName, rawField, typeSet)
				if parseErr != nil {
					return model.EntityTypeSpec{}, parseErr
				}

				if parsedField.IsRef {
					cardinality := model.RefCardinalityScalar
					if parsedField.IsArrayRef {
						cardinality = model.RefCardinalityArray
					}
					typeSpec.RefFields[metadataFieldName] = model.RefFieldSpec{
						Name:        metadataFieldName,
						Cardinality: cardinality,
						RefTypes:    append([]string(nil), parsedField.RefTypes...),
					}
				} else {
					typeSpec.MetaFields[metadataFieldName] = struct{}{}
				}
			}
		}
	}

	rawContent, hasContent := rawType["content"]
	if hasContent {
		contentNode, ok := support.ToStringMap(rawContent)
		if !ok {
			return model.EntityTypeSpec{}, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.content must be a mapping", name),
				nil,
			)
		}

		rawSections, hasSections := contentNode["sections"]
		if hasSections {
			sectionsNode, ok := support.ToStringMap(rawSections)
			if !ok {
				return model.EntityTypeSpec{}, newSchemaError(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("schema.entity.%s.content.sections must be a mapping", name),
					nil,
				)
			}
			for _, sectionName := range support.SortedMapKeys(sectionsNode) {
				typeSpec.SectionFields[sectionName] = struct{}{}
			}
		}
	}

	return typeSpec, nil
}

type parsedMetadataField struct {
	IsRef      bool
	IsArrayRef bool
	RefTypes   []string
}

func parseMetadataField(entityTypeName string, fieldName string, rawField map[string]any, typeSet map[string]struct{}) (parsedMetadataField, *domainerrors.AppError) {
	rawSchema, ok := rawField["schema"]
	if !ok {
		return parsedMetadataField{}, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema is required", entityTypeName, fieldName),
			nil,
		)
	}

	schemaNode, ok := support.ToStringMap(rawSchema)
	if !ok {
		return parsedMetadataField{}, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema must be a mapping", entityTypeName, fieldName),
			nil,
		)
	}

	rawType, ok := schemaNode["type"].(string)
	if !ok || strings.TrimSpace(rawType) == "" {
		return parsedMetadataField{}, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.type must be a non-empty string", entityTypeName, fieldName),
			nil,
		)
	}

	normalizedType := strings.TrimSpace(rawType)
	switch normalizedType {
	case "integer", "number", "boolean", "array", "string", "entityRef":
		parsed := parsedMetadataField{}
		if normalizedType == "entityRef" {
			parsed.IsRef = true
			refTypes, refErr := extractRefTypes(
				fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.refTypes", entityTypeName, fieldName),
				schemaNode,
				typeSet,
			)
			if refErr != nil {
				return parsedMetadataField{}, refErr
			}
			parsed.RefTypes = refTypes
			return parsed, nil
		}
		if normalizedType == "array" {
			isArrayRef, refTypes, parseErr := parseArrayEntityRefMetadataField(entityTypeName, fieldName, schemaNode, typeSet)
			if parseErr != nil {
				return parsedMetadataField{}, parseErr
			}
			if isArrayRef {
				parsed.IsRef = true
				parsed.IsArrayRef = true
				parsed.RefTypes = refTypes
			}
		}
		return parsed, nil
	default:
		return parsedMetadataField{}, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.type uses unsupported type", entityTypeName, fieldName),
			map[string]any{"type": normalizedType},
		)
	}
}

func parseArrayEntityRefMetadataField(
	entityTypeName string,
	fieldName string,
	schemaNode map[string]any,
	typeSet map[string]struct{},
) (bool, []string, *domainerrors.AppError) {
	rawItems, hasItems := schemaNode["items"]
	if !hasItems {
		return false, nil, nil
	}
	itemsNode, ok := support.ToStringMap(rawItems)
	if !ok {
		return false, nil, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.items must be a mapping", entityTypeName, fieldName),
			nil,
		)
	}

	rawItemType, ok := itemsNode["type"].(string)
	if !ok || strings.TrimSpace(rawItemType) == "" {
		return false, nil, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.items.type must be a non-empty string", entityTypeName, fieldName),
			nil,
		)
	}
	if strings.TrimSpace(rawItemType) != "entityRef" {
		return false, nil, nil
	}

	refTypes, refErr := extractRefTypes(
		fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.items.refTypes", entityTypeName, fieldName),
		itemsNode,
		typeSet,
	)
	if refErr != nil {
		return false, nil, refErr
	}
	return true, refTypes, nil
}

func extractRefTypes(path string, schemaNode map[string]any, typeSet map[string]struct{}) ([]string, *domainerrors.AppError) {
	rawRefTypes, ok := schemaNode["refTypes"]
	if !ok {
		return nil, nil
	}
	values, ok := rawRefTypes.([]any)
	if !ok || len(values) == 0 {
		return nil, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("%s must be a non-empty array", path),
			nil,
		)
	}

	refTypes := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for idx, raw := range values {
		refType, ok := raw.(string)
		if !ok || strings.TrimSpace(refType) == "" {
			return nil, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s[%d] must be non-empty string", path, idx),
				nil,
			)
		}
		refType = strings.TrimSpace(refType)
		if _, duplicate := seen[refType]; duplicate {
			return nil, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s contains duplicate '%s'", path, refType),
				nil,
			)
		}
		if _, exists := typeSet[refType]; !exists {
			return nil, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s references unknown entity type '%s'", path, refType),
				nil,
			)
		}
		seen[refType] = struct{}{}
		refTypes = append(refTypes, refType)
	}

	sort.Strings(refTypes)
	return refTypes, nil
}

func newSchemaError(code domainerrors.Code, message string, details map[string]any) *domainerrors.AppError {
	issue := support.ValidationIssue(
		"error",
		"SchemaError",
		message,
		getSchemaBlockingStandardRef,
	)
	return domainerrors.New(code, message, support.WithValidationIssues(details, issue))
}
