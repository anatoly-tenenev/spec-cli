// Package write projects the compiled schema into what add and update are
// allowed to do: the writable paths of an entity type, split by whether they
// address a meta field, a reference or a section, plus the constraints each
// field carries. Refusing an unknown --set path is a lookup here, so no write
// command decides on its own what may be written.
package write

import (
	"sort"

	"github.com/anatoly-tenenev/spec-cli/internal/application/collections"
	schemaexpressions "github.com/anatoly-tenenev/spec-cli/internal/application/schema/expressions"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/rulevalues"
)

type Capability struct {
	EntityTypes map[string]EntityWriteModel
}

type EntityWriteModel struct {
	Name              string
	IDPrefix          string
	PathPattern       PathPattern
	MetaFields        map[string]MetaField
	MetaFieldOrder    []string
	Sections          map[string]SectionSpec
	SectionOrder      []string
	HasContent        bool
	AllowWritePaths   map[string]WritePathSpec
	AllowSetFilePaths map[string]struct{}
	SetPaths          []string
	UnsetPaths        []string
	SetFilePaths      []string
}

type WritePathKind string

const (
	WritePathMeta    WritePathKind = "meta"
	WritePathRef     WritePathKind = "ref"
	WritePathSection WritePathKind = "section"
)

type WritePathSpec struct {
	Kind      WritePathKind
	FieldName string
}

type MetaField struct {
	Name             string
	Type             string
	Format           string
	Required         bool
	RequiredExpr     *schemaexpressions.CompiledExpression
	RequiredPath     string
	Enum             []RuleValue
	HasConst         bool
	Const            RuleValue
	IsEntityRef      bool
	IsEntityRefArray bool
	RefTypes         []string
	HasItems         bool
	ItemType         string
	ItemRefTypes     []string
	UniqueItems      bool
	HasMinItems      bool
	MinItems         int
	HasMaxItems      bool
	MaxItems         int
}

type SectionSpec struct {
	Name         string
	Title        string
	Required     bool
	RequiredExpr *schemaexpressions.CompiledExpression
	RequiredPath string
}

// RuleValue is the shared literal-or-template, aliased so that a const or
// enum the write side offers is the same value the validate side judges.
type RuleValue = rulevalues.RuleValue

type PathPattern struct {
	Cases []PathPatternCase
}

type PathPatternCase struct {
	Use         string
	UseTemplate *schemaexpressions.CompiledTemplate
	HasWhen     bool
	When        bool
	WhenExpr    *schemaexpressions.CompiledExpression
	WhenPath    string
	UsePath     string
}

