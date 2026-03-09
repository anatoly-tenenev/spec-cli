package engine

import (
	"fmt"

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

		if rule.Type != "any" && !support.MatchesRuleType(value, rule.Type) {
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

		if len(rule.Enum) > 0 && !support.ContainsEnumValue(rule.Enum, value) {
			addIssue(issues, entity, domainvalidation.Issue{
				Code:        "meta.required_enum_mismatch",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     fmt.Sprintf("metadata field '%s' is not in enum", rule.Name),
				StandardRef: "12.3",
				Field:       fmt.Sprintf("frontmatter.%s", rule.Name),
			})
		}

		if rule.HasValue && !support.LiteralEqual(value, rule.Value) {
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
