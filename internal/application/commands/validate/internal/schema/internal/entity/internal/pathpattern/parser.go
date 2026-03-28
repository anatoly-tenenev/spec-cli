package pathpattern

import (
	"fmt"
	"sort"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/expressions"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	expressioncontext "github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/schema/internal/entity/internal/expressioncontext"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

func Parse(
	typeName string,
	rawPathPattern any,
	engine *expressions.Engine,
	fieldsByName map[string]model.RequiredFieldRule,
) (model.PathPatternRule, []domainvalidation.Issue, *domainerrors.AppError) {
	if rawPathPattern == nil {
		return model.PathPatternRule{}, []domainvalidation.Issue{schemaIssue(
			"schema.pathTemplate.missing",
			fmt.Sprintf("schema.entity.%s.pathTemplate is required", typeName),
			fmt.Sprintf("schema.entity.%s.pathTemplate", typeName),
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
				fmt.Sprintf("schema.entity.%s.pathTemplate.cases[%d] must be an object", typeName, idx),
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
				fmt.Sprintf("schema.entity.%s.pathTemplate.cases[%d].use must be non-empty string", typeName, idx),
				nil,
			)
		}
		usePattern = strings.TrimSpace(usePattern)
		useFieldPath := fmt.Sprintf("schema.entity.%s.pathTemplate.cases[%d].use", typeName, idx)

		useTemplate, compileErr := expressions.CompileTemplate(usePattern, engine)
		if compileErr != nil {
			return model.PathPatternRule{}, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s has invalid interpolation in pathTemplate.use context: %s", useFieldPath, compileErr.Message),
				map[string]any{
					"code":         compileErr.Code,
					"field":        useFieldPath,
					"offset":       compileErr.Offset,
					"standard_ref": "9.1",
				},
			)
		}

		pathCase := model.PathPatternCase{
			Use:         usePattern,
			UseTemplate: useTemplate,
			WhenPath:    fmt.Sprintf("schema.entity.%s.pathTemplate.cases[%d].when", typeName, idx),
		}

		rawWhen, hasWhen := caseMap["when"]
		if !hasWhen {
			unconditionalIndexes = append(unconditionalIndexes, idx)
		} else {
			pathCase.HasWhen = true
			switch typed := rawWhen.(type) {
			case bool:
				pathCase.When = typed
			case string:
				expression, expressionErr := expressions.CompileScalarInterpolation(typed, engine)
				if expressionErr != nil {
					return model.PathPatternRule{}, nil, domainerrors.New(
						domainerrors.CodeSchemaInvalid,
						fmt.Sprintf("%s has invalid expression in when context: %s", pathCase.WhenPath, expressionErr.Message),
						map[string]any{
							"code":         expressionErr.Code,
							"field":        pathCase.WhenPath,
							"offset":       expressionErr.Offset,
							"standard_ref": "11.6",
						},
					)
				}
				pathCase.WhenExpr = expression
			default:
				return model.PathPatternRule{}, nil, domainerrors.New(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("%s must be boolean or string interpolation ${expr}", pathCase.WhenPath),
					nil,
				)
			}
		}

		if guardErr := validateUseGuardSafety(typeName, idx, useFieldPath, pathCase, fieldsByName); guardErr != nil {
			return model.PathPatternRule{}, nil, guardErr
		}

		cases = append(cases, pathCase)
	}

	if len(cases) == 0 {
		issues = append(issues, schemaIssue(
			"schema.pathTemplate.empty_cases",
			fmt.Sprintf("schema.entity.%s.pathTemplate.cases must be non-empty", typeName),
			fmt.Sprintf("schema.entity.%s.pathTemplate.cases", typeName),
			"8.3",
		))
	}

	if len(unconditionalIndexes) != 1 {
		issues = append(issues, schemaIssue(
			"schema.pathTemplate.unconditional_case_count",
			fmt.Sprintf("schema.entity.%s.pathTemplate must contain exactly one unconditional case", typeName),
			fmt.Sprintf("schema.entity.%s.pathTemplate.cases", typeName),
			"8.3",
		))
	}

	if len(unconditionalIndexes) == 1 && unconditionalIndexes[0] != len(cases)-1 {
		issues = append(issues, schemaIssue(
			"schema.pathTemplate.unconditional_case_position",
			fmt.Sprintf("schema.entity.%s.pathTemplate unconditional case must be the last one", typeName),
			fmt.Sprintf("schema.entity.%s.pathTemplate.cases[%d]", typeName, unconditionalIndexes[0]),
			"8.3",
		))
	}

	return model.PathPatternRule{Cases: cases}, issues, nil
}

func validateUseGuardSafety(
	typeName string,
	caseIndex int,
	useFieldPath string,
	pathCase model.PathPatternCase,
	fieldsByName map[string]model.RequiredFieldRule,
) *domainerrors.AppError {
	if pathCase.HasWhen && pathCase.WhenExpr == nil && !pathCase.When {
		return nil
	}

	for _, part := range pathCase.UseTemplate.Parts {
		if part.Expression == nil {
			continue
		}

		requiredRoots := requiredGuardRoots(part.Expression, fieldsByName)
		for _, root := range requiredRoots {
			if expressioncontext.IsPathGuaranteedBySchema(root, fieldsByName) {
				continue
			}
			if pathCase.WhenExpr != nil && pathCase.WhenExpr.ProtectsWhenTrue(root) {
				continue
			}
			guardLabel := pathCase.WhenPath
			if !pathCase.HasWhen {
				guardLabel = "missing when guard"
			}

			return domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s interpolation '${%s}' is not protected by %s for path '%s'", useFieldPath, part.Expression.Source, guardLabel, root),
				map[string]any{
					"code":         "schema.pathTemplate.use_missing_guard",
					"field":        useFieldPath,
					"standard_ref": "8.6",
				},
			)
		}
	}

	return nil
}

func requiredGuardRoots(expression *expressions.CompiledExpression, fieldsByName map[string]model.RequiredFieldRule) []string {
	paths := expression.GuardedPathsWhenTrue()
	if len(paths) == 0 {
		return nil
	}

	roots := map[string]struct{}{}
	for _, path := range paths {
		root, requiresGuard := expressioncontext.GuardRootForPath(path, fieldsByName)
		if !requiresGuard {
			continue
		}
		roots[root] = struct{}{}
	}

	if len(roots) == 0 {
		return nil
	}

	result := make([]string, 0, len(roots))
	for root := range roots {
		result = append(result, root)
	}
	sort.Strings(result)
	return result
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
				fmt.Sprintf("schema.entity.%s.pathTemplate.cases is required", typeName),
				nil,
			)
		}
		cases, ok := rawCases.([]any)
		if !ok {
			return nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.pathTemplate.cases must be a list", typeName),
				nil,
			)
		}
		return cases, nil
	default:
		return nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.pathTemplate must be string, list, or object", typeName),
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
			fmt.Sprintf("schema.entity.%s.pathTemplate has unsupported key '%s'", typeName, key),
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
				fmt.Sprintf("schema.entity.%s.pathTemplate.cases[%d] has unsupported key '%s'", typeName, caseIndex, key),
				nil,
			)
		}
	}
	return nil
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
