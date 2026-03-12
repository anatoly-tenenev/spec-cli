package pathpattern

import (
	"fmt"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/expressions"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

func Parse(
	typeName string,
	rawPathPattern any,
	compileContext expressions.CompileContext,
	fieldsByName map[string]model.RequiredFieldRule,
) (model.PathPatternRule, []domainvalidation.Issue, *domainerrors.AppError) {
	if rawPathPattern == nil {
		return model.PathPatternRule{}, []domainvalidation.Issue{schemaIssue(
			"schema.path_pattern.missing",
			fmt.Sprintf("schema.entity.%s.path_pattern is required", typeName),
			fmt.Sprintf("schema.entity.%s.path_pattern", typeName),
			"8",
		)}, nil
	}

	rawCases, casesErr := normalizeCases(typeName, rawPathPattern)
	if casesErr != nil {
		return model.PathPatternRule{}, nil, casesErr
	}

	cases := make([]model.PathPatternCase, 0, len(rawCases))
	issues := make([]domainvalidation.Issue, 0)
	unconditionalIndexes := make([]int, 0)

	for idx, rawCase := range rawCases {
		caseMap, ok := support.ToStringMap(rawCase)
		if !ok {
			return model.PathPatternRule{}, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.path_pattern.cases[%d] must be an object", typeName, idx),
				nil,
			)
		}
		if keyErr := validateCaseKeys(typeName, idx, caseMap); keyErr != nil {
			return model.PathPatternRule{}, nil, keyErr
		}

		useRaw, hasUse := caseMap["use"]
		usePattern, ok := useRaw.(string)
		if !hasUse || !ok || strings.TrimSpace(usePattern) == "" {
			return model.PathPatternRule{}, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.path_pattern.cases[%d].use must be non-empty string", typeName, idx),
				nil,
			)
		}
		usePattern = strings.TrimSpace(usePattern)
		issues = append(issues, validateTemplate(typeName, idx, usePattern, fieldsByName)...)
		usePath := fmt.Sprintf("schema.entity.%s.path_pattern.cases[%d].use", typeName, idx)

		pathCase := model.PathPatternCase{Use: usePattern, WhenPath: fmt.Sprintf("schema.entity.%s.path_pattern.cases[%d].when", typeName, idx)}
		rawWhen, hasWhen := caseMap["when"]
		if !hasWhen {
			unconditionalIndexes = append(unconditionalIndexes, idx)
		} else {
			pathCase.HasWhen = true
			switch typed := rawWhen.(type) {
			case bool:
				pathCase.When = typed
			case map[string]any:
				expression, compileIssues := expressions.Compile(typed, pathCase.WhenPath, compileContext)
				if len(compileIssues) > 0 {
					for _, compileIssue := range compileIssues {
						issues = append(issues, fromCompileIssue(compileIssue))
					}
				} else {
					pathCase.WhenExpr = expression
				}
			default:
				return model.PathPatternRule{}, nil, domainerrors.New(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("%s must be boolean or expression object", pathCase.WhenPath),
					nil,
				)
			}
		}

		if strictErr := validateStrictWhenOperands(typeName, idx, pathCase, fieldsByName); strictErr != nil {
			return model.PathPatternRule{}, nil, strictErr
		}
		if placeholderErr := validateTemplatePlaceholderAvailability(typeName, idx, usePattern, usePath, pathCase, fieldsByName); placeholderErr != nil {
			return model.PathPatternRule{}, nil, placeholderErr
		}
		cases = append(cases, pathCase)
	}

	if len(cases) == 0 {
		issues = append(issues, schemaIssue(
			"schema.path_pattern.empty_cases",
			fmt.Sprintf("schema.entity.%s.path_pattern.cases must be non-empty", typeName),
			fmt.Sprintf("schema.entity.%s.path_pattern.cases", typeName),
			"8.3",
		))
	}

	if len(unconditionalIndexes) != 1 {
		issues = append(issues, schemaIssue(
			"schema.path_pattern.unconditional_case_count",
			fmt.Sprintf("schema.entity.%s.path_pattern must contain exactly one unconditional case", typeName),
			fmt.Sprintf("schema.entity.%s.path_pattern.cases", typeName),
			"8.3",
		))
	}

	if len(unconditionalIndexes) == 1 && unconditionalIndexes[0] != len(cases)-1 {
		issues = append(issues, schemaIssue(
			"schema.path_pattern.unconditional_case_position",
			fmt.Sprintf("schema.entity.%s.path_pattern unconditional case must be the last one", typeName),
			fmt.Sprintf("schema.entity.%s.path_pattern.cases[%d]", typeName, unconditionalIndexes[0]),
			"8.3",
		))
	}

	return model.PathPatternRule{Cases: cases}, issues, nil
}

