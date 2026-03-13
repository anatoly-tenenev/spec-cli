package engine

import "github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"

func newEngineTestIndex() model.QuerySchemaIndex {
	return model.QuerySchemaIndex{
		EntityTypes: map[string]model.EntityTypeSpec{
			"feature": {Name: "feature", RefFields: map[string]struct{}{"owner": {}}, SectionFields: map[string]struct{}{"summary": {}}},
			"service": {Name: "service", RefFields: map[string]struct{}{}, SectionFields: map[string]struct{}{"summary": {}}},
		},
		SelectorPaths: map[string]struct{}{
			"type":                     {},
			"id":                       {},
			"slug":                     {},
			"meta":                     {},
			"meta.status":              {},
			"meta.score":               {},
			"meta.tags":                {},
			"refs":                     {},
			"refs.owner":               {},
			"refs.owner.type":          {},
			"refs.owner.id":            {},
			"refs.owner.slug":          {},
			"content.raw":              {},
			"content.sections":         {},
			"content.sections.summary": {},
		},
		SortFields: map[string]model.SchemaFieldSpec{
			"type":                     {Path: "type", Kind: model.FieldKindString},
			"id":                       {Path: "id", Kind: model.FieldKindString},
			"updated_date":             {Path: "updated_date", Kind: model.FieldKindDate},
			"meta.score":               {Path: "meta.score", Kind: model.FieldKindNumber},
			"meta.status":              {Path: "meta.status", Kind: model.FieldKindString},
			"content.sections.summary": {Path: "content.sections.summary", Kind: model.FieldKindString},
		},
		FilterFields: map[string]model.SchemaFieldSpec{
			"type":                     {Path: "type", Kind: model.FieldKindString},
			"id":                       {Path: "id", Kind: model.FieldKindString},
			"updated_date":             {Path: "updated_date", Kind: model.FieldKindDate},
			"meta.status":              {Path: "meta.status", Kind: model.FieldKindString, EnumValues: []any{"draft", "active", "deprecated"}},
			"meta.score":               {Path: "meta.score", Kind: model.FieldKindNumber},
			"meta.tags":                {Path: "meta.tags", Kind: model.FieldKindArray},
			"content.sections.summary": {Path: "content.sections.summary", Kind: model.FieldKindString},
			"refs.owner.id":            {Path: "refs.owner.id", Kind: model.FieldKindString},
		},
	}
}
