package metafields

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	expressioncontext "github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/schema/internal/entity/internal/expressioncontext"
	names "github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/schema/internal/entity/internal/names"
	requiredconstraint "github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/schema/internal/entity/internal/requiredconstraint"
	schemachecks "github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/schema/internal/entity/internal/schemachecks"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

func Parse(
	typeName string,
	rawMeta any,
	typeSet map[string]struct{},
) ([]model.RequiredFieldRule, []domainvalidation.Issue, *domainerrors.AppError) {
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
	if keyErr := schemachecks.EnsureOnlyKeys(fmt.Sprintf("schema.entity.%s.meta", typeName), metaMap, "fields"); keyErr != nil {
		return nil, nil, keyErr
	}

	rawFields, exists := metaMap["fields"]
	if !exists || rawFields == nil {
		return nil, nil, nil
	}

	rawByName, ok := support.ToStringMap(rawFields)
	if !ok {
		return nil, nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields must be a mapping", typeName),
			nil,
		)
	}

	fieldNames := support.SortedMapKeys(rawByName)
	rules := make([]model.RequiredFieldRule, 0, len(fieldNames))
	rawRules := make([]map[string]any, 0, len(fieldNames))
	issues := make([]domainvalidation.Issue, 0)

	for _, fieldName := range fieldNames {
		if keyErr := names.ValidateMetaFieldName(typeName, fieldName); keyErr != nil {
			return nil, nil, keyErr
		}

		fieldPath := fmt.Sprintf("schema.entity.%s.meta.fields.%s", typeName, fieldName)
		rawRule, ok := support.ToStringMap(rawByName[fieldName])
		if !ok {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s must be an object", fieldPath),
				nil,
			)
		}
		if keyErr := schemachecks.EnsureOnlyKeys(fieldPath, rawRule, "required", "required_when", "description", "schema"); keyErr != nil {
			return nil, nil, keyErr
		}

		schemaRaw, ok := support.ToStringMap(rawRule["schema"])
		if !ok {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s.schema must be an object", fieldPath),
				nil,
			)
		}
		if keyErr := schemachecks.EnsureOnlyKeys(fieldPath+".schema", schemaRaw, "type", "const", "enum", "refTypes", "items", "uniqueItems", "minItems", "maxItems"); keyErr != nil {
			return nil, nil, keyErr
		}

		ruleTypeRaw, hasType := schemaRaw["type"]
		ruleType, ok := ruleTypeRaw.(string)
		if !hasType || !ok || strings.TrimSpace(ruleType) == "" {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s.schema.type must be non-empty string", fieldPath),
				nil,
			)
		}
		ruleType = strings.TrimSpace(ruleType)
		if !support.IsSupportedRuleType(ruleType) {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s.schema.type uses unsupported type", fieldPath),
				map[string]any{"type": ruleType},
			)
		}

		rule := model.RequiredFieldRule{
			Name:             fieldName,
			Type:             ruleType,
			RequiredWhenPath: fieldPath + ".required_when",
		}

		if enumRaw, exists := schemaRaw["enum"]; exists {
			enumValues, ok := support.ToSlice(enumRaw)
			if !ok || len(enumValues) == 0 {
				return nil, nil, domainerrors.New(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("%s.schema.enum must be a non-empty list", fieldPath),
					nil,
				)
			}
			for enumIndex, enumValue := range enumValues {
				if !support.MatchesRuleType(enumValue, ruleType) {
					return nil, nil, domainerrors.New(
						domainerrors.CodeSchemaInvalid,
						fmt.Sprintf("%s.schema.enum[%d] does not match declared type", fieldPath, enumIndex),
						nil,
					)
				}
			}
			rule.Enum = append(rule.Enum, enumValues...)
		}

		if value, exists := schemaRaw["const"]; exists {
			if !support.MatchesRuleType(value, ruleType) {
				return nil, nil, domainerrors.New(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("%s.schema.const does not match declared type", fieldPath),
					nil,
				)
			}
			rule.HasValue = true
			rule.Value = value
		}

		if ruleType == "array" {
			if arrayErr := parseArrayConstraints(fieldPath, schemaRaw, &rule); arrayErr != nil {
				return nil, nil, arrayErr
			}
		} else if hasArrayConstraint(schemaRaw) {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s.schema array constraints are allowed only for type array", fieldPath),
				nil,
			)
		}

		if ruleType == "entity_ref" {
			refTypesRaw, hasRefTypes := schemaRaw["refTypes"]
			if hasRefTypes {
				refTypes, refTypesErr := parseRefTypes(typeName, fieldName, refTypesRaw, typeSet)
				if refTypesErr != nil {
					return nil, nil, refTypesErr
				}
				rule.RefTypes = refTypes
			}
		} else if _, hasRefTypes := schemaRaw["refTypes"]; hasRefTypes {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s.schema.refTypes is allowed only for type entity_ref", fieldPath),
				nil,
			)
		}

		rules = append(rules, rule)
		rawRules = append(rawRules, rawRule)
	}

	compileContext := expressioncontext.Build(rules)
	for idx := range rules {
		fieldPath := fmt.Sprintf("schema.entity.%s.meta.fields.%s", typeName, rules[idx].Name)
		required, requiredWhenLiteral, requiredWhenExpr, requiredIssues, requiredErr := requiredconstraint.Parse(
			rawRules[idx],
			fieldPath,
			compileContext,
		)
		if requiredErr != nil {
			return nil, nil, requiredErr
		}
		rules[idx].Required = required
		rules[idx].RequiredWhen = requiredWhenLiteral
		rules[idx].RequiredWhenExpr = requiredWhenExpr
		issues = append(issues, requiredIssues...)
	}
	fieldsByName := make(map[string]model.RequiredFieldRule, len(rules))
	for _, rule := range rules {
		fieldsByName[rule.Name] = rule
	}
	for _, rule := range rules {
		if usage, hasUsage := schemachecks.StrictMissingUsageInRequiredWhen(rule.RequiredWhenExpr, fieldsByName); hasUsage {
			message := fmt.Sprintf(
				"schema.entity.%s.meta.fields.%s.required_when uses strict operator '%s' with potentially missing operand '%s'",
				typeName,
				rule.Name,
				usage.Operator,
				usage.Operand.Raw,
			)
			issues = append(issues, domainvalidation.Issue{
				Code:        "schema.required_when.strict_potentially_missing",
				Level:       domainvalidation.LevelError,
				Class:       "SchemaError",
				Message:     message,
				StandardRef: "11.6",
				Field:       rule.RequiredWhenPath,
			})
		}
	}

	return rules, issues, nil
}

