package schema

import (
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func BuildIndex(loaded LoadedSchema) (model.QuerySchemaIndex, *domainerrors.AppError) {
	index := model.QuerySchemaIndex{
		EntityTypes: make(map[string]model.EntityTypeSpec, len(loaded.EntityTypes)),
	}

	for _, entityTypeName := range support.SortedMapKeys(loaded.EntityTypes) {
		entityType := loaded.EntityTypes[entityTypeName]
		entityTypeSpec := model.EntityTypeSpec{
			Name:          entityTypeName,
			MetaFields:    map[string]model.MetadataFieldSpec{},
			RefFields:     map[string]model.RefFieldSpec{},
			SectionFields: map[string]model.SectionFieldSpec{},
		}

		for _, metadataFieldName := range support.SortedMapKeys(entityType.MetadataFields) {
			metadataField := entityType.MetadataFields[metadataFieldName]
			if metadataField.IsEntityRef {
				cardinality := model.RefCardinalityScalar
				if metadataField.IsArrayRef {
					cardinality = model.RefCardinalityArray
				}
				entityTypeSpec.RefFields[metadataFieldName] = model.RefFieldSpec{
					Name:        metadataFieldName,
					Cardinality: cardinality,
					RefTypes:    append([]string(nil), metadataField.RefTypes...),
				}
				continue
			}
			entityTypeSpec.MetaFields[metadataFieldName] = model.MetadataFieldSpec{
				Name:       metadataFieldName,
				Kind:       metadataField.Kind,
				ItemKind:   metadataField.ItemKind,
				EnumValues: append([]any(nil), metadataField.EnumValues...),
				HasConst:   metadataField.HasConst,
				ConstValue: metadataField.ConstValue,
				Required:   metadataField.Required,
			}
		}

		for _, sectionName := range support.SortedMapKeys(entityType.ContentSections) {
			section := entityType.ContentSections[sectionName]
			entityTypeSpec.SectionFields[sectionName] = model.SectionFieldSpec{
				Name:     sectionName,
				Required: section.Required,
			}
		}

		index.EntityTypes[entityTypeName] = entityTypeSpec
	}

	if len(index.EntityTypes) == 0 {
		return model.QuerySchemaIndex{}, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			"schema.entity must be non-empty",
			nil,
		)
	}

	return index, nil
}
