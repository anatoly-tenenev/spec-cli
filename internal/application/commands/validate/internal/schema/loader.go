package schema

import (
	"fmt"
	"os"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	schemaentity "github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/schema/internal/entity"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
	"gopkg.in/yaml.v3"
)

func Load(path string) (model.ValidationSchema, []domainvalidation.Issue, *domainerrors.AppError) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return model.ValidationSchema{}, nil, domainerrors.New(
			domainerrors.CodeSchemaNotFound,
			"schema file is not readable",
			map[string]any{"reason": err.Error()},
		)
	}

	if len(strings.TrimSpace(string(raw))) == 0 {
		return model.ValidationSchema{}, nil, domainerrors.New(
			domainerrors.CodeSchemaParseError,
			"schema file is empty",
			nil,
		)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return model.ValidationSchema{}, nil, domainerrors.New(
			domainerrors.CodeSchemaParseError,
			"failed to parse schema yaml/json",
			map[string]any{"reason": err.Error()},
		)
	}

	doc := support.FirstContentNode(&root)
	if doc == nil || doc.Kind != yaml.MappingNode {
		return model.ValidationSchema{}, nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			"schema root must be a mapping object",
			nil,
		)
	}

	if duplicateKey, ok := support.FindDuplicateMappingKey(doc); ok {
		return model.ValidationSchema{}, nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			"schema contains duplicate keys",
			map[string]any{"key": duplicateKey},
		)
	}

	decoded := map[string]any{}
	if err := doc.Decode(&decoded); err != nil {
		return model.ValidationSchema{}, nil, domainerrors.New(
			domainerrors.CodeSchemaParseError,
			"failed to decode schema mapping",
			map[string]any{"reason": err.Error()},
		)
	}
	if keyErr := validateTopLevelKeys(decoded); keyErr != nil {
		return model.ValidationSchema{}, nil, keyErr
	}

	entityRaw, ok := support.ToStringMap(decoded["entity"])
	if !ok || len(entityRaw) == 0 {
		return model.ValidationSchema{}, nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			"schema.entity must be a non-empty mapping",
			nil,
		)
	}

	typeNames := support.SortedMapKeys(entityRaw)
	typeSet := make(map[string]struct{}, len(typeNames))
	for _, typeName := range typeNames {
		typeSet[typeName] = struct{}{}
	}

	loaded := model.ValidationSchema{Entity: make(map[string]model.SchemaEntityType, len(entityRaw))}
	issues := make([]domainvalidation.Issue, 0)
	usedPrefixes := map[string]string{}

	for _, typeName := range typeNames {
		typeConfig, ok := support.ToStringMap(entityRaw[typeName])
		if !ok {
			return model.ValidationSchema{}, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s must be a mapping", typeName),
				nil,
			)
		}

		entityType, typeIssues, typeErr := schemaentity.ParseType(typeName, typeConfig, typeSet, usedPrefixes)
		if typeErr != nil {
			return model.ValidationSchema{}, nil, typeErr
		}

		loaded.Entity[typeName] = entityType
		issues = append(issues, typeIssues...)
	}
	if schemaIssue, hasSchemaIssue := firstSchemaError(issues); hasSchemaIssue {
		return model.ValidationSchema{}, nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			schemaIssue.Message,
			map[string]any{
				"code":         schemaIssue.Code,
				"field":        schemaIssue.Field,
				"standard_ref": schemaIssue.StandardRef,
			},
		)
	}

	return loaded, issues, nil
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

func firstSchemaError(issues []domainvalidation.Issue) (domainvalidation.Issue, bool) {
	for _, issue := range issues {
		if issue.Class != "SchemaError" || issue.Level != domainvalidation.LevelError {
			continue
		}
		return issue, true
	}
	return domainvalidation.Issue{}, false
}
