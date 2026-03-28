package metafields

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/expressions"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	expressioncontext "github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/schema/internal/entity/internal/expressioncontext"
	names "github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/schema/internal/entity/internal/names"
	requiredconstraint "github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/schema/internal/entity/internal/requiredconstraint"
	schemachecks "github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/schema/internal/entity/internal/schemachecks"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	"github.com/anatoly-tenenev/spec-cli/internal/domain/reservedkeys"
)

func Parse(
	typeName string,
	rawMeta any,
	typeSet map[string]struct{},
) ([]model.RequiredFieldRule, *expressions.Engine, *domainerrors.AppError) {
	metaMap := map[string]any{}
	if rawMeta != nil {
		parsedMetaMap, ok := support.ToStringMap(rawMeta)
		if !ok {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta must be a mapping", typeName),
				nil,
			)
		}
		metaMap = parsedMetaMap
	}
	if keyErr := schemachecks.EnsureOnlyKeys(fmt.Sprintf("schema.entity.%s.meta", typeName), metaMap, "fields"); keyErr != nil {
		return nil, nil, keyErr
	}

	rawByName := map[string]any{}
	if rawFields, exists := metaMap["fields"]; exists && rawFields != nil {
		parsedFields, ok := support.ToStringMap(rawFields)
		if !ok {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.fields must be a mapping", typeName),
				nil,
			)
		}
		rawByName = parsedFields
	}

	fieldNames := support.SortedMapKeys(rawByName)
	rules := make([]model.RequiredFieldRule, 0, len(fieldNames))
	rawRules := make([]map[string]any, 0, len(fieldNames))
	schemaRules := make([]map[string]any, 0, len(fieldNames))
	constraintsByField := make(map[string]expressioncontext.MetaFieldConstraints, len(fieldNames))

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
		if keyErr := schemachecks.EnsureOnlyKeys(fieldPath, rawRule, "required", "description", "schema"); keyErr != nil {
			return nil, nil, keyErr
		}

		if rawDescription, hasDescription := rawRule["description"]; hasDescription {
			description, ok := rawDescription.(string)
			if !ok || strings.TrimSpace(description) == "" {
				return nil, nil, domainerrors.New(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("%s.description must be non-empty string", fieldPath),
					nil,
				)
			}
			if expressions.ContainsInterpolation(description) {
				return nil, nil, domainerrors.New(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("%s.description does not allow interpolation ${...}", fieldPath),
					nil,
				)
			}
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

		rawType, hasType := schemaRaw["type"]
		ruleType, ok := rawType.(string)
		if !hasType || !ok || strings.TrimSpace(ruleType) == "" {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s.schema.type must be non-empty string", fieldPath),
				nil,
			)
		}
		if expressions.ContainsInterpolation(ruleType) {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s.schema.type does not allow interpolation ${...}", fieldPath),
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
			Name:         fieldName,
			Type:         ruleType,
			RequiredPath: fieldPath + ".required",
		}

		if ruleType == "array" {
			if arrayErr := parseArrayConstraints(fieldPath, schemaRaw, typeSet, &rule); arrayErr != nil {
				return nil, nil, arrayErr
			}
		} else if hasArrayConstraint(schemaRaw) {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s.schema array constraints are allowed only for type array", fieldPath),
				nil,
			)
		}

		if ruleType == reservedkeys.SchemaTypeEntityRef {
			if refTypesRaw, hasRefTypes := schemaRaw["refTypes"]; hasRefTypes {
				refTypes, refTypesErr := parseRefTypes(fieldPath+".schema.refTypes", refTypesRaw, typeSet)
				if refTypesErr != nil {
					return nil, nil, refTypesErr
				}
				rule.RefTypes = refTypes
			}
		} else if _, hasRefTypes := schemaRaw["refTypes"]; hasRefTypes {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s.schema.refTypes is allowed only for type entityRef", fieldPath),
				nil,
			)
		}

		constraintsByField[fieldName] = extractMetaFieldConstraints(ruleType, schemaRaw)
		rules = append(rules, rule)
		rawRules = append(rawRules, rawRule)
		schemaRules = append(schemaRules, schemaRaw)
	}

	expressionSchema := expressioncontext.BuildEntityExpressionSchema(rules, constraintsByField)
	expressionEngine, engineErr := expressions.NewSchemaAwareEngine(typeName, expressionSchema)
	if engineErr != nil {
		return nil, nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s expressions context schema is invalid: %s", typeName, engineErr.Message),
			map[string]any{
				"code":         engineErr.Code,
				"field":        fmt.Sprintf("schema.entity.%s", typeName),
				"offset":       engineErr.Offset,
				"standard_ref": "7",
			},
		)
	}

	for idx := range rules {
		fieldPath := fmt.Sprintf("schema.entity.%s.meta.fields.%s", typeName, rules[idx].Name)
		required, requiredExpr, requiredErr := requiredconstraint.Parse(rawRules[idx], fieldPath, expressionEngine)
		if requiredErr != nil {
			return nil, nil, requiredErr
		}
		rules[idx].Required = required
		rules[idx].RequiredExpr = requiredExpr

		if enumRaw, exists := schemaRules[idx]["enum"]; exists {
			enumValues, ok := support.ToSlice(enumRaw)
			if !ok || len(enumValues) == 0 {
				return nil, nil, domainerrors.New(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("%s.schema.enum must be a non-empty list", fieldPath),
					nil,
				)
			}
			parsedEnum := make([]model.RuleValue, 0, len(enumValues))
			for enumIndex, enumValue := range enumValues {
				parsedValue, parseErr := parseRuleValue(
					fmt.Sprintf("%s.schema.enum[%d]", fieldPath, enumIndex),
					enumValue,
					rules[idx].Type,
					expressionEngine,
				)
				if parseErr != nil {
					return nil, nil, parseErr
				}
				parsedEnum = append(parsedEnum, parsedValue)
			}
			rules[idx].Enum = parsedEnum
		}

		if value, exists := schemaRules[idx]["const"]; exists {
			parsedValue, parseErr := parseRuleValue(
				fieldPath+".schema.const",
				value,
				rules[idx].Type,
				expressionEngine,
			)
			if parseErr != nil {
				return nil, nil, parseErr
			}
			rules[idx].HasValue = true
			rules[idx].Value = parsedValue
		}
	}

	return rules, expressionEngine, nil
}

