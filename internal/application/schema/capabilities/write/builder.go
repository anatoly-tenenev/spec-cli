package write

import (
	"sort"

	schemaexpressions "github.com/anatoly-tenenev/spec-cli/internal/application/schema/expressions"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
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

type RuleValue struct {
	Literal  any
	Template *schemaexpressions.CompiledTemplate
}

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
	typeNames := sortedEntityNames(compiled)
	capability := Capability{EntityTypes: make(map[string]EntityWriteModel, len(typeNames))}

	for _, typeName := range typeNames {
		entity := compiled.Entities[typeName]

		metaOrder := filteredOrder(entity.MetaFieldOrder, sortedMetaFieldNames(entity.MetaFields), entity.MetaFields)
		sectionOrder := filteredOrder(entity.SectionOrder, sortedSectionNames(entity.Sections), entity.Sections)

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
		Type:         kindToTypeName(field.Value.Kind),
		Format:       field.Value.Format,
		Required:     field.Required.Always,
		RequiredExpr: field.Required.Expr,
		RequiredPath: field.Required.Path,
		UniqueItems:  field.Value.UniqueItems,
	}

	if field.Value.Const != nil {
		result.HasConst = true
		result.Const = RuleValue{
			Literal:  field.Value.Const.Value,
			Template: field.Value.Const.Template,
		}
	}

	if len(field.Value.Enum) > 0 {
		result.Enum = make([]RuleValue, 0, len(field.Value.Enum))
		for _, enumValue := range field.Value.Enum {
			result.Enum = append(result.Enum, RuleValue{
				Literal:  enumValue.Value,
				Template: enumValue.Template,
			})
		}
	}

	if field.Value.Ref != nil {
		result.IsEntityRef = field.Value.Ref.Cardinality == model.RefCardinalityScalar
		result.RefTypes = append([]string(nil), field.Value.Ref.AllowedTypes...)
	}

	if field.Value.Kind == model.ValueKindArray && field.Value.Items != nil {
		result.HasItems = true
		result.ItemType = kindToTypeName(field.Value.Items.Kind)
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

func kindToTypeName(kind model.ValueKind) string {
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
	for fieldName := range fields {
		names = append(names, fieldName)
	}
	sort.Strings(names)
	return names
}

func sortedSectionNames(sections map[string]model.Section) []string {
	names := make([]string, 0, len(sections))
	for sectionName := range sections {
		names = append(names, sectionName)
	}
	sort.Strings(names)
	return names
}

func filteredOrder[T any](preferred []string, fallback []string, values map[string]T) []string {
	if len(preferred) == 0 {
		return append([]string(nil), fallback...)
	}

	order := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range preferred {
		if _, exists := values[value]; !exists {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		order = append(order, value)
	}
	for _, value := range fallback {
		if _, exists := seen[value]; exists {
			continue
		}
		order = append(order, value)
	}
	return order
}
