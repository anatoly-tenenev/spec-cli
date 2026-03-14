package schema

import (
	"fmt"
	"os"
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
	"created_date",
	"updated_date",
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

	for _, entityTypeName := range support.SortedMapKeys(rawEntity) {
		rawType, ok := support.ToStringMap(rawEntity[entityTypeName])
		if !ok {
			return model.ReadModel{}, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s must be a mapping", entityTypeName),
				nil,
			)
		}

		typeSpec, parseErr := parseEntityType(entityTypeName, rawType)
		if parseErr != nil {
			return model.ReadModel{}, parseErr
		}

		readModel.EntityTypes[entityTypeName] = typeSpec
		for field := range typeSpec.MetaFields {
			readModel.AllowedSelectors["meta."+field] = struct{}{}
		}
		for field := range typeSpec.RefFields {
			readModel.AllowedSelectors["refs."+field] = struct{}{}
			for _, refPart := range []string{"type", "id", "slug"} {
				readModel.AllowedSelectors[fmt.Sprintf("refs.%s.%s", field, refPart)] = struct{}{}
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

func parseEntityType(name string, rawType map[string]any) (model.EntityTypeSpec, *domainerrors.AppError) {
	typeSpec := model.EntityTypeSpec{
		Name:          name,
		MetaFields:    map[string]struct{}{},
		RefFields:     map[string]struct{}{},
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

				isRef, parseErr := parseMetadataField(name, metadataFieldName, rawField)
				if parseErr != nil {
					return model.EntityTypeSpec{}, parseErr
				}

				typeSpec.MetaFields[metadataFieldName] = struct{}{}
				if isRef {
					typeSpec.RefFields[metadataFieldName] = struct{}{}
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

func parseMetadataField(entityTypeName string, fieldName string, rawField map[string]any) (bool, *domainerrors.AppError) {
	rawSchema, ok := rawField["schema"]
	if !ok {
		return false, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema is required", entityTypeName, fieldName),
			nil,
		)
	}

	schemaNode, ok := support.ToStringMap(rawSchema)
	if !ok {
		return false, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema must be a mapping", entityTypeName, fieldName),
			nil,
		)
	}

	rawType, ok := schemaNode["type"].(string)
	if !ok || strings.TrimSpace(rawType) == "" {
		return false, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.type must be a non-empty string", entityTypeName, fieldName),
			nil,
		)
	}

	normalizedType := strings.TrimSpace(rawType)
	switch normalizedType {
	case "integer", "number", "boolean", "array", "string", "entity_ref":
		return normalizedType == "entity_ref", nil
	default:
		return false, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.type uses unsupported type", entityTypeName, fieldName),
			map[string]any{"type": normalizedType},
		)
	}
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
