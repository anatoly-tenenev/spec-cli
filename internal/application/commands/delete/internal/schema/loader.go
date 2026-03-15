package schema

import (
	"fmt"
	"os"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/delete/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/delete/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	"gopkg.in/yaml.v3"
)

func Load(path string, sourcePath string) (model.Schema, *domainerrors.AppError) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return model.Schema{}, domainerrors.New(
			domainerrors.CodeSchemaNotFound,
			"schema file is not readable",
			map[string]any{"reason": schemaReadErrorReason(err, path, sourcePath)},
		)
	}

	if len(strings.TrimSpace(string(raw))) == 0 {
		return model.Schema{}, domainerrors.New(
			domainerrors.CodeSchemaParseError,
			"schema file is empty",
			nil,
		)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return model.Schema{}, domainerrors.New(
			domainerrors.CodeSchemaParseError,
			"failed to parse schema yaml/json",
			map[string]any{"reason": err.Error()},
		)
	}

	doc := support.FirstContentNode(&root)
	if doc == nil || doc.Kind != yaml.MappingNode {
		return model.Schema{}, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			"schema root must be a mapping object",
			nil,
		)
	}

	if duplicateKey, ok := support.FindDuplicateMappingKey(doc); ok {
		return model.Schema{}, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			"schema contains duplicate keys",
			map[string]any{"key": duplicateKey},
		)
	}

	decoded := map[string]any{}
	if err := doc.Decode(&decoded); err != nil {
		return model.Schema{}, domainerrors.New(
			domainerrors.CodeSchemaParseError,
			"failed to decode schema mapping",
			map[string]any{"reason": err.Error()},
		)
	}

	if topErr := validateTopLevelKeys(decoded); topErr != nil {
		return model.Schema{}, topErr
	}

	entityRaw, ok := support.ToStringMap(decoded["entity"])
	if !ok || len(entityRaw) == 0 {
		return model.Schema{}, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			"schema.entity must be a non-empty mapping",
			nil,
		)
	}

	referenceSlotsByType := make(map[string][]model.ReferenceSlot, len(entityRaw))
	for _, typeName := range support.SortedMapKeys(entityRaw) {
		typeConfig, ok := support.ToStringMap(entityRaw[typeName])
		if !ok {
			return model.Schema{}, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s must be a mapping", typeName),
				nil,
			)
		}

		slots, parseErr := parseReferenceSlots(typeName, typeConfig)
		if parseErr != nil {
			return model.Schema{}, parseErr
		}
		referenceSlotsByType[typeName] = slots
	}

	return model.Schema{ReferenceSlotsByType: referenceSlotsByType}, nil
}

func schemaReadErrorReason(err error, absolutePath string, sourcePath string) string {
	reason := err.Error()
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return reason
	}
	return strings.Replace(reason, absolutePath, sourcePath, 1)
}

func validateTopLevelKeys(values map[string]any) *domainerrors.AppError {
	for key := range values {
		switch key {
		case "version", "entity", "description":
			continue
		default:
			return domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema has unsupported top-level key '%s'", key),
				nil,
			)
		}
	}
	return nil
}

func parseReferenceSlots(typeName string, typeConfig map[string]any) ([]model.ReferenceSlot, *domainerrors.AppError) {
	rawMeta, hasMeta := typeConfig["meta"]
	if !hasMeta {
		return []model.ReferenceSlot{}, nil
	}

	metaMap, ok := support.ToStringMap(rawMeta)
	if !ok {
		return nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta must be a mapping", typeName),
			nil,
		)
	}

	rawFields, hasFields := metaMap["fields"]
	if !hasFields {
		return []model.ReferenceSlot{}, nil
	}

	fieldsMap, ok := support.ToStringMap(rawFields)
	if !ok {
		return nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields must be a mapping", typeName),
			nil,
		)
	}

	slots := make([]model.ReferenceSlot, 0)
	for _, fieldName := range support.SortedMapKeys(fieldsMap) {
		rawField, ok := support.ToStringMap(fieldsMap[fieldName])
		if !ok {
			return nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.fields.%s must be a mapping", typeName, fieldName),
				nil,
			)
		}

		rawSchema, hasSchema := rawField["schema"]
		if !hasSchema {
			return nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema is required", typeName, fieldName),
				nil,
			)
		}

		schemaMap, ok := support.ToStringMap(rawSchema)
		if !ok {
			return nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema must be a mapping", typeName, fieldName),
				nil,
			)
		}

		typeValue, ok := schemaMap["type"].(string)
		if !ok || strings.TrimSpace(typeValue) == "" {
			return nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.type must be a non-empty string", typeName, fieldName),
				nil,
			)
		}
		typeValue = strings.TrimSpace(typeValue)

		switch typeValue {
		case "entity_ref":
			slots = append(slots, model.ReferenceSlot{FieldName: fieldName, Kind: model.ReferenceSlotScalar})
		case "array":
			itemsRaw, hasItems := schemaMap["items"]
			if !hasItems {
				return nil, domainerrors.New(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.items is required for array type", typeName, fieldName),
					nil,
				)
			}
			itemsMap, ok := support.ToStringMap(itemsRaw)
			if !ok {
				return nil, domainerrors.New(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.items must be a mapping", typeName, fieldName),
					nil,
				)
			}
			itemType, ok := itemsMap["type"].(string)
			if !ok || strings.TrimSpace(itemType) == "" {
				return nil, domainerrors.New(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.items.type must be a non-empty string", typeName, fieldName),
					nil,
				)
			}
			if strings.TrimSpace(itemType) == "entity_ref" {
				slots = append(slots, model.ReferenceSlot{FieldName: fieldName, Kind: model.ReferenceSlotArray})
			}
		}
	}

	return slots, nil
}
