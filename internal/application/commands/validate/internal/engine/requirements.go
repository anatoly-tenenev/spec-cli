package engine

import (
	"fmt"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/expressions"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/support"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

func validateRequiredFields(
	issues *[]domainvalidation.Issue,
	entity *model.CheckedEntity,
	frontmatter map[string]any,
	typeSpec model.SchemaEntityType,
	context runtimeExpressionContext,
) {
	for _, rule := range typeSpec.RequiredFields {
		requiredWhen := evaluateRequiredWhen(
			issues,
			entity,
			rule.RequiredWhen,
			rule.RequiredWhenExpr,
			rule.RequiredWhenPath,
			fmt.Sprintf("metadata field '%s'", rule.Name),
			context,
		)
		required := rule.Required || requiredWhen

		value, exists := frontmatter[rule.Name]
		if !exists {
			if !required {
				continue
			}
			addIssue(issues, entity, domainvalidation.Issue{
				Code:        "meta.required_missing",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     fmt.Sprintf("required metadata field '%s' is missing", rule.Name),
				StandardRef: "11.5",
				Field:       fmt.Sprintf("frontmatter.%s", rule.Name),
			})
			continue
		}

		if !support.MatchesRuleType(value, rule.Type) {
			addIssue(issues, entity, domainvalidation.Issue{
				Code:        "meta.required_type_mismatch",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     fmt.Sprintf("metadata field '%s' must be of type '%s'", rule.Name, rule.Type),
				StandardRef: "12.3",
				Field:       fmt.Sprintf("frontmatter.%s", rule.Name),
			})
			continue
		}

		if rule.Type == "array" {
			arrayValue, ok := value.([]any)
			if !ok {
				addIssue(issues, entity, domainvalidation.Issue{
					Code:        "meta.required_type_mismatch",
					Level:       domainvalidation.LevelError,
					Class:       "InstanceError",
					Message:     fmt.Sprintf("metadata field '%s' must be of type '%s'", rule.Name, rule.Type),
					StandardRef: "12.3",
					Field:       fmt.Sprintf("frontmatter.%s", rule.Name),
				})
				continue
			}
			validateArrayField(issues, entity, rule, arrayValue)
		}

		resolvedEnum, enumResolveErr := resolveStringRuleValues(rule.Enum, context)
		if enumResolveErr != nil {
			addIssue(issues, entity, domainvalidation.Issue{
				Code:        "meta.required_enum_placeholder_unresolved",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     fmt.Sprintf("metadata field '%s' enum placeholder is unresolved: %s", rule.Name, enumResolveErr.Error()),
				StandardRef: "9.4",
				Field:       fmt.Sprintf("frontmatter.%s", rule.Name),
			})
			continue
		}

		if len(resolvedEnum) > 0 && !support.ContainsEnumValue(resolvedEnum, value) {
			addIssue(issues, entity, domainvalidation.Issue{
				Code:        "meta.required_enum_mismatch",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     fmt.Sprintf("metadata field '%s' is not in enum", rule.Name),
				StandardRef: "12.3",
				Field:       fmt.Sprintf("frontmatter.%s", rule.Name),
			})
		}

		resolvedValue := rule.Value
		if rule.HasValue {
			resolvedValues, valueResolveErr := resolveStringRuleValues([]any{rule.Value}, context)
			if valueResolveErr != nil {
				addIssue(issues, entity, domainvalidation.Issue{
					Code:        "meta.required_const_placeholder_unresolved",
					Level:       domainvalidation.LevelError,
					Class:       "InstanceError",
					Message:     fmt.Sprintf("metadata field '%s' const placeholder is unresolved: %s", rule.Name, valueResolveErr.Error()),
					StandardRef: "9.4",
					Field:       fmt.Sprintf("frontmatter.%s", rule.Name),
				})
				continue
			}
			if len(resolvedValues) == 1 {
				resolvedValue = resolvedValues[0]
			}
		}

		if rule.HasValue && !support.LiteralEqual(value, resolvedValue) {
			addIssue(issues, entity, domainvalidation.Issue{
				Code:        "meta.required_value_mismatch",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     fmt.Sprintf("metadata field '%s' does not match required value", rule.Name),
				StandardRef: "12.3",
				Field:       fmt.Sprintf("frontmatter.%s", rule.Name),
			})
		}
	}
}

func validateArrayField(
	issues *[]domainvalidation.Issue,
	entity *model.CheckedEntity,
	rule model.RequiredFieldRule,
	arrayValue []any,
) {
	fieldPath := fmt.Sprintf("frontmatter.%s", rule.Name)
	itemCount := len(arrayValue)

	if rule.HasMinItems && itemCount < rule.MinItems {
		addIssue(issues, entity, domainvalidation.Issue{
			Code:        "meta.required_array_min_items",
			Level:       domainvalidation.LevelError,
			Class:       "InstanceError",
			Message:     fmt.Sprintf("metadata field '%s' must contain at least %d items", rule.Name, rule.MinItems),
			StandardRef: "12.3",
			Field:       fieldPath,
		})
	}

	if rule.HasMaxItems && itemCount > rule.MaxItems {
		addIssue(issues, entity, domainvalidation.Issue{
			Code:        "meta.required_array_max_items",
			Level:       domainvalidation.LevelError,
			Class:       "InstanceError",
			Message:     fmt.Sprintf("metadata field '%s' must contain at most %d items", rule.Name, rule.MaxItems),
			StandardRef: "12.3",
			Field:       fieldPath,
		})
	}

	if rule.HasItemType {
		for idx, item := range arrayValue {
			if support.MatchesRuleType(item, rule.ItemType) {
				continue
			}
			addIssue(issues, entity, domainvalidation.Issue{
				Code:        "meta.required_array_items_mismatch",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     fmt.Sprintf("metadata field '%s' item at index %d must be of type '%s'", rule.Name, idx, rule.ItemType),
				StandardRef: "12.3",
				Field:       fieldPath,
			})
		}
	}

	if rule.UniqueItems {
		for left := 0; left < itemCount; left++ {
			for right := left + 1; right < itemCount; right++ {
				if !support.LiteralEqual(arrayValue[left], arrayValue[right]) {
					continue
				}
				addIssue(issues, entity, domainvalidation.Issue{
					Code:        "meta.required_array_unique_items",
					Level:       domainvalidation.LevelError,
					Class:       "InstanceError",
					Message:     fmt.Sprintf("metadata field '%s' must contain unique items", rule.Name),
					StandardRef: "12.3",
					Field:       fieldPath,
				})
				return
			}
		}
	}
}

func resolveStringRuleValues(values []any, context runtimeExpressionContext) ([]any, error) {
	if len(values) == 0 {
		return nil, nil
	}

	resolved := make([]any, 0, len(values))
	for _, value := range values {
		stringValue, ok := value.(string)
		if !ok || !strings.Contains(stringValue, "{") {
			resolved = append(resolved, value)
			continue
		}

		rendered, renderErr := renderMetadataTemplate(stringValue, context)
		if renderErr != nil {
			return nil, renderErr
		}
		resolved = append(resolved, rendered)
	}

	return resolved, nil
}

func renderMetadataTemplate(template string, context runtimeExpressionContext) (string, error) {
	var builder strings.Builder
	for idx := 0; idx < len(template); idx++ {
		current := template[idx]
		if current == '}' {
			return "", fmt.Errorf("template contains unexpected '}'")
		}
		if current != '{' {
			builder.WriteByte(current)
			continue
		}

		endOffset := strings.IndexByte(template[idx+1:], '}')
		if endOffset < 0 {
			return "", fmt.Errorf("template contains unclosed '{'")
		}
		token := template[idx+1 : idx+1+endOffset]
		if token == "" {
			return "", fmt.Errorf("template contains empty placeholder")
		}

		resolvedValue, resolveErr := resolveMetadataPlaceholder(token, context)
		if resolveErr != nil {
			return "", resolveErr
		}
		builder.WriteString(resolvedValue)
		idx = idx + endOffset + 1
	}
	return builder.String(), nil
}

func resolveMetadataPlaceholder(token string, context runtimeExpressionContext) (string, error) {
	switch token {
	case "id", "slug", "created_date", "updated_date":
		value, exists := context.ResolveReference(expressions.Reference{Kind: expressions.ReferenceMeta, Field: token, Raw: "meta." + token})
		if !exists {
			return "", fmt.Errorf("placeholder '{%s}' is missing", token)
		}
		return stringifyPathValue(value, token)
	}

	if strings.HasPrefix(token, "meta.") {
		fieldName := strings.TrimPrefix(token, "meta.")
		if fieldName == "" || strings.Contains(fieldName, ".") {
			return "", fmt.Errorf("placeholder '{%s}' has invalid format", token)
		}
		value, exists := context.ResolveReference(expressions.Reference{Kind: expressions.ReferenceMeta, Field: fieldName, Raw: "meta." + fieldName})
		if !exists {
			return "", fmt.Errorf("placeholder '{%s}' is missing", token)
		}
		return stringifyPathValue(value, token)
	}

	if strings.HasPrefix(token, "refs.") {
		parts := strings.Split(token, ".")
		if len(parts) != 3 {
			return "", fmt.Errorf("placeholder '{%s}' has invalid format", token)
		}
		if parts[1] == "" {
			return "", fmt.Errorf("placeholder '{%s}' has invalid format", token)
		}
		reference := expressions.Reference{Kind: expressions.ReferenceRefs, Field: parts[1], Part: parts[2], Raw: "refs." + parts[1] + "." + parts[2]}
		value, exists := context.ResolveReference(reference)
		if !exists {
			return "", fmt.Errorf("placeholder '{%s}' is missing", token)
		}
		return stringifyPathValue(value, token)
	}

	return "", fmt.Errorf("unsupported placeholder '{%s}'", token)
}

func validateRequiredSections(
	issues *[]domainvalidation.Issue,
	entity *model.CheckedEntity,
	sections map[string]string,
	duplicateLabels []string,
	typeSpec model.SchemaEntityType,
	context runtimeExpressionContext,
) {
	for _, label := range duplicateLabels {
		addIssue(issues, entity, domainvalidation.Issue{
			Code:        "content.section_label_duplicate",
			Level:       domainvalidation.LevelError,
			Class:       "InstanceError",
			Message:     fmt.Sprintf("section label '%s' is duplicated", label),
			StandardRef: "13.2",
		})
	}

	for _, sectionRule := range typeSpec.RequiredSections {
		requiredWhen := evaluateRequiredWhen(
			issues,
			entity,
			sectionRule.RequiredWhen,
			sectionRule.RequiredWhenExpr,
			sectionRule.RequiredWhenPath,
			fmt.Sprintf("content section '%s'", sectionRule.Name),
			context,
		)

		required := sectionRule.Required || requiredWhen
		if !required {
			continue
		}

		if title, exists := sections[sectionRule.Name]; exists {
			if sectionRule.HasTitle && title != sectionRule.Title {
				addIssue(issues, entity, domainvalidation.Issue{
					Code:        "content.section_title_mismatch",
					Level:       domainvalidation.LevelError,
					Class:       "InstanceError",
					Message:     fmt.Sprintf("section '%s' title must be '%s'", sectionRule.Name, sectionRule.Title),
					StandardRef: "13.2",
					Field:       fmt.Sprintf("content.sections.%s.title", sectionRule.Name),
				})
			}
			continue
		}

		addIssue(issues, entity, domainvalidation.Issue{
			Code:        "content.required_missing",
			Level:       domainvalidation.LevelError,
			Class:       "InstanceError",
			Message:     fmt.Sprintf("required section '%s' is missing", sectionRule.Name),
			StandardRef: "13.2",
			Field:       fmt.Sprintf("content.sections.%s", sectionRule.Name),
		})
	}
}

func evaluateRequiredWhen(
	issues *[]domainvalidation.Issue,
	entity *model.CheckedEntity,
	literal bool,
	expression *expressions.Expression,
	fieldPath string,
	label string,
	context runtimeExpressionContext,
) bool {
	if expression == nil {
		return literal
	}

	value, evalErr := expressions.Evaluate(expression, context)
	if evalErr == nil {
		return value
	}

	addIssue(issues, entity, domainvalidation.Issue{
		Code:        evalErr.Code,
		Level:       domainvalidation.LevelError,
		Class:       "InstanceError",
		Message:     fmt.Sprintf("failed to evaluate required_when for %s: %s", label, evalErr.Message),
		StandardRef: evalErr.StandardRef,
		Field:       fieldPath,
	})
	return false
}
