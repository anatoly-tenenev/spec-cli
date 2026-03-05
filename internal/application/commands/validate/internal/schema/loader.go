package schema

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
	"gopkg.in/yaml.v3"
)

var idPrefixPattern = regexp.MustCompile(`^[A-Za-z0-9_]+(?:-[A-Za-z0-9_]+)*$`)

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

	entityRaw, ok := support.ToStringMap(decoded["entity"])
	if !ok || len(entityRaw) == 0 {
		return model.ValidationSchema{}, nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			"schema.entity must be a non-empty mapping",
			nil,
		)
	}

	loaded := model.ValidationSchema{Entity: make(map[string]model.SchemaEntityType, len(entityRaw))}
	warnings := make([]domainvalidation.Issue, 0)

	typeNames := support.SortedMapKeys(entityRaw)
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

		idPrefix, ok := typeConfig["id_prefix"].(string)
		if !ok || strings.TrimSpace(idPrefix) == "" {
			return model.ValidationSchema{}, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.id_prefix must be a non-empty string", typeName),
				nil,
			)
		}
		if !idPrefixPattern.MatchString(idPrefix) {
			return model.ValidationSchema{}, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.id_prefix has invalid format", typeName),
				nil,
			)
		}
		if existingType, exists := usedPrefixes[idPrefix]; exists {
			return model.ValidationSchema{}, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				"schema contains duplicated id_prefix across entity types",
				map[string]any{"id_prefix": idPrefix, "types": []string{existingType, typeName}},
			)
		}
		usedPrefixes[idPrefix] = typeName

		requiredFields, fieldWarnings, requiredFieldErr := parseRequiredFields(typeName, typeConfig["meta"])
		if requiredFieldErr != nil {
			return model.ValidationSchema{}, nil, requiredFieldErr
		}
		warnings = append(warnings, fieldWarnings...)

		requiredSections, requiredSectionsErr := parseRequiredSections(typeName, typeConfig["content"])
		if requiredSectionsErr != nil {
			return model.ValidationSchema{}, nil, requiredSectionsErr
		}

		loaded.Entity[typeName] = model.SchemaEntityType{
			Name:             typeName,
			IDPrefix:         idPrefix,
			RequiredFields:   requiredFields,
			RequiredSections: requiredSections,
		}
	}

	return loaded, warnings, nil
}

