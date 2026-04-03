package write

import (
	"reflect"
	"testing"

	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
)

func TestBuildWriteCapability(t *testing.T) {
	compiled := model.CompiledSchema{
		Entities: map[string]model.EntityType{
			"feature": {
				MetaFields: map[string]model.MetaField{
					"status": {
						Value: model.ValueSpec{Kind: model.ValueKindString},
					},
					"owner": {
						Value: model.ValueSpec{
							Kind: model.ValueKindEntityRef,
							Ref:  &model.RefSpec{Cardinality: model.RefCardinalityScalar},
						},
					},
				},
				Sections: map[string]model.Section{
					"summary": {},
				},
			},
		},
	}

	capability := Build(compiled)
	entity := capability.EntityTypes["feature"]

	expectedSetPaths := []string{"content.sections.summary", "meta.status", "refs.owner"}
	if !reflect.DeepEqual(entity.SetPaths, expectedSetPaths) {
		t.Fatalf("unexpected set paths: %#v", entity.SetPaths)
	}
	if !reflect.DeepEqual(entity.UnsetPaths, expectedSetPaths) {
		t.Fatalf("unexpected unset paths: %#v", entity.UnsetPaths)
	}
	if !reflect.DeepEqual(entity.SetFilePaths, []string{"content.sections.summary"}) {
		t.Fatalf("unexpected set-file paths: %#v", entity.SetFilePaths)
	}
}