func normalizeCases(typeName string, raw any) ([]any, *domainerrors.AppError) {
	switch typed := raw.(type) {
	case string:
		return []any{map[string]any{"use": strings.TrimSpace(typed)}}, nil
	case []any:
		return typed, nil
	case map[string]any:
		if keyErr := validatePathPatternObjectKeys(typeName, typed); keyErr != nil {
			return nil, keyErr
		}
		rawCases, exists := typed["cases"]
		if !exists {
			return nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.path_pattern.cases is required", typeName),
				nil,
			)
		}
		cases, ok := rawCases.([]any)
		if !ok {
			return nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.path_pattern.cases must be a list", typeName),
				nil,
			)
		}
		return cases, nil
	default:
		return nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.path_pattern must be string, list, or object", typeName),
			nil,
		)
	}
}

func validatePathPatternObjectKeys(typeName string, values map[string]any) *domainerrors.AppError {
	for key := range values {
		if key == "cases" {
			continue
		}
		return domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.path_pattern has unsupported key '%s'", typeName, key),
			nil,
		)
	}
	return nil
}

func validateCaseKeys(typeName string, caseIndex int, values map[string]any) *domainerrors.AppError {
	for key := range values {
		switch key {
		case "use", "when":
			continue
		default:
			return domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.path_pattern.cases[%d] has unsupported key '%s'", typeName, caseIndex, key),
				nil,
			)
		}
	}
	return nil
}

func validateTemplate(
	typeName string,
	caseIndex int,
	template string,
	fieldsByName map[string]model.RequiredFieldRule,
) []domainvalidation.Issue {
	path := fmt.Sprintf("schema.entity.%s.path_pattern.cases[%d].use", typeName, caseIndex)
	placeholders, err := extractPlaceholders(template)
	if err != nil {
		return []domainvalidation.Issue{schemaIssue(
			"schema.path_pattern.invalid_placeholder",
			err.Error(),
			path,
			"9",
		)}
	}

	issues := make([]domainvalidation.Issue, 0)
	for _, token := range placeholders {
		switch token {
		case "id", "slug", "created_date", "updated_date":
			continue
		}

		if strings.HasPrefix(token, "meta.") {
			fieldName := strings.TrimPrefix(token, "meta.")
			if fieldName == "" || strings.Contains(fieldName, ".") {
				issues = append(issues, schemaIssue(
					"schema.path_pattern.invalid_placeholder",
					fmt.Sprintf("meta placeholder '%s' must use format meta.<field>", token),
					path,
					"8.5",
				))
				continue
			}

			rule, exists := fieldsByName[fieldName]
			if !exists {
				issues = append(issues, schemaIssue(
					"schema.expression.invalid_reference",
					fmt.Sprintf("unknown meta field '%s' in path_pattern placeholder", fieldName),
					path,
					"8.5",
				))
				continue
			}
			if rule.Type == "entity_ref" {
				continue
			}
			if !(rule.Type == "string" || rule.Type == "integer" || rule.Type == "boolean" || rule.Type == "null") {
				issues = append(issues, schemaIssue(
					"schema.expression.invalid_reference",
					fmt.Sprintf("meta placeholder '%s' requires field type string|integer|boolean|null|entity_ref", token),
					path,
					"8.5",
				))
				continue
			}
			if len(rule.Enum) == 0 {
				issues = append(issues, schemaIssue(
					"schema.expression.invalid_reference",
					fmt.Sprintf("meta placeholder '%s' requires schema.enum on the field", token),
					path,
					"8.5",
				))
			}
			continue
		}

		if strings.HasPrefix(token, "refs.") {
			parts := strings.Split(token, ".")
			if len(parts) != 3 {
				issues = append(issues, schemaIssue(
					"schema.path_pattern.invalid_placeholder",
					fmt.Sprintf("refs placeholder '%s' must use format refs.<field>.<part>", token),
					path,
					"8.5",
				))
				continue
			}

			fieldName := parts[1]
			part := parts[2]
			if fieldName == "" {
				issues = append(issues, schemaIssue(
					"schema.path_pattern.invalid_placeholder",
					fmt.Sprintf("refs placeholder '%s' must include field name", token),
					path,
					"8.5",
				))
				continue
			}
			rule, exists := fieldsByName[fieldName]
			if !exists || rule.Type != "entity_ref" {
				issues = append(issues, schemaIssue(
					"schema.expression.invalid_reference",
					fmt.Sprintf("refs placeholder '%s' requires entity_ref field '%s'", token, fieldName),
					path,
					"8.5",
				))
				continue
			}
			switch part {
			case "id", "type", "slug", "dir_path":
			default:
				issues = append(issues, schemaIssue(
					"schema.path_pattern.invalid_placeholder",
					fmt.Sprintf("unsupported refs placeholder part '%s'", part),
					path,
					"8.5",
				))
			}
			continue
		}

		issues = append(issues, schemaIssue(
			"schema.path_pattern.invalid_placeholder",
			fmt.Sprintf("unsupported placeholder '{%s}'", token),
			path,
			"9",
		))
	}

	return issues
}