func parseRequiredFields(typeName string, rawMeta any) ([]model.RequiredFieldRule, []domainvalidation.Issue, *domainerrors.AppError) {
	if rawMeta == nil {
		return nil, nil, nil
	}

	metaMap, ok := support.ToStringMap(rawMeta)
	if !ok {
		return nil, nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta must be a mapping", typeName),
			nil,
		)
	}

	requiredRaw, exists := metaMap["required_fields"]
	if !exists || requiredRaw == nil {
		return nil, nil, nil
	}

	requiredItems, ok := support.ToSlice(requiredRaw)
	if !ok {
		return nil, nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.required_fields must be a list", typeName),
			nil,
		)
	}

	warnings := make([]domainvalidation.Issue, 0)
	rules := make([]model.RequiredFieldRule, 0, len(requiredItems))
	usedNames := map[string]struct{}{}

	for idx, item := range requiredItems {
		rule := model.RequiredFieldRule{}

		switch typed := item.(type) {
		case string:
			rule.Name = strings.TrimSpace(typed)
			rule.Type = "any"
		case map[string]any:
			name, ok := typed["name"].(string)
			if !ok || strings.TrimSpace(name) == "" {
				return nil, nil, domainerrors.New(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("schema.entity.%s.meta.required_fields[%d].name must be non-empty string", typeName, idx),
					nil,
				)
			}
			rule.Name = strings.TrimSpace(name)

			if fieldType, hasType := typed["type"]; hasType {
				fieldTypeName, ok := fieldType.(string)
				if !ok || strings.TrimSpace(fieldTypeName) == "" {
					return nil, nil, domainerrors.New(
						domainerrors.CodeSchemaInvalid,
						fmt.Sprintf("schema.entity.%s.meta.required_fields[%d].type must be non-empty string", typeName, idx),
						nil,
					)
				}
				rule.Type = strings.TrimSpace(fieldTypeName)
			} else {
				rule.Type = "any"
			}

			if enumRaw, hasEnum := typed["enum"]; hasEnum {
				enumValues, ok := support.ToSlice(enumRaw)
				if !ok {
					return nil, nil, domainerrors.New(
						domainerrors.CodeSchemaInvalid,
						fmt.Sprintf("schema.entity.%s.meta.required_fields[%d].enum must be a list", typeName, idx),
						nil,
					)
				}
				rule.Enum = append(rule.Enum, enumValues...)
			}

			if value, hasValue := typed["value"]; hasValue {
				rule.HasValue = true
				rule.Value = value
				if support.IsExpressionValue(value) {
					rule.ValueIsExpression = true
					warnings = append(warnings, domainvalidation.Issue{
						Code:        "profile.expression_not_supported",
						Level:       domainvalidation.LevelWarning,
						Class:       "ProfileError",
						Message:     fmt.Sprintf("expressions are not supported for required field '%s'", rule.Name),
						StandardRef: "11.6",
						Field:       fmt.Sprintf("schema.entity.%s.meta.required_fields.%s.value", typeName, rule.Name),
					})
				}
			}
		default:
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.required_fields[%d] must be string or object", typeName, idx),
				nil,
			)
		}

		if rule.Name == "" {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.required_fields[%d] contains empty field name", typeName, idx),
				nil,
			)
		}
		if !support.IsSupportedRuleType(rule.Type) {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.required_fields[%d].type uses unsupported type", typeName, idx),
				map[string]any{"type": rule.Type},
			)
		}
		if _, exists := usedNames[rule.Name]; exists {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.required_fields has duplicate field '%s'", typeName, rule.Name),
				nil,
			)
		}
		usedNames[rule.Name] = struct{}{}

		if rule.Type == "entity_ref" {
			warnings = append(warnings, domainvalidation.Issue{
				Code:        "profile.entity_ref_not_supported",
				Level:       domainvalidation.LevelWarning,
				Class:       "ProfileError",
				Message:     fmt.Sprintf("entity_ref is not supported for required field '%s'", rule.Name),
				StandardRef: "6.3",
				Field:       fmt.Sprintf("schema.entity.%s.meta.required_fields.%s", typeName, rule.Name),
			})
		}

		rules = append(rules, rule)
	}

	return rules, warnings, nil
}

func parseRequiredSections(typeName string, rawContent any) ([]string, *domainerrors.AppError) {
	if rawContent == nil {
		return nil, nil
	}

	contentMap, ok := support.ToStringMap(rawContent)
	if !ok {
		return nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.content must be a mapping", typeName),
			nil,
		)
	}

	requiredRaw, exists := contentMap["required_sections"]
	if !exists || requiredRaw == nil {
		return nil, nil
	}

	sectionItems, ok := support.ToSlice(requiredRaw)
	if !ok {
		return nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.content.required_sections must be a list", typeName),
			nil,
		)
	}

	sections := make([]string, 0, len(sectionItems))
	seen := map[string]struct{}{}

	for idx, item := range sectionItems {
		name, ok := item.(string)
		if !ok {
			return nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.content.required_sections[%d] must be string", typeName, idx),
				nil,
			)
		}
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.content.required_sections[%d] cannot be empty", typeName, idx),
				nil,
			)
		}
		if _, exists := seen[name]; exists {
			return nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.content.required_sections contains duplicate '%s'", typeName, name),
				nil,
			)
		}
		seen[name] = struct{}{}
		sections = append(sections, name)
	}

	return sections, nil
}
