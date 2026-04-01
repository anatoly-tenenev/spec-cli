package engine

import "github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"

func newEngineTestIndex() model.QuerySchemaIndex {
	return model.QuerySchemaIndex{
		EntityTypes: map[string]model.EntityTypeSpec{
			"feature": {
				Name: "feature",
				MetaFields: map[string]model.MetadataFieldSpec{
					"status": {Name: "status", Kind: model.FieldKindString, EnumValues: []any{"draft", "active", "deprecated"}, Required: true},
					"score":  {Name: "score", Kind: model.FieldKindNumber, Required: true},
					"tags":   {Name: "tags", Kind: model.FieldKindArray, ItemKind: model.FieldKindString, Required: true},
				},
				RefFields: map[string]model.RefFieldSpec{
					"owner": {Name: "owner", Cardinality: model.RefCardinalityScalar, RefTypes: []string{"service"}},
				},
				SectionFields: map[string]model.SectionFieldSpec{
					"summary": {Name: "summary", Required: true},
				},
			},
			"service": {
				Name: "service",
				MetaFields: map[string]model.MetadataFieldSpec{
					"status": {Name: "status", Kind: model.FieldKindString, Required: true},
					"score":  {Name: "score", Kind: model.FieldKindNumber, Required: true},
				},
				RefFields: map[string]model.RefFieldSpec{},
				SectionFields: map[string]model.SectionFieldSpec{
					"summary": {Name: "summary", Required: true},
				},
			},
		},
	}
}
