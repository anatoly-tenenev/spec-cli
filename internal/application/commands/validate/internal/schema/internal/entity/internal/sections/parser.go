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
)

func Parse(
	typeName string,
	rawContent any,
	engine *expressions.Engine,
) ([]model.RequiredSectionRule, *domainerrors.AppError) {
	if rawContent == nil {
		return nil, nil
	}

	contentMap, ok := support.ToStringMap(rawContent)
	if !ok {
		return nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.content must be a mapping", typeName),
			nil,
		)
	}
	if keyErr := schemachecks.EnsureOnlyKeys(fmt.Sprintf("schema.entity.%s.content", typeName), contentMap, "sections"); keyErr != nil {
		return nil, keyErr
	}

	rawSections, exists := contentMap["sections"]
	if !exists || rawSections == nil {
		return nil, nil
	}

	rawByName, ok := support.ToStringMap(rawSections)
	if !ok || len(rawByName) == 0 {
		return nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.content.sections must be a non-empty mapping", typeName),
			nil,
		)
	}

	sectionNames := support.SortedMapKeys(rawByName)
	rules := make([]model.RequiredSectionRule, 0, len(sectionNames))

	for _, sectionName := range sectionNames {
		if keyErr := names.ValidateSectionName(typeName, sectionName); keyErr != nil {
			return nil, keyErr
		}

		sectionPath := fmt.Sprintf("schema.entity.%s.content.sections.%s", typeName, sectionName)
		rawRule, ok := support.ToStringMap(rawByName[sectionName])
		if !ok {
			return nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s must be an object", sectionPath),
				nil,
			)
		}
		if keyErr := schemachecks.EnsureOnlyKeys(sectionPath, rawRule, "required", "title", "description"); keyErr != nil {
			return nil, keyErr
		}

		rule := model.RequiredSectionRule{
			Name:         sectionName,
			RequiredPath: sectionPath + ".required",
		}
		if rawTitle, hasTitle := rawRule["title"]; hasTitle {
			title, ok := rawTitle.(string)
			if !ok || strings.TrimSpace(title) == "" {
				return nil, domainerrors.New(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("%s.title must be non-empty string", sectionPath),
					nil,
				)
			}
			if expressions.ContainsInterpolation(title) {
				return nil, domainerrors.New(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("%s.title does not allow interpolation ${...}", sectionPath),
					nil,
				)
			}
			rule.HasTitle = true
			rule.Title = strings.TrimSpace(title)
		}

		if rawDescription, hasDescription := rawRule["description"]; hasDescription {
			description, ok := rawDescription.(string)
			if !ok || strings.TrimSpace(description) == "" {
				return nil, domainerrors.New(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("%s.description must be non-empty string", sectionPath),
					nil,
				)
			}
			if expressions.ContainsInterpolation(description) {
				return nil, domainerrors.New(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("%s.description does not allow interpolation ${...}", sectionPath),
					nil,
				)
			}
		}

		required, requiredExpr, requiredErr := requiredconstraint.Parse(rawRule, sectionPath, engine)
		if requiredErr != nil {
			return nil, requiredErr
		}
		rule.Required = required
		rule.RequiredExpr = requiredExpr

		rules = append(rules, rule)
	}

	return rules, nil
}
