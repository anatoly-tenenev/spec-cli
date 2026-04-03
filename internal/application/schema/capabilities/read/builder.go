package read

import "github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"

type Capability struct {
	EntityTypes map[string]EntityReadModel
}

type EntityReadModel struct {
	MetaFields      map[string]model.MetaField
	Sections        map[string]model.Section
	ReferenceFields map[string]model.RefSpec
}

func Build(compiled model.CompiledSchema) Capability {
	result := Capability{EntityTypes: make(map[string]EntityReadModel, len(compiled.Entities))}
	for typeName, entity := range compiled.Entities {
		metaFields := make(map[string]model.MetaField, len(entity.MetaFields))
		references := make(map[string]model.RefSpec)
		for fieldName, field := range entity.MetaFields {
			metaFields[fieldName] = field
			if field.Value.Ref != nil {
				references[fieldName] = *field.Value.Ref
				continue
			}
			if field.Value.Kind == model.ValueKindArray && field.Value.Items != nil && field.Value.Items.Ref != nil {
				references[fieldName] = model.RefSpec{
					Cardinality:  model.RefCardinalityArray,
					AllowedTypes: append([]string(nil), field.Value.Items.Ref.AllowedTypes...),
				}
			}
		}

		sections := make(map[string]model.Section, len(entity.Sections))
		for sectionName, section := range entity.Sections {
			sections[sectionName] = section
		}

		result.EntityTypes[typeName] = EntityReadModel{
			MetaFields:      metaFields,
			Sections:        sections,
			ReferenceFields: references,
		}
	}
	return result
}
