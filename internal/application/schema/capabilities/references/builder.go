package references

import (
	"sort"

	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
)

type Capability struct {
	InboundByTargetType map[string][]InboundSlot
}

type InboundSlot struct {
	SourceType  string
	FieldName   string
	Cardinality model.RefCardinality
}

func Build(compiled model.CompiledSchema) Capability {
	result := Capability{InboundByTargetType: map[string][]InboundSlot{}}
	allTypes := sortedTypeNames(compiled)

	for _, sourceType := range allTypes {
		entity := compiled.Entities[sourceType]
		for fieldName, field := range entity.MetaFields {
			cardinality, allowedTypes, isRef := extractRef(field, allTypes)
			if !isRef {
				continue
			}
			for _, targetType := range allowedTypes {
				result.InboundByTargetType[targetType] = append(result.InboundByTargetType[targetType], InboundSlot{
					SourceType:  sourceType,
					FieldName:   fieldName,
					Cardinality: cardinality,
				})
			}
		}
	}

	for targetType, slots := range result.InboundByTargetType {
		sort.SliceStable(slots, func(i int, j int) bool {
			if slots[i].SourceType == slots[j].SourceType {
				return slots[i].FieldName < slots[j].FieldName
			}
			return slots[i].SourceType < slots[j].SourceType
		})
		result.InboundByTargetType[targetType] = slots
	}

	return result
}

func extractRef(field model.MetaField, allTypes []string) (model.RefCardinality, []string, bool) {
	if field.Value.Ref != nil {
		allowed := normalizedAllowedTypes(field.Value.Ref.AllowedTypes, allTypes)
		return model.RefCardinalityScalar, allowed, true
	}
	if field.Value.Kind == model.ValueKindArray && field.Value.Items != nil && field.Value.Items.Ref != nil {
		allowed := normalizedAllowedTypes(field.Value.Items.Ref.AllowedTypes, allTypes)
		return model.RefCardinalityArray, allowed, true
	}
	return "", nil, false
}

func normalizedAllowedTypes(types []string, allTypes []string) []string {
	if len(types) == 0 {
		return append([]string(nil), allTypes...)
	}
	result := append([]string(nil), types...)
	sort.Strings(result)
	return result
}

func sortedTypeNames(compiled model.CompiledSchema) []string {
	names := make([]string, 0, len(compiled.Entities))
	for typeName := range compiled.Entities {
		names = append(names, typeName)
	}
	sort.Strings(names)
	return names
}