func Build(compiled model.CompiledSchema) Capability {
	typeNames := collections.SortedMapKeys(compiled.Entities)
	capability := Capability{EntityTypes: make(map[string]EntityWriteModel, len(typeNames))}

	for _, typeName := range typeNames {
		entity := compiled.Entities[typeName]

		metaOrder := collections.OrderedKeys(entity.MetaFieldOrder, entity.MetaFields)
		sectionOrder := collections.OrderedKeys(entity.SectionOrder, entity.Sections)

		metaFields := make(map[string]MetaField, len(entity.MetaFields))
		for fieldName, field := range entity.MetaFields {
			metaFields[fieldName] = buildMetaField(field)
		}

		sections := make(map[string]SectionSpec, len(entity.Sections))
		for sectionName, section := range entity.Sections {
			sections[sectionName] = buildSection(section)
		}

		pathCases := make([]PathPatternCase, 0, len(entity.PathTemplate.Cases))
		for _, pathCase := range entity.PathTemplate.Cases {
			pathCases = append(pathCases, buildPathCase(pathCase))
		}

		allowWritePaths := map[string]WritePathSpec{}
		allowSetFilePaths := map[string]struct{}{}
		setPaths := make([]string, 0, len(metaOrder)+len(sectionOrder))
		unsetPaths := make([]string, 0, len(metaOrder)+len(sectionOrder))
		setFilePaths := make([]string, 0, len(sectionOrder))

		for _, fieldName := range metaOrder {
			field := metaFields[fieldName]

			path := "meta." + fieldName
			pathKind := WritePathMeta
			if field.IsEntityRef || field.IsEntityRefArray {
				path = "refs." + fieldName
				pathKind = WritePathRef
			}
			allowWritePaths[path] = WritePathSpec{Kind: pathKind, FieldName: fieldName}
			setPaths = append(setPaths, path)
			unsetPaths = append(unsetPaths, path)
		}

		for _, sectionName := range sectionOrder {
			path := "content.sections." + sectionName
			allowWritePaths[path] = WritePathSpec{Kind: WritePathSection, FieldName: sectionName}
			allowSetFilePaths[path] = struct{}{}
			setPaths = append(setPaths, path)
			unsetPaths = append(unsetPaths, path)
			setFilePaths = append(setFilePaths, path)
		}

		capability.EntityTypes[typeName] = EntityWriteModel{
			Name:              typeName,
			IDPrefix:          entity.IDPrefix,
			PathPattern:       PathPattern{Cases: pathCases},
			MetaFields:        metaFields,
			MetaFieldOrder:    append([]string(nil), metaOrder...),
			Sections:          sections,
			SectionOrder:      append([]string(nil), sectionOrder...),
			HasContent:        entity.HasContent,
			AllowWritePaths:   allowWritePaths,
			AllowSetFilePaths: allowSetFilePaths,
			SetPaths:          dedupeSorted(setPaths),
			UnsetPaths:        dedupeSorted(unsetPaths),
			SetFilePaths:      dedupeSorted(setFilePaths),
		}
	}

	return capability
}

func dedupeSorted(values []string) []string {
	if len(values) == 0 {
		return values
	}
	sort.Strings(values)
	result := make([]string, 0, len(values))
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}

func buildMetaField(field model.MetaField) MetaField {
	result := MetaField{
		Name:         field.Name,
		Type:         field.Value.Kind.TypeName(),
		Format:       field.Value.Format,
		Required:     field.Required.Always,
		RequiredExpr: field.Required.Expr,
		RequiredPath: field.Required.Path,
		UniqueItems:  field.Value.UniqueItems,
	}

	if field.Value.Const != nil {
		result.HasConst = true
		result.Const = rulevalues.FromLiteral(*field.Value.Const)
	}

	result.Enum = rulevalues.FromLiterals(field.Value.Enum)

	if field.Value.Ref != nil {
		result.IsEntityRef = field.Value.Ref.Cardinality == model.RefCardinalityScalar
		result.RefTypes = append([]string(nil), field.Value.Ref.AllowedTypes...)
	}

	if field.Value.Kind == model.ValueKindArray && field.Value.Items != nil {
		result.HasItems = true
		result.ItemType = field.Value.Items.Kind.TypeName()
		if field.Value.Items.Ref != nil {
			result.IsEntityRefArray = true
			result.ItemRefTypes = append([]string(nil), field.Value.Items.Ref.AllowedTypes...)
		}
	}

	if field.Value.MinItems != nil {
		result.HasMinItems = true
		result.MinItems = *field.Value.MinItems
	}
	if field.Value.MaxItems != nil {
		result.HasMaxItems = true
		result.MaxItems = *field.Value.MaxItems
	}

	return result
}

func buildSection(section model.Section) SectionSpec {
	return SectionSpec{
		Name:         section.Name,
		Title:        section.Title,
		Required:     section.Required.Always,
		RequiredExpr: section.Required.Expr,
		RequiredPath: section.Required.Path,
	}
}

func buildPathCase(pathCase model.PathTemplateCase) PathPatternCase {
	result := PathPatternCase{
		Use:         pathCase.Use,
		UseTemplate: pathCase.UseTemplate,
		WhenPath:    pathCase.When.Path,
		UsePath:     pathCase.UsePath,
	}

	switch {
	case pathCase.When.Expr != nil:
		result.HasWhen = true
		result.WhenExpr = pathCase.When.Expr
	case pathCase.When.Always:
		result.HasWhen = false
	default:
		result.HasWhen = true
		result.When = false
	}

	return result
}
