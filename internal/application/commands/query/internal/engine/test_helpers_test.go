package engine

import schemacapread "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/read"

func newEngineTestCapability() schemacapread.Capability {
	return schemacapread.Capability{
		EntityTypes: map[string]schemacapread.EntityReadModel{
			"feature": {
				MetaFields: map[string]schemacapread.MetaField{
					"status": {Kind: schemacapread.FieldKindString, EnumValues: []any{"draft", "active", "deprecated"}, Required: true},
					"score":  {Kind: schemacapread.FieldKindNumber, Required: true},
					"tags":   {Kind: schemacapread.FieldKindArray, ItemKind: schemacapread.FieldKindString, Required: true},
				},
				RefFields: map[string]schemacapread.RefField{
					"owner": {Cardinality: schemacapread.RefCardinalityScalar, AllowedTypes: []string{"service"}},
				},
				Sections: map[string]schemacapread.Section{
					"summary": {Required: true},
				},
			},
			"service": {
				MetaFields: map[string]schemacapread.MetaField{
					"status": {Kind: schemacapread.FieldKindString, Required: true},
					"score":  {Kind: schemacapread.FieldKindNumber, Required: true},
				},
				RefFields: map[string]schemacapread.RefField{},
				Sections: map[string]schemacapread.Section{
					"summary": {Required: true},
				},
			},
		},
	}
}
