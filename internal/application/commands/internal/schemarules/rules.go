// Package schemarules evaluates what the compiled schema demands of a document
// against a concrete candidate: whether a field or section is required here,
// and whether a value matches the declared type and constraints.
//
// add and update ask the same schema the same questions, so a document one of
// them accepts must be accepted by the other.
package schemarules

import (
	"fmt"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/issues"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/writemodel"
	schemaexpressions "github.com/anatoly-tenenev/spec-cli/internal/application/schema/expressions"
	"github.com/anatoly-tenenev/spec-cli/internal/application/values"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

func EvaluateRequired(
	literal bool,
	expression *schemaexpressions.CompiledExpression,
	context map[string]any,
) (bool, *schemaexpressions.EvalError) {
	if expression == nil {
		return literal, nil
	}

	value, evalErr := schemaexpressions.Evaluate(expression, context)
	if evalErr != nil {
		return false, evalErr
	}
	return schemaexpressions.IsTruthy(value), nil
}

func resolveRuleValues(values []writemodel.RuleValue, context map[string]any) ([]any, *schemaexpressions.EvalError) {
	if len(values) == 0 {
		return nil, nil
	}

	resolved := make([]any, 0, len(values))
	for _, value := range values {
		resolvedValue, resolveErr := resolveRuleValue(value, context)
		if resolveErr != nil {
			return nil, resolveErr
		}
		resolved = append(resolved, resolvedValue)
	}

	return resolved, nil
}

func resolveRuleValue(value writemodel.RuleValue, context map[string]any) (any, *schemaexpressions.EvalError) {
	if value.Template == nil {
		return value.Literal, nil
	}

	rendered, renderErr := schemaexpressions.RenderTemplate(value.Template, context)
	if renderErr != nil {
		return nil, renderErr
	}

	return rendered, nil
}

func Check(
	fieldSpec writemodel.MetaField,
	rawValue any,
	candidate *writemodel.Candidate,
	evaluationContext map[string]any,
) []domainvalidation.Issue {
	issuesList := make([]domainvalidation.Issue, 0)
	value := values.NormalizeValue(rawValue)

	typeMismatch := func(expected string) {
		issuesList = append(issuesList, issues.New(
			"meta.required_type_mismatch",
			fmt.Sprintf("field '%s' must be %s", fieldSpec.Name, expected),
			"11.5",
			"frontmatter."+fieldSpec.Name,
			candidate,
		))
	}

	switch fieldSpec.Type {
	case "string":
		if _, ok := value.(string); !ok {
			typeMismatch("string")
		}
	case "integer":
		number, ok := values.NumberToFloat64(value)
		if !ok || number != float64(int(number)) {
			typeMismatch("integer")
		}
	case "number":
		if _, ok := values.NumberToFloat64(value); !ok {
			typeMismatch("number")
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			typeMismatch("boolean")
		}
	case "entityRef":
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			typeMismatch("non-empty string")
		}
	case "array":
		arr, ok := value.([]any)
		if !ok {
			typeMismatch("array")
			break
		}
		if fieldSpec.HasMinItems && len(arr) < fieldSpec.MinItems {
			issuesList = append(issuesList, issues.New(
				"meta.required_array_min_items",
				fmt.Sprintf("field '%s' requires at least %d items", fieldSpec.Name, fieldSpec.MinItems),
				"11.5",
				"frontmatter."+fieldSpec.Name,
				candidate,
			))
		}
		if fieldSpec.HasMaxItems && len(arr) > fieldSpec.MaxItems {
			issuesList = append(issuesList, issues.New(
				"meta.required_array_max_items",
				fmt.Sprintf("field '%s' allows at most %d items", fieldSpec.Name, fieldSpec.MaxItems),
				"11.5",
				"frontmatter."+fieldSpec.Name,
				candidate,
			))
		}
		if fieldSpec.UniqueItems {
			for i := 0; i < len(arr); i++ {
				for j := i + 1; j < len(arr); j++ {
					if values.LiteralEqual(arr[i], arr[j]) {
						issuesList = append(issuesList, issues.New(
							"meta.required_array_unique_items",
							fmt.Sprintf("field '%s' requires unique items", fieldSpec.Name),
							"11.5",
							"frontmatter."+fieldSpec.Name,
							candidate,
						))
						break
					}
				}
			}
		}
		if fieldSpec.HasItems {
			for _, item := range arr {
				if !IsValueOfType(item, fieldSpec.ItemType) {
					issuesList = append(issuesList, issues.New(
						"meta.required_array_items_mismatch",
						fmt.Sprintf("field '%s' contains item with unsupported type", fieldSpec.Name),
						"11.5",
						"frontmatter."+fieldSpec.Name,
						candidate,
					))
					break
				}
			}
		}
	}

	if len(fieldSpec.Enum) > 0 {
		resolvedEnum, enumResolveErr := resolveRuleValues(fieldSpec.Enum, evaluationContext)
		if enumResolveErr != nil {
			issuesList = append(issuesList, issues.New(
				"meta.required_enum_interpolation_failed",
				fmt.Sprintf("field '%s' enum interpolation failed: %s", fieldSpec.Name, enumResolveErr.Message),
				"9.4",
				"frontmatter."+fieldSpec.Name,
				candidate,
			))
		}

		matched := false
		for _, enumValue := range resolvedEnum {
			if values.LiteralEqual(enumValue, value) {
				matched = true
				break
			}
		}
		if enumResolveErr == nil && !matched {
			issuesList = append(issuesList, issues.New(
				"meta.required_enum_mismatch",
				fmt.Sprintf("field '%s' value is outside enum", fieldSpec.Name),
				"11.5",
				"frontmatter."+fieldSpec.Name,
				candidate,
			))
		}
	}

	if fieldSpec.HasConst {
		resolvedConst, constResolveErr := resolveRuleValue(fieldSpec.Const, evaluationContext)
		if constResolveErr != nil {
			issuesList = append(issuesList, issues.New(
				"meta.required_const_interpolation_failed",
				fmt.Sprintf("field '%s' const interpolation failed: %s", fieldSpec.Name, constResolveErr.Message),
				"9.4",
				"frontmatter."+fieldSpec.Name,
				candidate,
			))
		} else if !values.LiteralEqual(resolvedConst, value) {
			issuesList = append(issuesList, issues.New(
				"meta.required_value_mismatch",
				fmt.Sprintf("field '%s' must match schema const", fieldSpec.Name),
				"11.5",
				"frontmatter."+fieldSpec.Name,
				candidate,
			))
		}
	}

	return issuesList
}

func IsValueOfType(value any, typeName string) bool {
	typeName = strings.TrimSpace(typeName)
	value = values.NormalizeValue(value)

	switch typeName {
	case "string":
		_, ok := value.(string)
		return ok
	case "integer":
		number, ok := values.NumberToFloat64(value)
		return ok && number == float64(int(number))
	case "number":
		_, ok := values.NumberToFloat64(value)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "entityRef":
		text, ok := value.(string)
		return ok && strings.TrimSpace(text) != ""
	default:
		return false
	}
}