func hasArrayConstraint(values map[string]any) bool {
	if _, exists := values["items"]; exists {
		return true
	}
	if _, exists := values["uniqueItems"]; exists {
		return true
	}
	if _, exists := values["minItems"]; exists {
		return true
	}
	if _, exists := values["maxItems"]; exists {
		return true
	}
	return false
}

func parseArrayConstraints(fieldPath string, schemaRaw map[string]any, rule *model.RequiredFieldRule) *domainerrors.AppError {
	if rawItems, hasItems := schemaRaw["items"]; hasItems {
		itemsMap, ok := support.ToStringMap(rawItems)
		if !ok {
			return domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s.schema.items must be an object", fieldPath),
				nil,
			)
		}
		if keyErr := schemachecks.EnsureOnlyKeys(fieldPath+".schema.items", itemsMap, "type"); keyErr != nil {
			return keyErr
		}

		rawItemType, exists := itemsMap["type"]
		itemType, ok := rawItemType.(string)
		if !exists || !ok || strings.TrimSpace(itemType) == "" {
			return domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s.schema.items.type must be non-empty string", fieldPath),
				nil,
			)
		}
		itemType = strings.TrimSpace(itemType)
		if !support.IsSupportedRuleType(itemType) || itemType == "array" {
			return domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s.schema.items.type uses unsupported type", fieldPath),
				map[string]any{"type": itemType},
			)
		}
		rule.HasItemType = true
		rule.ItemType = itemType
	}

	if rawUnique, hasUnique := schemaRaw["uniqueItems"]; hasUnique {
		uniqueItems, ok := rawUnique.(bool)
		if !ok {
			return domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s.schema.uniqueItems must be boolean", fieldPath),
				nil,
			)
		}
		rule.UniqueItems = uniqueItems
	}

	if rawMinItems, hasMinItems := schemaRaw["minItems"]; hasMinItems {
		minItems, ok := parseArrayLengthConstraint(rawMinItems)
		if !ok {
			return domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s.schema.minItems must be non-negative integer", fieldPath),
				nil,
			)
		}
		rule.HasMinItems = true
		rule.MinItems = minItems
	}

	if rawMaxItems, hasMaxItems := schemaRaw["maxItems"]; hasMaxItems {
		maxItems, ok := parseArrayLengthConstraint(rawMaxItems)
		if !ok {
			return domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s.schema.maxItems must be non-negative integer", fieldPath),
				nil,
			)
		}
		rule.HasMaxItems = true
		rule.MaxItems = maxItems
	}

	if rule.HasMinItems && rule.HasMaxItems && rule.MinItems > rule.MaxItems {
		return domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("%s.schema.minItems cannot be greater than maxItems", fieldPath),
			nil,
		)
	}

	return nil
}

