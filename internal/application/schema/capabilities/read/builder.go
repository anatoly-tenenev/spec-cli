package read

import (
	"sort"

	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/derivedschema"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
)

type Capability struct {
	EntityTypes map[string]EntityReadModel
}

type EntityReadModel struct {
	MetaFields map[string]MetaField
	RefFields  map[string]RefField
	Sections   map[string]Section
}

type FieldKind string

const (
	FieldKindUnknown FieldKind = "unknown"
	FieldKindString  FieldKind = "string"
	FieldKindDate    FieldKind = "date"
	FieldKindNumber  FieldKind = "number"
	FieldKindBoolean FieldKind = "boolean"
	FieldKindArray   FieldKind = "array"
)

type MetaField struct {
	Kind       FieldKind
	ItemKind   FieldKind
	EnumValues []any
	HasConst   bool
	ConstValue any
	Required   bool
}

type RefCardinality string

const (
	RefCardinalityScalar RefCardinality = "scalar"
	RefCardinalityArray  RefCardinality = "array"
)

type RefField struct {
	Cardinality  RefCardinality
	AllowedTypes []string
}

type Section struct {
	Required bool
}

func Build(compiled model.CompiledSchema) Capability {
	typeNames := sortedNames(compiled.Entities)
	capability := Capability{
		EntityTypes: make(map[string]EntityReadModel, len(typeNames)),
	}

	for _, typeName := range typeNames {
		entity := compiled.Entities[typeName]
		metaFields := make(map[string]MetaField, len(entity.MetaFields))
		refFields := make(map[string]RefField)
		sections := make(map[string]Section, len(entity.Sections))

		for _, fieldName := range sortedNames(entity.MetaFields) {
			field := entity.MetaFields[fieldName]
			refField, isRef := buildRefField(field, typeNames)
			if isRef {
				refFields[fieldName] = refField
				continue
			}
			enumValues, _ := derivedschema.StaticEnumValues(field.Value.Enum)
			constValue, hasConst := derivedschema.StaticConstValue(field.Value.Const)
			metaFields[fieldName] = MetaField{
				Kind:       mapFieldKind(field.Value),
				ItemKind:   mapArrayItemKind(field.Value),
				EnumValues: enumValues,
				HasConst:   hasConst,
				ConstValue: constValue,
				Required:   field.Required.Always,
			}
		}

		for _, sectionName := range sortedNames(entity.Sections) {
			sections[sectionName] = Section{
				Required: entity.Sections[sectionName].Required.Always,
			}
		}

		capability.EntityTypes[typeName] = EntityReadModel{
			MetaFields: metaFields,
			RefFields:  refFields,
			Sections:   sections,
		}
	}

	return capability
}

func buildRefField(field model.MetaField, allTypes []string) (RefField, bool) {
	if field.Value.Ref != nil {
		return RefField{
			Cardinality:  RefCardinalityScalar,
			AllowedTypes: normalizedAllowedTypes(field.Value.Ref.AllowedTypes, allTypes),
		}, true
	}
	if field.Value.Kind == model.ValueKindArray && field.Value.Items != nil && field.Value.Items.Ref != nil {
		return RefField{
			Cardinality:  RefCardinalityArray,
			AllowedTypes: normalizedAllowedTypes(field.Value.Items.Ref.AllowedTypes, allTypes),
		}, true
	}
	return RefField{}, false
}

func mapFieldKind(value model.ValueSpec) FieldKind {
	switch value.Kind {
	case model.ValueKindString, model.ValueKindEntityRef:
		if value.Format == "date" {
			return FieldKindDate
		}
		return FieldKindString
	case model.ValueKindNumber, model.ValueKindInteger:
		return FieldKindNumber
	case model.ValueKindBoolean:
		return FieldKindBoolean
	case model.ValueKindArray:
		return FieldKindArray
	default:
		return FieldKindUnknown
	}
}

func mapArrayItemKind(value model.ValueSpec) FieldKind {
	if value.Kind != model.ValueKindArray || value.Items == nil {
		return ""
	}

	switch value.Items.Kind {
	case model.ValueKindString, model.ValueKindEntityRef:
		if value.Items.Format == "date" {
			return FieldKindDate
		}
		return FieldKindString
	case model.ValueKindNumber, model.ValueKindInteger:
		return FieldKindNumber
	case model.ValueKindBoolean:
		return FieldKindBoolean
	case model.ValueKindArray:
		return FieldKindArray
	default:
		return FieldKindUnknown
	}
}

func normalizedAllowedTypes(types []string, allTypes []string) []string {
	if len(types) == 0 {
		return append([]string(nil), allTypes...)
	}
	result := append([]string(nil), types...)
	sort.Strings(result)
	return dedupeSorted(result)
}

func dedupeSorted(values []string) []string {
	if len(values) == 0 {
		return values
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}

func sortedNames[T any](values map[string]T) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
