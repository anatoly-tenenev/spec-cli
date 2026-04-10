package validate

import (
	"sort"

	schemaexpressions "github.com/anatoly-tenenev/spec-cli/internal/application/schema/expressions"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
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

type RuleValue struct {
	Literal  any
	Template *schemaexpressions.CompiledTemplate
}

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
	typeNames := sortedEntityNames(compiled)
	capability := Capability{
		EntityOrder: typeNames,
		EntityTypes: make(map[string]EntityValidationModel, len(typeNames)),
	}

	for _, typeName := range typeNames {
		entity := compiled.Entities[typeName]
		fieldNames := sortedMetaFieldNames(entity.MetaFields)
		sectionNames := sortedSectionNames(entity.Sections)

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
		Type:         kindToRuleType(field.Value.Kind),
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
		rule.Value = RuleValue{Literal: field.Value.Const.Value, Template: field.Value.Const.Template}
	}
	if len(field.Value.Enum) > 0 {
		rule.Enum = make([]RuleValue, 0, len(field.Value.Enum))
		for _, enumValue := range field.Value.Enum {
			rule.Enum = append(rule.Enum, RuleValue{Literal: enumValue.Value, Template: enumValue.Template})
		}
	}

	if field.Value.Items != nil {
		rule.HasItemType = true
		rule.ItemType = kindToRuleType(field.Value.Items.Kind)
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

func kindToRuleType(kind model.ValueKind) string {
	switch kind {
	case model.ValueKindString:
		return "string"
	case model.ValueKindNumber:
		return "number"
	case model.ValueKindInteger:
		return "integer"
	case model.ValueKindBoolean:
		return "boolean"
	case model.ValueKindArray:
		return "array"
	case model.ValueKindEntityRef:
		return "entityRef"
	default:
		return "unknown"
	}
}

func sortedEntityNames(compiled model.CompiledSchema) []string {
	names := make([]string, 0, len(compiled.Entities))
	for typeName := range compiled.Entities {
		names = append(names, typeName)
	}
	sort.Strings(names)
	return names
}

func sortedMetaFieldNames(fields map[string]model.MetaField) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedSectionNames(sections map[string]model.Section) []string {
	names := make([]string, 0, len(sections))
	for name := range sections {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