func validateStrictWhenOperands(
	typeName string,
	caseIndex int,
	pathCase model.PathPatternCase,
	fieldsByName map[string]model.RequiredFieldRule,
) *domainerrors.AppError {
	if pathCase.WhenExpr == nil {
		return nil
	}

	for _, usage := range expressions.CollectStrictReferenceUsages(pathCase.WhenExpr) {
		if !referencePotentiallyMissing(usage.Reference, fieldsByName) {
			continue
		}

		return domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf(
				"schema.entity.%s.path_pattern.cases[%d].when uses strict operator '%s' with potentially missing operand '%s'",
				typeName,
				caseIndex,
				usage.Operator,
				usage.Reference.Raw,
			),
			map[string]any{
				"field":    pathCase.WhenPath,
				"operator": string(usage.Operator),
				"operand":  usage.Reference.Raw,
			},
		)
	}

	return nil
}

func validateTemplatePlaceholderAvailability(
	typeName string,
	caseIndex int,
	template string,
	usePath string,
	pathCase model.PathPatternCase,
	fieldsByName map[string]model.RequiredFieldRule,
) *domainerrors.AppError {
	placeholders, err := extractPlaceholders(template)
	if err != nil {
		return nil
	}

	guardKeys := collectCaseGuardKeys(pathCase, fieldsByName)
	for _, token := range placeholders {
		reference, hasReference := placeholderReference(token)
		if !hasReference {
			continue
		}
		if !referencePotentiallyMissing(reference, fieldsByName) {
			continue
		}
		targetKey := presenceKeyForReference(reference, fieldsByName)
		if targetKey == "" {
			continue
		}
		if _, guarded := guardKeys[targetKey]; guarded {
			continue
		}

		return domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf(
				"schema.entity.%s.path_pattern.cases[%d].use placeholder '{%s}' references potentially missing value without static guard",
				typeName,
				caseIndex,
				token,
			),
			map[string]any{
				"field":       usePath,
				"placeholder": token,
			},
		)
	}

	return nil
}