func parseArrayLengthConstraint(raw any) (int, bool) {
	switch typed := raw.(type) {
	case int:
		if typed < 0 {
			return 0, false
		}
		return typed, true
	case int8:
		if typed < 0 {
			return 0, false
		}
		return int(typed), true
	case int16:
		if typed < 0 {
			return 0, false
		}
		return int(typed), true
	case int32:
		if typed < 0 {
			return 0, false
		}
		return int(typed), true
	case int64:
		if typed < 0 {
			return 0, false
		}
		return int(typed), true
	case uint:
		return int(typed), true
	case uint8:
		return int(typed), true
	case uint16:
		return int(typed), true
	case uint32:
		return int(typed), true
	case uint64:
		if typed > uint64(^uint(0)>>1) {
			return 0, false
		}
		return int(typed), true
	case float64:
		if typed < 0 || typed != float64(int(typed)) {
			return 0, false
		}
		return int(typed), true
	case string:
		if strings.TrimSpace(typed) == "" {
			return 0, false
		}
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil || parsed < 0 {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func parseRefTypes(typeName string, fieldName string, raw any, typeSet map[string]struct{}) ([]string, *domainerrors.AppError) {
	rawItems, ok := raw.([]any)
	if !ok || len(rawItems) == 0 {
		return nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.refTypes must be a non-empty list", typeName, fieldName),
			nil,
		)
	}

	refTypes := make([]string, 0, len(rawItems))
	seen := map[string]struct{}{}
	for idx, item := range rawItems {
		value, ok := item.(string)
		if !ok || strings.TrimSpace(value) == "" {
			return nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.refTypes[%d] must be non-empty string", typeName, fieldName, idx),
				nil,
			)
		}
		value = strings.TrimSpace(value)
		if _, exists := seen[value]; exists {
			return nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.refTypes contains duplicate '%s'", typeName, fieldName, value),
				nil,
			)
		}
		if _, exists := typeSet[value]; !exists {
			return nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.refTypes references unknown entity type '%s'", typeName, fieldName, value),
				nil,
			)
		}
		seen[value] = struct{}{}
		refTypes = append(refTypes, value)
	}

	sort.Strings(refTypes)
	return refTypes, nil
}
