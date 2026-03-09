package entity

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/expressions"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	pathpattern "github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/schema/internal/entity/internal/pathpattern"
	schemachecks "github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/schema/internal/entity/internal/schemachecks"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

var idPrefixPattern = regexp.MustCompile(`^[A-Za-z0-9_]+(?:-[A-Za-z0-9_]+)*$`)

var builtinMetaSpecs = map[string]expressions.MetaFieldSpec{
	"type":         {Type: "string", Comparable: true},
	"id":           {Type: "string", Comparable: true},
	"slug":         {Type: "string", Comparable: true},
	"created_date": {Type: "string", Comparable: true},
	"updated_date": {Type: "string", Comparable: true},
}

func ParseType(
	typeName string,
	typeConfig map[string]any,
	typeSet map[string]struct{},
	usedPrefixes map[string]string,
) (model.SchemaEntityType, []domainvalidation.Issue, *domainerrors.AppError) {
	if keyErr := schemachecks.EnsureOnlyKeys(fmt.Sprintf("schema.entity.%s", typeName), typeConfig, "id_prefix", "path_pattern", "meta", "content", "description"); keyErr != nil {
		return model.SchemaEntityType{}, nil, keyErr
	}

	idPrefix, idPrefixErr := parseIDPrefix(typeName, typeConfig["id_prefix"], usedPrefixes)
	if idPrefixErr != nil {
		return model.SchemaEntityType{}, nil, idPrefixErr
	}

	requiredFields, fieldIssues, requiredFieldErr := parseRequiredFields(typeName, typeConfig["meta"], typeSet)
	if requiredFieldErr != nil {
		return model.SchemaEntityType{}, nil, requiredFieldErr
	}
	fieldByName := make(map[string]model.RequiredFieldRule, len(requiredFields))
	for _, fieldRule := range requiredFields {
		fieldByName[fieldRule.Name] = fieldRule
	}

	expressionContext := buildExpressionContext(requiredFields)
	requiredSections, sectionIssues, requiredSectionsErr := parseRequiredSections(typeName, typeConfig["content"], expressionContext, fieldByName)
	if requiredSectionsErr != nil {
		return model.SchemaEntityType{}, nil, requiredSectionsErr
	}

	pathRule, pathIssues, pathErr := pathpattern.Parse(typeName, typeConfig["path_pattern"], expressionContext, fieldByName)
	if pathErr != nil {
		return model.SchemaEntityType{}, nil, pathErr
	}

	issues := make([]domainvalidation.Issue, 0, len(fieldIssues)+len(sectionIssues)+len(pathIssues))
	issues = append(issues, fieldIssues...)
	issues = append(issues, sectionIssues...)
	issues = append(issues, pathIssues...)

	return model.SchemaEntityType{
		Name:             typeName,
		IDPrefix:         idPrefix,
		RequiredFields:   requiredFields,
		RequiredSections: requiredSections,
		PathPattern:      pathRule,
	}, issues, nil
}

func parseIDPrefix(typeName string, rawIDPrefix any, usedPrefixes map[string]string) (string, *domainerrors.AppError) {
	idPrefix, ok := rawIDPrefix.(string)
	if !ok || strings.TrimSpace(idPrefix) == "" {
		return "", domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.id_prefix must be a non-empty string", typeName),
			nil,
		)
	}

	if !idPrefixPattern.MatchString(idPrefix) {
		return "", domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.id_prefix has invalid format", typeName),
			nil,
		)
	}

	if existingType, exists := usedPrefixes[idPrefix]; exists {
		return "", domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			"schema contains duplicated id_prefix across entity types",
			map[string]any{"id_prefix": idPrefix, "types": []string{existingType, typeName}},
		)
	}

	usedPrefixes[idPrefix] = typeName
	return idPrefix, nil
}