func parseRuleValue(
	path string,
	value any,
	ruleType string,
	engine *expressions.Engine,
) (model.RuleValue, *domainerrors.AppError) {
	stringValue, isString := value.(string)
	if isString && expressions.ContainsInterpolation(stringValue) {
		if ruleType != "string" {
			return model.RuleValue{}, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s interpolation ${...} is allowed only for string type", path),
				nil,
			)
		}

		template, compileErr := expressions.CompileTemplate(stringValue, engine)
		if compileErr != nil {
			return model.RuleValue{}, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s has invalid interpolation in schema.%s: %s", path, schemaValueContext(path), compileErr.Message),
				map[string]any{
					"code":         compileErr.Code,
					"field":        path,
					"offset":       compileErr.Offset,
					"standard_ref": "9.4",
				},
			)
		}

		return model.RuleValue{Literal: stringValue, Template: template}, nil
	}

	if !support.MatchesRuleType(value, ruleType) {
		return model.RuleValue{}, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("%s does not match declared type", path),
			nil,
		)
	}

	return model.RuleValue{Literal: value}, nil
}

func extractMetaFieldConstraints(ruleType string, schemaRaw map[string]any) expressioncontext.MetaFieldConstraints {
	constraints := expressioncontext.MetaFieldConstraints{}
	if !isRuleScalar(ruleType) {
		return constraints
	}

	if rawConst, exists := schemaRaw["const"]; exists {
		if isScalarLiteral(rawConst) && !isInterpolatedString(rawConst) {
			constraints.HasConst = true
			constraints.Const = rawConst
		}
	}

	if rawEnum, exists := schemaRaw["enum"]; exists {
		rawItems, ok := support.ToSlice(rawEnum)
		if !ok || len(rawItems) == 0 {
			return constraints
		}
		enumValues := make([]any, 0, len(rawItems))
		for _, item := range rawItems {
			if !isScalarLiteral(item) || isInterpolatedString(item) {
				return constraints
			}
			enumValues = append(enumValues, item)
		}
		constraints.Enum = enumValues
	}

	return constraints
}

func isRuleScalar(ruleType string) bool {
	switch ruleType {
	case "string", "number", "integer", "boolean", "null", reservedkeys.SchemaTypeEntityRef:
		return true
	default:
		return false
	}
}

func isScalarLiteral(value any) bool {
	switch value.(type) {
	case string, bool, nil, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	default:
		return false
	}
}

func isInterpolatedString(value any) bool {
	typed, ok := value.(string)
	return ok && expressions.ContainsInterpolation(typed)
}

func schemaValueContext(path string) string {
	if strings.Contains(path, ".schema.const") {
		return "const"
	}
	if strings.Contains(path, ".schema.enum") {
		return "enum"
	}
	return "value"
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

func parseArrayConstraints(
	fieldPath string,
	schemaRaw map[string]any,
	typeSet map[string]struct{},
	rule *model.RequiredFieldRule,
) *domainerrors.AppError {
	if rawItems, hasItems := schemaRaw["items"]; hasItems {
		itemsMap, ok := support.ToStringMap(rawItems)
		if !ok {
			return domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s.schema.items must be an object", fieldPath),
				nil,
			)
		}
		if keyErr := schemachecks.EnsureOnlyKeys(fieldPath+".schema.items", itemsMap, "type", "refTypes"); keyErr != nil {
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
		if expressions.ContainsInterpolation(itemType) {
			return domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s.schema.items.type does not allow interpolation ${...}", fieldPath),
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

		if itemType == reservedkeys.SchemaTypeEntityRef {
			if refTypesRaw, hasRefTypes := itemsMap["refTypes"]; hasRefTypes {
				itemRefTypes, refTypesErr := parseRefTypes(fieldPath+".schema.items.refTypes", refTypesRaw, typeSet)
				if refTypesErr != nil {
					return refTypesErr
				}
				rule.ItemRefTypes = itemRefTypes
			}
		} else if _, hasRefTypes := itemsMap["refTypes"]; hasRefTypes {
			return domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s.schema.items.refTypes is allowed only for items.type entityRef", fieldPath),
				nil,
			)
		}
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

func parseRefTypes(path string, raw any, typeSet map[string]struct{}) ([]string, *domainerrors.AppError) {
	rawItems, ok := raw.([]any)
	if !ok || len(rawItems) == 0 {
		return nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("%s must be a non-empty list", path),
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
				fmt.Sprintf("%s[%d] must be non-empty string", path, idx),
				nil,
			)
		}
		if expressions.ContainsInterpolation(value) {
			return nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s[%d] does not allow interpolation ${...}", path, idx),
				nil,
			)
		}

		value = strings.TrimSpace(value)
		if _, exists := seen[value]; exists {
			return nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s contains duplicate '%s'", path, value),
				nil,
			)
		}
		if _, exists := typeSet[value]; !exists {
			return nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s references unknown entity type '%s'", path, value),
				nil,
			)
		}
		seen[value] = struct{}{}
		refTypes = append(refTypes, value)
	}

	sort.Strings(refTypes)
	return refTypes, nil
}
