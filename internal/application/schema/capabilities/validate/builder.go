package validate

import "github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"

type Capability struct {
	EntityTypes map[string]EntityValidationModel
}

type EntityValidationModel struct {
	IDPrefix         string
	RequiredFields   map[string]model.MetaField
	RequiredSections map[string]model.Section
	PathTemplate     model.PathTemplate
}

func Build(compiled model.CompiledSchema) Capability {
	result := Capability{EntityTypes: make(map[string]EntityValidationModel, len(compiled.Entities))}

	for typeName, entity := range compiled.Entities {
		requiredFields := make(map[string]model.MetaField)
		for fieldName, field := range entity.MetaFields {
			if field.Required.Always || field.Required.Expr != nil {
				requiredFields[fieldName] = field
			}
		}

		requiredSections := make(map[string]model.Section)
		for sectionName, section := range entity.Sections {
			if section.Required.Always || section.Required.Expr != nil {
				requiredSections[sectionName] = section
			}
		}

		result.EntityTypes[typeName] = EntityValidationModel{
			IDPrefix:         entity.IDPrefix,
			RequiredFields:   requiredFields,
			RequiredSections: requiredSections,
			PathTemplate:     entity.PathTemplate,
		}
	}

	return result
}
