package sections

import (
	"fmt"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/expressions"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	names "github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/schema/internal/entity/internal/names"
	requiredconstraint "github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/schema/internal/entity/internal/requiredconstraint"
	schemachecks "github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/schema/internal/entity/internal/schemachecks"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

func Parse(
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

	rawByName, ok := support.ToStringMap(rawSections)
	if !ok || len(rawByName) == 0 {
		return nil, nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.content.sections must be a non-empty mapping", typeName),
			nil,
		)
	}

	sectionNames := support.SortedMapKeys(rawByName)
	rules := make([]model.RequiredSectionRule, 0, len(sectionNames))
	rawRules := make([]map[string]any, 0, len(sectionNames))
	issues := make([]domainvalidation.Issue, 0)

	for _, sectionName := range sectionNames {
		if keyErr := names.ValidateSectionName(typeName, sectionName); keyErr != nil {
			return nil, nil, keyErr
		}

		sectionPath := fmt.Sprintf("schema.entity.%s.content.sections.%s", typeName, sectionName)
		rawRule, ok := support.ToStringMap(rawByName[sectionName])
		if !ok {
			return nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s must be an object", sectionPath),
				nil,
			)
		}
		if keyErr := schemachecks.EnsureOnlyKeys(sectionPath, rawRule, "required", "required_when", "title", "description"); keyErr != nil {
			return nil, nil, keyErr
		}

		rule := model.RequiredSectionRule{
			Name:             sectionName,
			RequiredWhenPath: sectionPath + ".required_when",
		}
		if rawTitle, hasTitle := rawRule["title"]; hasTitle {
			title, ok := rawTitle.(string)
			if !ok || strings.TrimSpace(title) == "" {
				return nil, nil, domainerrors.New(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("%s.title must be non-empty string", sectionPath),
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
		sectionPath := fmt.Sprintf("schema.entity.%s.content.sections.%s", typeName, rules[idx].Name)
		required, requiredWhenLiteral, requiredWhenExpr, requiredIssues, requiredErr := requiredconstraint.Parse(
			rawRules[idx],
			sectionPath,
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
	for _, rule := range rules {
		if usage, hasUsage := schemachecks.StrictMissingUsageInRequiredWhen(rule.RequiredWhenExpr, fieldsByName); hasUsage {
			message := fmt.Sprintf(
				"schema.entity.%s.content.sections.%s.required_when uses strict operator '%s' with potentially missing operand '%s'",
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
