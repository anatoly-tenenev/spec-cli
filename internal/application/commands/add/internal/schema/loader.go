package schema

import (
	"fmt"
	"os"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/model"
	schemaentity "github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/schema/internal/entity"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	"gopkg.in/yaml.v3"
)

const schemaBlockingStandardRef = "7"

func Load(path string, sourcePath string) (model.AddSchema, *domainerrors.AppError) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return model.AddSchema{}, newSchemaError(
			domainerrors.CodeSchemaNotFound,
			"schema file is not readable",
			map[string]any{"reason": schemaReadErrorReason(err, path, sourcePath)},
		)
	}

	if len(strings.TrimSpace(string(raw))) == 0 {
		return model.AddSchema{}, newSchemaError(
			domainerrors.CodeSchemaParseError,
			"schema file is empty",
			nil,
		)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return model.AddSchema{}, newSchemaError(
			domainerrors.CodeSchemaParseError,
			"failed to parse schema yaml/json",
			map[string]any{"reason": err.Error()},
		)
	}

	doc := support.FirstContentNode(&root)
	if doc == nil || doc.Kind != yaml.MappingNode {
		return model.AddSchema{}, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			"schema root must be a mapping object",
			nil,
		)
	}

	if duplicateKey, ok := support.FindDuplicateMappingKey(doc); ok {
		return model.AddSchema{}, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			"schema contains duplicate keys",
			map[string]any{"key": duplicateKey},
		)
	}

	decoded := map[string]any{}
	if err := doc.Decode(&decoded); err != nil {
		return model.AddSchema{}, newSchemaError(
			domainerrors.CodeSchemaParseError,
			"failed to decode schema mapping",
			map[string]any{"reason": err.Error()},
		)
	}

	if topErr := validateTopLevelKeys(decoded); topErr != nil {
		return model.AddSchema{}, topErr
	}

	entityRaw, ok := support.ToStringMap(decoded["entity"])
	if !ok || len(entityRaw) == 0 {
		return model.AddSchema{}, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			"schema.entity must be a non-empty mapping",
			nil,
		)
	}

	entityNode := mappingValueNode(doc, "entity")
	if entityNode == nil || entityNode.Kind != yaml.MappingNode {
		return model.AddSchema{}, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			"schema.entity must be a mapping",
			nil,
		)
	}

	spec := model.AddSchema{EntityTypes: make(map[string]model.EntityTypeSpec, len(entityRaw))}
	usedPrefixes := map[string]string{}

	for _, typeName := range support.SortedMapKeys(entityRaw) {
		typeConfig, ok := support.ToStringMap(entityRaw[typeName])
		if !ok {
			return model.AddSchema{}, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s must be a mapping", typeName),
				nil,
			)
		}

		typeNode := mappingValueNode(entityNode, typeName)
		parsed, parseErr := schemaentity.ParseType(typeName, typeConfig, typeNode, usedPrefixes)
		if parseErr != nil {
			return model.AddSchema{}, parseErr
		}
		spec.EntityTypes[typeName] = parsed
	}

	return spec, nil
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
			return newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema has unsupported top-level key '%s'", key),
				nil,
			)
		}
	}
	return nil
}

func mappingValueNode(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for idx := 0; idx+1 < len(node.Content); idx += 2 {
		if node.Content[idx].Value == key {
			return node.Content[idx+1]
		}
	}
	return nil
}

func newSchemaError(code domainerrors.Code, message string, details map[string]any) *domainerrors.AppError {
	issue := support.ValidationIssue("schema.invalid", message, schemaBlockingStandardRef, "")
	return domainerrors.New(code, message, support.WithValidationIssues(details, issue))
}
