// Package validate projects the compiled schema into the rule set validate
// checks documents against: the frontmatter fields an entity type allows, the
// requirements each field and section carries, and the path pattern the file
// is expected to sit at. Conditional requirements stay as compiled
// expressions, because they are only decidable against a concrete entity.
package validate

import (
	"github.com/anatoly-tenenev/spec-cli/internal/application/collections"
	schemaexpressions "github.com/anatoly-tenenev/spec-cli/internal/application/schema/expressions"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/rulevalues"
)

type Capability struct {
	EntityOrder []string
	EntityTypes map[string]EntityValidationModel
}

type EntityValidationModel struct {
	Name                     string
	IDPrefix                 string
	AllowedFrontmatterFields []string
	RequiredFields           []RequiredFieldRule
	RequiredSections         []RequiredSectionRule
	PathPattern              PathPatternRule
}

type RequiredFieldRule struct {
	Name         string
	Type         string
	RefTypes     []string
	Enum         []RuleValue
	HasValue     bool
	Value        RuleValue
	HasItemType  bool
	ItemType     string
	ItemRefTypes []string
	UniqueItems  bool
	HasMinItems  bool
	MinItems     int
	HasMaxItems  bool
	MaxItems     int
	Required     bool
	RequiredExpr *schemaexpressions.CompiledExpression
	RequiredPath string
}

type RequiredSectionRule struct {
	Name         string
	Title        string
	Required     bool
	RequiredExpr *schemaexpressions.CompiledExpression
	RequiredPath string
	TitlePath    string
}

// RuleValue is the shared literal-or-template, aliased so that a const or
// enum validate judges is the same value the write side offers.
type RuleValue = rulevalues.RuleValue

type PathPatternRule struct {
	Cases []PathPatternCase
}

type PathPatternCase struct {
	Use         string
	UseTemplate *schemaexpressions.CompiledTemplate
	HasWhen     bool
	When        bool
	WhenExpr    *schemaexpressions.CompiledExpression
	WhenPath    string
}

func Build(compiled model.CompiledSchema) Capability {
	typeNames := collections.SortedMapKeys(compiled.Entities)
	capability := Capability{
		EntityOrder: typeNames,
		EntityTypes: make(map[string]EntityValidationModel, len(typeNames)),
	}

	for _, typeName := range typeNames {
		entity := compiled.Entities[typeName]
		fieldNames := collections.SortedMapKeys(entity.MetaFields)
		sectionNames := collections.SortedMapKeys(entity.Sections)

		requiredFields := make([]RequiredFieldRule, 0, len(fieldNames))
		for _, fieldName := range fieldNames {
			requiredFields = append(requiredFields, buildFieldRule(entity.MetaFields[fieldName]))
		}

		requiredSections := make([]RequiredSectionRule, 0, len(sectionNames))
		for _, sectionName := range sectionNames {
			requiredSections = append(requiredSections, buildSectionRule(entity.Sections[sectionName]))
		}

		pathCases := make([]PathPatternCase, 0, len(entity.PathTemplate.Cases))
		for _, pathCase := range entity.PathTemplate.Cases {
			pathCases = append(pathCases, buildPathCase(pathCase))
		}

		allowedFields := make([]string, 0, 5+len(fieldNames))
		allowedFields = append(allowedFields, "type", "id", "slug", "createdDate", "updatedDate")
		allowedFields = append(allowedFields, fieldNames...)

		capability.EntityTypes[typeName] = EntityValidationModel{
			Name:                     typeName,
			IDPrefix:                 entity.IDPrefix,
			AllowedFrontmatterFields: allowedFields,
			RequiredFields:           requiredFields,
			RequiredSections:         requiredSections,
			PathPattern:              PathPatternRule{Cases: pathCases},
		}
	}

	return capability
}

func buildFieldRule(field model.MetaField) RequiredFieldRule {
	rule := RequiredFieldRule{
		Name:         field.Name,
		Type:         field.Value.Kind.TypeName(),
		Required:     field.Required.Always,
		RequiredExpr: field.Required.Expr,
		RequiredPath: field.Required.Path,
		UniqueItems:  field.Value.UniqueItems,
	}

	if field.Value.Ref != nil {
		rule.RefTypes = append([]string(nil), field.Value.Ref.AllowedTypes...)
	}

	if field.Value.Const != nil {
		rule.HasValue = true
		rule.Value = rulevalues.FromLiteral(*field.Value.Const)
	}
	rule.Enum = rulevalues.FromLiterals(field.Value.Enum)

	if field.Value.Items != nil {
		rule.HasItemType = true
		rule.ItemType = field.Value.Items.Kind.TypeName()
		if field.Value.Items.Ref != nil {
			rule.ItemRefTypes = append([]string(nil), field.Value.Items.Ref.AllowedTypes...)
		}
	}
	if field.Value.MinItems != nil {
		rule.HasMinItems = true
		rule.MinItems = *field.Value.MinItems
	}
	if field.Value.MaxItems != nil {
		rule.HasMaxItems = true
		rule.MaxItems = *field.Value.MaxItems
	}

	return rule
}

func buildSectionRule(section model.Section) RequiredSectionRule {
	return RequiredSectionRule{
		Name:         section.Name,
		Title:        section.Title,
		Required:     section.Required.Always,
		RequiredExpr: section.Required.Expr,
		RequiredPath: section.Required.Path,
		TitlePath:    section.TitlePath,
	}
}

func buildPathCase(pathCase model.PathTemplateCase) PathPatternCase {
	rule := PathPatternCase{
		Use:         pathCase.Use,
		UseTemplate: pathCase.UseTemplate,
		WhenPath:    pathCase.When.Path,
	}

	switch {
	case pathCase.When.Expr != nil:
		rule.HasWhen = true
		rule.WhenExpr = pathCase.When.Expr
	case pathCase.When.Always:
		rule.HasWhen = false
	default:
		rule.HasWhen = true
		rule.When = false
	}

	return rule
}
