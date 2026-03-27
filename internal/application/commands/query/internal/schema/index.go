package schema

import (
	"fmt"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

var builtinFieldKinds = map[string]model.SchemaFieldKind{
	"type":         model.FieldKindString,
	"id":           model.FieldKindString,
	"slug":         model.FieldKindString,
	"revision":     model.FieldKindString,
	"createdDate": model.FieldKindDate,
	"updatedDate": model.FieldKindDate,
}

func BuildIndex(loaded LoadedSchema) (model.QuerySchemaIndex, *domainerrors.AppError) {
	index := model.QuerySchemaIndex{
		EntityTypes:   make(map[string]model.EntityTypeSpec, len(loaded.EntityTypes)),
		SelectorPaths: map[string]struct{}{},
		SortFields:    map[string]model.SchemaFieldSpec{},
		FilterFields:  map[string]model.SchemaFieldSpec{},
	}

	for _, entityTypeName := range support.SortedMapKeys(loaded.EntityTypes) {
		entityType := loaded.EntityTypes[entityTypeName]

		entityTypeSpec := model.EntityTypeSpec{
			Name:          entityTypeName,
			RefFields:     map[string]struct{}{},
			RefTypeHints:  map[string]string{},
			SectionFields: map[string]struct{}{},
		}
		for refField, refFieldSpec := range entityType.EntityRefFields {
			entityTypeSpec.RefFields[refField] = struct{}{}
			if refFieldSpec.RefTypeHint != "" {
				entityTypeSpec.RefTypeHints[refField] = refFieldSpec.RefTypeHint
			}
		}
		for section := range entityType.ContentSections {
			entityTypeSpec.SectionFields[section] = struct{}{}
		}
		index.EntityTypes[entityTypeName] = entityTypeSpec

		addCommonSelectors(index.SelectorPaths)
		for builtinPath, builtinKind := range builtinFieldKinds {
			fieldSpec := model.SchemaFieldSpec{Path: builtinPath, Kind: builtinKind}
			if err := addFieldSpec(index.FilterFields, fieldSpec); err != nil {
				return model.QuerySchemaIndex{}, err
			}
			if isOrderableKind(builtinKind) {
				if err := addFieldSpec(index.SortFields, fieldSpec); err != nil {
					return model.QuerySchemaIndex{}, err
				}
			}
		}

		for metadataFieldName, metadataField := range entityType.MetadataFields {
			if metadataField.IsEntityRef {
				continue
			}
			path := "meta." + metadataFieldName
			index.SelectorPaths[path] = struct{}{}
			fieldSpec := model.SchemaFieldSpec{Path: path, Kind: metadataField.Kind, EnumValues: metadataField.EnumValues}
			if err := addFieldSpec(index.FilterFields, fieldSpec); err != nil {
				return model.QuerySchemaIndex{}, err
			}
			if isOrderableKind(metadataField.Kind) {
				if err := addFieldSpec(index.SortFields, fieldSpec); err != nil {
					return model.QuerySchemaIndex{}, err
				}
			}
		}

		for refFieldName := range entityType.EntityRefFields {
			index.SelectorPaths["refs."+refFieldName] = struct{}{}
			refFieldSpecs := []model.SchemaFieldSpec{
				{Path: fmt.Sprintf("refs.%s.id", refFieldName), Kind: model.FieldKindString},
				{Path: fmt.Sprintf("refs.%s.resolved", refFieldName), Kind: model.FieldKindBoolean},
				{Path: fmt.Sprintf("refs.%s.type", refFieldName), Kind: model.FieldKindString},
				{Path: fmt.Sprintf("refs.%s.slug", refFieldName), Kind: model.FieldKindString},
			}
			for _, fieldSpec := range refFieldSpecs {
				if err := addFieldSpec(index.FilterFields, fieldSpec); err != nil {
					return model.QuerySchemaIndex{}, err
				}
				if err := addFieldSpec(index.SortFields, fieldSpec); err != nil {
					return model.QuerySchemaIndex{}, err
				}
			}
		}

		if err := addFieldSpec(index.SortFields, model.SchemaFieldSpec{Path: "content.raw", Kind: model.FieldKindString}); err != nil {
			return model.QuerySchemaIndex{}, err
		}

		for sectionName := range entityType.ContentSections {
			path := "content.sections." + sectionName
			index.SelectorPaths[path] = struct{}{}
			fieldSpec := model.SchemaFieldSpec{Path: path, Kind: model.FieldKindString}
			if err := addFieldSpec(index.FilterFields, fieldSpec); err != nil {
				return model.QuerySchemaIndex{}, err
			}
			if err := addFieldSpec(index.SortFields, fieldSpec); err != nil {
				return model.QuerySchemaIndex{}, err
			}
		}
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

func addCommonSelectors(index map[string]struct{}) {
	for builtinPath := range builtinFieldKinds {
		index[builtinPath] = struct{}{}
	}
	index["meta"] = struct{}{}
	index["refs"] = struct{}{}
	index["content.raw"] = struct{}{}
	index["content.sections"] = struct{}{}
}

func addFieldSpec(index map[string]model.SchemaFieldSpec, candidate model.SchemaFieldSpec) *domainerrors.AppError {
	existing, exists := index[candidate.Path]
	if !exists {
		index[candidate.Path] = cloneFieldSpec(candidate)
		return nil
	}
	if existing.Kind != candidate.Kind {
		return newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("field '%s' has conflicting types across schema.entity", candidate.Path),
			nil,
		)
	}
	mergedEnum := mergeEnumValues(existing.EnumValues, candidate.EnumValues)
	existing.EnumValues = mergedEnum
	index[candidate.Path] = existing
	return nil
}

func mergeEnumValues(left []any, right []any) []any {
	if len(left) == 0 && len(right) == 0 {
		return nil
	}
	merged := make([]any, 0, len(left)+len(right))
	for _, item := range left {
		merged = append(merged, item)
	}
	for _, item := range right {
		alreadyAdded := false
		for _, candidate := range merged {
			if support.LiteralEqual(candidate, item) {
				alreadyAdded = true
				break
			}
		}
		if !alreadyAdded {
			merged = append(merged, item)
		}
	}
	return merged
}

func cloneFieldSpec(spec model.SchemaFieldSpec) model.SchemaFieldSpec {
	cloned := model.SchemaFieldSpec{Path: spec.Path, Kind: spec.Kind}
	if len(spec.EnumValues) > 0 {
		cloned.EnumValues = append([]any(nil), spec.EnumValues...)
	}
	return cloned
}

func isOrderableKind(kind model.SchemaFieldKind) bool {
	switch kind {
	case model.FieldKindString, model.FieldKindDate, model.FieldKindNumber, model.FieldKindBoolean:
		return true
	default:
		return false
	}
}
