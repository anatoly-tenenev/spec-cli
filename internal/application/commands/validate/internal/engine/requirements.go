package engine

import (
	"fmt"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/support"
	schemacapvalidate "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/validate"
	schemaexpressions "github.com/anatoly-tenenev/spec-cli/internal/application/schema/expressions"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

func validateRequiredFields(
	issues *[]domainvalidation.Issue,
	entity *model.CheckedEntity,
	frontmatter map[string]any,
	typeSpec schemacapvalidate.EntityValidationModel,
	idIndex map[string][]resolvedEntityRef,
	context map[string]any,
) {
	for _, rule := range typeSpec.RequiredFields {
		required, requiredErr := evaluateRequiredConstraint(rule.Required, rule.RequiredExpr, context)
		if requiredErr != nil {
			addIssue(issues, entity, domainvalidation.Issue{
				Code:        "meta.required_expression_evaluation_failed",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     fmt.Sprintf("failed to evaluate required for metadata field '%s': %s", rule.Name, requiredErr.Message),
				StandardRef: "11.6",
				Field:       rule.RequiredPath,
			})
			required = false
		}

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
			validateArrayField(issues, entity, rule, arrayValue, idIndex)
		}

		resolvedEnum, enumResolveErr := resolveRuleValues(rule.Enum, context)
		if enumResolveErr != nil {
			addIssue(issues, entity, domainvalidation.Issue{
				Code:        "meta.required_enum_interpolation_failed",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     fmt.Sprintf("metadata field '%s' enum interpolation failed: %s", rule.Name, enumResolveErr.Message),
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

		if rule.HasValue {
			resolvedConst, constResolveErr := resolveRuleValue(rule.Value, context)
			if constResolveErr != nil {
				addIssue(issues, entity, domainvalidation.Issue{
					Code:        "meta.required_const_interpolation_failed",
					Level:       domainvalidation.LevelError,
					Class:       "InstanceError",
					Message:     fmt.Sprintf("metadata field '%s' const interpolation failed: %s", rule.Name, constResolveErr.Message),
					StandardRef: "9.4",
					Field:       fmt.Sprintf("frontmatter.%s", rule.Name),
				})
				continue
			}

			if !support.LiteralEqual(value, resolvedConst) {
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
}

func validateArrayField(
	issues *[]domainvalidation.Issue,
	entity *model.CheckedEntity,
	rule schemacapvalidate.RequiredFieldRule,
	arrayValue []any,
	idIndex map[string][]resolvedEntityRef,
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
			matchesItemType := support.MatchesRuleType(item, rule.ItemType)
			if matchesItemType && rule.ItemType == "entityRef" {
				referenceID, _ := item.(string)
				if strings.TrimSpace(referenceID) == "" {
					matchesItemType = false
				}
			}

			if matchesItemType {
				if rule.ItemType == "entityRef" {
					referenceID, _ := item.(string)
					referenceID = strings.TrimSpace(referenceID)
					if referenceID != "" {
						indexedField := fmt.Sprintf("%s[%d]", fieldPath, idx)
						resolution := resolveEntityReferenceValue(
							fmt.Sprintf("%s[%d]", rule.Name, idx),
							indexedField,
							referenceID,
							rule.ItemRefTypes,
							idIndex,
						)
						if resolution.Issue != nil {
							addIssue(issues, entity, *resolution.Issue)
						}
					}
				}
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

func validateRequiredSections(
	issues *[]domainvalidation.Issue,
	entity *model.CheckedEntity,
	sections map[string]string,
	duplicateLabels []string,
	typeSpec schemacapvalidate.EntityValidationModel,
	context map[string]any,
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
		required, requiredErr := evaluateRequiredConstraint(sectionRule.Required, sectionRule.RequiredExpr, context)
		if requiredErr != nil {
			addIssue(issues, entity, domainvalidation.Issue{
				Code:        "content.required_expression_evaluation_failed",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     fmt.Sprintf("failed to evaluate required for content section '%s': %s", sectionRule.Name, requiredErr.Message),
				StandardRef: "11.6",
				Field:       sectionRule.RequiredPath,
			})
			required = false
		}

		if !required {
			continue
		}

		if title, exists := sections[sectionRule.Name]; exists {
			if len(sectionRule.Titles) > 0 && !containsSectionTitle(sectionRule.Titles, title) {
				addIssue(issues, entity, domainvalidation.Issue{
					Code:        "content.section_title_mismatch",
					Level:       domainvalidation.LevelError,
					Class:       "InstanceError",
					Message:     fmt.Sprintf("section '%s' title must be one of %v", sectionRule.Name, sectionRule.Titles),
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

func evaluateRequiredConstraint(
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

func resolveRuleValues(values []schemacapvalidate.RuleValue, context map[string]any) ([]any, *schemaexpressions.EvalError) {
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

func resolveRuleValue(value schemacapvalidate.RuleValue, context map[string]any) (any, *schemaexpressions.EvalError) {
	if value.Template == nil {
		return value.Literal, nil
	}

	rendered, renderErr := schemaexpressions.RenderTemplate(value.Template, context)
	if renderErr != nil {
		return nil, renderErr
	}

	return rendered, nil
}

func containsSectionTitle(allowed []string, title string) bool {
	for _, candidate := range allowed {
		if candidate == title {
			return true
		}
	}
	return false
}
