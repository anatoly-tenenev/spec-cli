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

		pathCase := model.PathPatternCase{Use: usePattern, WhenPath: fmt.Sprintf("schema.entity.%s.path_pattern.cases[%d].when", typeName, idx)}
		rawWhen, hasWhen := caseMap["when"]
		if !hasWhen {
			unconditionalIndexes = append(unconditionalIndexes, idx)
			cases = append(cases, pathCase)
			continue
		}

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

		if strings.HasPrefix(token, "meta:") {
			fieldName := strings.TrimPrefix(token, "meta:")
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
			if !(rule.Type == "string" || rule.Type == "integer" || rule.Type == "boolean" || rule.Type == "null") {
				issues = append(issues, schemaIssue(
					"schema.expression.invalid_reference",
					fmt.Sprintf("meta placeholder '%s' requires field type string|integer|boolean|null", token),
					path,
					"8.5",
				))
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

		if strings.HasPrefix(token, "ref:") {
			parts := strings.Split(token, ":")
			if len(parts) != 3 {
				issues = append(issues, schemaIssue(
					"schema.path_pattern.invalid_placeholder",
					fmt.Sprintf("ref placeholder '%s' must use format ref:<field>:<part>", token),
					path,
					"8.5",
				))
				continue
			}

			fieldName := parts[1]
			part := parts[2]
			rule, exists := fieldsByName[fieldName]
			if !exists || rule.Type != "entity_ref" {
				issues = append(issues, schemaIssue(
					"schema.expression.invalid_reference",
					fmt.Sprintf("ref placeholder '%s' requires entity_ref field '%s'", token, fieldName),
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
					fmt.Sprintf("unsupported ref placeholder part '%s'", part),
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