func parseRequiredFields(
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

	rawItems, ok := support.ToSlice(rawFields)
	if !ok {
		return nil, nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields must be a list", typeName),
			nil,
		)
	}

	rules := make([]model.RequiredFieldRule, 0, len(rawItems))
	rawRules := make([]map[string]any, 0, len(rawItems))
	issues := make([]domainvalidation.Issue, 0)
	usedNames := map[string]struct{}{}

	for idx, item := range rawItems {
		rawRule, ok := support.ToStringMap(item)
		if !ok {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.fields[%d] must be an object", typeName, idx),
				nil,
			)
		}
		if keyErr := schemachecks.EnsureOnlyKeys(fmt.Sprintf("schema.entity.%s.meta.fields[%d]", typeName, idx), rawRule, "name", "required", "required_when", "description", "schema"); keyErr != nil {
			return nil, nil, keyErr
		}

		name, ok := rawRule["name"].(string)
		if !ok || strings.TrimSpace(name) == "" {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.fields[%d].name must be non-empty string", typeName, idx),
				nil,
			)
		}
		name = strings.TrimSpace(name)

		if _, isBuiltin := builtinMetaSpecs[name]; isBuiltin {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.fields cannot redefine built-in field '%s'", typeName, name),
				nil,
			)
		}
		if _, exists := usedNames[name]; exists {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.fields has duplicate field '%s'", typeName, name),
				nil,
			)
		}
		usedNames[name] = struct{}{}

		schemaRaw, ok := support.ToStringMap(rawRule["schema"])
		if !ok {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.fields[%d].schema must be an object", typeName, idx),
				nil,
			)
		}
		if keyErr := schemachecks.EnsureOnlyKeys(fmt.Sprintf("schema.entity.%s.meta.fields[%d].schema", typeName, idx), schemaRaw, "type", "const", "enum", "refTypes"); keyErr != nil {
			return nil, nil, keyErr
		}

		ruleTypeRaw, hasType := schemaRaw["type"]
		ruleType, ok := ruleTypeRaw.(string)
		if !hasType || !ok || strings.TrimSpace(ruleType) == "" {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.fields[%d].schema.type must be non-empty string", typeName, idx),
				nil,
			)
		}
		ruleType = strings.TrimSpace(ruleType)
		if !support.IsSupportedRuleType(ruleType) {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.fields[%d].schema.type uses unsupported type", typeName, idx),
				map[string]any{"type": ruleType},
			)
		}

		rule := model.RequiredFieldRule{
			Name:             name,
			Type:             ruleType,
			RequiredWhenPath: fmt.Sprintf("schema.entity.%s.meta.fields[%d].required_when", typeName, idx),
		}

		if enumRaw, exists := schemaRaw["enum"]; exists {
			enumValues, ok := support.ToSlice(enumRaw)
			if !ok || len(enumValues) == 0 {
				return nil, nil, domainerrors.New(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("schema.entity.%s.meta.fields[%d].schema.enum must be a non-empty list", typeName, idx),
					nil,
				)
			}
			for enumIndex, enumValue := range enumValues {
				if !support.MatchesRuleType(enumValue, ruleType) {
					return nil, nil, domainerrors.New(
						domainerrors.CodeSchemaInvalid,
						fmt.Sprintf("schema.entity.%s.meta.fields[%d].schema.enum[%d] does not match declared type", typeName, idx, enumIndex),
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
					fmt.Sprintf("schema.entity.%s.meta.fields[%d].schema.const does not match declared type", typeName, idx),
					nil,
				)
			}
			rule.HasValue = true
			rule.Value = value
		}

		if ruleType == "entity_ref" {
			refTypesRaw, hasRefTypes := schemaRaw["refTypes"]
			if hasRefTypes {
				refTypes, refTypesErr := parseRefTypes(typeName, idx, refTypesRaw, typeSet)
				if refTypesErr != nil {
					return nil, nil, refTypesErr
				}
				rule.RefTypes = refTypes
			}
		} else if _, hasRefTypes := schemaRaw["refTypes"]; hasRefTypes {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.fields[%d].schema.refTypes is allowed only for type entity_ref", typeName, idx),
				nil,
			)
		}

		rules = append(rules, rule)
		rawRules = append(rawRules, rawRule)
	}

	compileContext := buildExpressionContext(rules)
	for idx := range rules {
		required, requiredWhenLiteral, requiredWhenExpr, requiredIssues, requiredErr := parseRequiredConstraint(
			rawRules[idx],
			fmt.Sprintf("schema.entity.%s.meta.fields[%d]", typeName, idx),
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
	for idx, rule := range rules {
		if usage, hasUsage := schemachecks.StrictMissingUsageInRequiredWhen(rule.RequiredWhenExpr, fieldsByName); hasUsage {
			message := fmt.Sprintf(
				"schema.entity.%s.meta.fields[%d].required_when uses strict operator '%s' with potentially missing operand '%s'",
				typeName,
				idx,
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

func parseRefTypes(typeName string, index int, raw any, typeSet map[string]struct{}) ([]string, *domainerrors.AppError) {
	rawItems, ok := raw.([]any)
	if !ok || len(rawItems) == 0 {
		return nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields[%d].schema.refTypes must be a non-empty list", typeName, index),
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
				fmt.Sprintf("schema.entity.%s.meta.fields[%d].schema.refTypes[%d] must be non-empty string", typeName, index, idx),
				nil,
			)
		}
		value = strings.TrimSpace(value)
		if _, exists := seen[value]; exists {
			return nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.fields[%d].schema.refTypes contains duplicate '%s'", typeName, index, value),
				nil,
			)
		}
		if _, exists := typeSet[value]; !exists {
			return nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.fields[%d].schema.refTypes references unknown entity type '%s'", typeName, index, value),
				nil,
			)
		}
		seen[value] = struct{}{}
		refTypes = append(refTypes, value)
	}

	sort.Strings(refTypes)
	return refTypes, nil
}

func parseRequiredSections(
	typeName string,
	rawContent any,
	compileContext expressions.CompileContext,
	fieldsByName map[string]model.RequiredFieldRule,
) ([]model.RequiredSectionRule, []domainvalidation.Issue, *domainerrors.AppError) {
	if rawContent == nil {
		return nil, nil, nil
	}

	contentMap, ok := support.ToStringMap(rawContent)
	if !ok {
		return nil, nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.content must be a mapping", typeName),
			nil,
		)
	}
	if keyErr := schemachecks.EnsureOnlyKeys(fmt.Sprintf("schema.entity.%s.content", typeName), contentMap, "sections"); keyErr != nil {
		return nil, nil, keyErr
	}

	rawSections, exists := contentMap["sections"]
	if !exists || rawSections == nil {
		return nil, nil, nil
	}

	rawItems, ok := support.ToSlice(rawSections)
	if !ok || len(rawItems) == 0 {
		return nil, nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.content.sections must be a non-empty list", typeName),
			nil,
		)
	}

	rules := make([]model.RequiredSectionRule, 0, len(rawItems))
	rawRules := make([]map[string]any, 0, len(rawItems))
	issues := make([]domainvalidation.Issue, 0)
	seen := map[string]struct{}{}

	for idx, item := range rawItems {
		rawRule, ok := support.ToStringMap(item)
		if !ok {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.content.sections[%d] must be an object", typeName, idx),
				nil,
			)
		}
		if keyErr := schemachecks.EnsureOnlyKeys(fmt.Sprintf("schema.entity.%s.content.sections[%d]", typeName, idx), rawRule, "name", "required", "required_when", "title", "description"); keyErr != nil {
			return nil, nil, keyErr
		}

		name, ok := rawRule["name"].(string)
		if !ok || strings.TrimSpace(name) == "" {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.content.sections[%d].name must be non-empty string", typeName, idx),
				nil,
			)
		}
		name = strings.TrimSpace(name)
		if _, exists := seen[name]; exists {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.content.sections contains duplicate '%s'", typeName, name),
				nil,
			)
		}
		seen[name] = struct{}{}

		rule := model.RequiredSectionRule{
			Name:             name,
			RequiredWhenPath: fmt.Sprintf("schema.entity.%s.content.sections[%d].required_when", typeName, idx),
		}
		if rawTitle, hasTitle := rawRule["title"]; hasTitle {
			title, ok := rawTitle.(string)
			if !ok || strings.TrimSpace(title) == "" {
				return nil, nil, domainerrors.New(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("schema.entity.%s.content.sections[%d].title must be non-empty string", typeName, idx),
					nil,
				)
			}
			rule.HasTitle = true
			rule.Title = strings.TrimSpace(title)
		}
		rules = append(rules, rule)
		rawRules = append(rawRules, rawRule)
	}

	for idx := range rules {
		required, requiredWhenLiteral, requiredWhenExpr, requiredIssues, requiredErr := parseRequiredConstraint(
			rawRules[idx],
			fmt.Sprintf("schema.entity.%s.content.sections[%d]", typeName, idx),
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
	for idx, rule := range rules {
		if usage, hasUsage := schemachecks.StrictMissingUsageInRequiredWhen(rule.RequiredWhenExpr, fieldsByName); hasUsage {
			message := fmt.Sprintf(
				"schema.entity.%s.content.sections[%d].required_when uses strict operator '%s' with potentially missing operand '%s'",
				typeName,
				idx,
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

func parseRequiredConstraint(
	rawRule map[string]any,
	path string,
	compileContext expressions.CompileContext,
) (bool, bool, *expressions.Expression, []domainvalidation.Issue, *domainerrors.AppError) {
	required := true
	requiredWhen := false
	var requiredWhenExpr *expressions.Expression
	hasRequired := false
	hasRequiredWhen := false
	issues := make([]domainvalidation.Issue, 0)

	if rawRequired, exists := rawRule["required"]; exists {
		typed, ok := rawRequired.(bool)
		if !ok {
			return false, false, nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s.required must be boolean", path),
				nil,
			)
		}
		required = typed
		hasRequired = true
	}

	if rawRequiredWhen, exists := rawRule["required_when"]; exists {
		hasRequiredWhen = true
		switch typed := rawRequiredWhen.(type) {
		case bool:
			requiredWhen = typed
		case map[string]any:
			expression, compileIssues := expressions.Compile(typed, fmt.Sprintf("%s.required_when", path), compileContext)
			for _, compileIssue := range compileIssues {
				issues = append(issues, fromCompileIssue(compileIssue))
			}
			if len(compileIssues) == 0 {
				requiredWhenExpr = expression
			}
		default:
			return false, false, nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s.required_when must be boolean or expression object", path),
				nil,
			)
		}
	}

	if hasRequired && hasRequiredWhen && required {
		return false, false, nil, nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("%s cannot set required=true together with required_when", path),
			nil,
		)
	}

	if !hasRequired && hasRequiredWhen {
		required = false
		issues = append(issues, domainvalidation.Issue{
			Code:        "schema.required_when.required_not_explicit",
			Level:       domainvalidation.LevelWarning,
			Class:       "SchemaError",
			Message:     "required_when is set without explicit required; effective required=false is applied",
			StandardRef: "11.5",
			Field:       fmt.Sprintf("%s.required", path),
		})
	}

	return required, requiredWhen, requiredWhenExpr, issues, nil
}

func buildExpressionContext(fields []model.RequiredFieldRule) expressions.CompileContext {
	metaSpecs := make(map[string]expressions.MetaFieldSpec, len(builtinMetaSpecs)+len(fields))
	for name, spec := range builtinMetaSpecs {
		metaSpecs[name] = spec
	}
	for _, field := range fields {
		metaSpecs[field.Name] = expressions.MetaFieldSpec{
			Type:       field.Type,
			Comparable: isComparableFieldType(field.Type),
			EntityRef:  field.Type == "entity_ref",
		}
	}
	return expressions.CompileContext{MetaFields: metaSpecs}
}

func isComparableFieldType(typeName string) bool {
	switch typeName {
	case "string", "integer", "number", "boolean", "null", "entity_ref":
		return true
	default:
		return false
	}
}

func fromCompileIssue(issue expressions.CompileIssue) domainvalidation.Issue {
	standardRef := issue.StandardRef
	if standardRef == "" {
		standardRef = "11.6"
	}

	return domainvalidation.Issue{
		Code:        issue.Code,
		Level:       domainvalidation.LevelError,
		Class:       "SchemaError",
		Message:     issue.Message,
		StandardRef: standardRef,
		Field:       issue.Field,
	}
}