func placeholderReference(token string) (expressions.Reference, bool) {
	switch token {
	case "id", "slug", "created_date", "updated_date":
		return expressions.Reference{}, false
	}

	if strings.HasPrefix(token, "meta.") {
		field := strings.TrimPrefix(token, "meta.")
		if field == "" || strings.Contains(field, ".") {
			return expressions.Reference{}, false
		}
		return expressions.Reference{
			Kind:  expressions.ReferenceMeta,
			Field: field,
			Raw:   "meta." + field,
		}, true
	}

	if strings.HasPrefix(token, "refs.") {
		parts := strings.Split(token, ".")
		if len(parts) != 3 {
			return expressions.Reference{}, false
		}
		if parts[1] == "" {
			return expressions.Reference{}, false
		}
		switch parts[2] {
		case "id", "type", "slug", "dir_path":
		default:
			return expressions.Reference{}, false
		}
		return expressions.Reference{
			Kind:  expressions.ReferenceRefs,
			Field: parts[1],
			Part:  parts[2],
			Raw:   "refs." + parts[1] + "." + parts[2],
		}, true
	}

	return expressions.Reference{}, false
}

func referencePotentiallyMissing(reference expressions.Reference, fieldsByName map[string]model.RequiredFieldRule) bool {
	switch reference.Kind {
	case expressions.ReferenceMeta:
		if isBuiltinMetaField(reference.Field) {
			return false
		}

		rule, exists := fieldsByName[reference.Field]
		if !exists {
			return false
		}
		if !rule.Required {
			return true
		}
		return rule.RequiredWhen || rule.RequiredWhenExpr != nil
	case expressions.ReferenceRefs:
		_, exists := fieldsByName[reference.Field]
		return exists
	default:
		return true
	}
}

func collectCaseGuardKeys(
	pathCase model.PathPatternCase,
	fieldsByName map[string]model.RequiredFieldRule,
) map[string]struct{} {
	guards := map[string]struct{}{}
	if !pathCase.HasWhen || pathCase.WhenExpr == nil {
		return guards
	}

	switch pathCase.WhenExpr.Operator {
	case expressions.OpExists:
		if pathCase.WhenExpr.ExistsRef != nil {
			guards[presenceKeyForReference(*pathCase.WhenExpr.ExistsRef, fieldsByName)] = struct{}{}
		}
	case expressions.OpAll:
		for _, subexpression := range pathCase.WhenExpr.Subexpressions {
			if subexpression == nil || subexpression.Operator != expressions.OpExists || subexpression.ExistsRef == nil {
				continue
			}
			guards[presenceKeyForReference(*subexpression.ExistsRef, fieldsByName)] = struct{}{}
		}
	}

	return guards
}

func presenceKeyForReference(reference expressions.Reference, fieldsByName map[string]model.RequiredFieldRule) string {
	_ = fieldsByName

	switch reference.Kind {
	case expressions.ReferenceMeta:
		if isBuiltinMetaField(reference.Field) {
			return "builtin:" + reference.Field
		}
		return "meta:" + reference.Field
	case expressions.ReferenceRefs:
		return "refs:" + reference.Field
	default:
		return ""
	}
}

func isBuiltinMetaField(name string) bool {
	switch name {
	case "type", "id", "slug", "created_date", "updated_date":
		return true
	default:
		return false
	}
}

func extractPlaceholders(template string) ([]string, error) {
	placeholders := make([]string, 0)
	for idx := 0; idx < len(template); idx++ {
		if template[idx] == '}' {
			return nil, fmt.Errorf("template contains unexpected '}'")
		}
		if template[idx] != '{' {
			continue
		}

		endOffset := strings.IndexByte(template[idx+1:], '}')
		if endOffset < 0 {
			return nil, fmt.Errorf("template contains unclosed '{'")
		}

		token := template[idx+1 : idx+1+endOffset]
		if token == "" {
			return nil, fmt.Errorf("template contains empty placeholder '{}'")
		}
		if strings.Contains(token, "{") || strings.Contains(token, "}") {
			return nil, fmt.Errorf("template contains nested braces")
		}

		placeholders = append(placeholders, token)
		idx = idx + endOffset + 1
	}

	return placeholders, nil
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

func schemaIssue(code string, message string, field string, standardRef string) domainvalidation.Issue {
	return domainvalidation.Issue{
		Code:        code,
		Level:       domainvalidation.LevelError,
		Class:       "SchemaError",
		Message:     message,
		StandardRef: standardRef,
		Field:       field,
	}
}
