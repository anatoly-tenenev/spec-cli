package validate

import (
	"reflect"
	"testing"

	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
)

func TestBuildValidationCapabilityDeterministicProjection(t *testing.T) {
	minItems := 1
	maxItems := 2

	compiled := model.CompiledSchema{
		Entities: map[string]model.EntityType{
			"service": {
				Name:     "service",
				IDPrefix: "SVC",
				MetaFields: map[string]model.MetaField{
					"owner": {
						Name: "owner",
						Value: model.ValueSpec{
							Kind: model.ValueKindEntityRef,
							Ref: &model.RefSpec{
								Cardinality:  model.RefCardinalityScalar,
								AllowedTypes: []string{"person"},
							},
						},
						Required: model.Requirement{Always: false},
					},
					"tags": {
						Name: "tags",
						Value: model.ValueSpec{
							Kind: model.ValueKindArray,
							Items: &model.ValueSpec{
								Kind: model.ValueKindString,
							},
							UniqueItems: true,
							MinItems:    &minItems,
							MaxItems:    &maxItems,
						},
						Required: model.Requirement{Always: true},
					},
				},
				Sections: map[string]model.Section{
					"summary": {
						Name:     "summary",
						Title:    "Summary",
						Required: model.Requirement{Always: true},
					},
				},
				PathTemplate: model.PathTemplate{
					Cases: []model.PathTemplateCase{
						{Use: "services/${slug}.md", When: model.Requirement{Always: true}},
					},
				},
			},
			"person": {
				Name:     "person",
				IDPrefix: "PER",
				MetaFields: map[string]model.MetaField{
					"name": {
						Name: "name",
						Value: model.ValueSpec{
							Kind: model.ValueKindString,
						},
						Required: model.Requirement{Always: true},
					},
				},
				PathTemplate: model.PathTemplate{
					Cases: []model.PathTemplateCase{
						{Use: "people/${slug}.md", When: model.Requirement{Always: true}},
					},
				},
			},
		},
	}

	capability := Build(compiled)

	if !reflect.DeepEqual(capability.EntityOrder, []string{"person", "service"}) {
		t.Fatalf("unexpected entity order: %#v", capability.EntityOrder)
	}

	service := capability.EntityTypes["service"]
	if !reflect.DeepEqual(
		service.AllowedFrontmatterFields,
		[]string{"type", "id", "slug", "createdDate", "updatedDate", "owner", "tags"},
	) {
		t.Fatalf("unexpected allowed frontmatter fields: %#v", service.AllowedFrontmatterFields)
	}
	if len(service.RequiredFields) != 2 {
		t.Fatalf("unexpected required fields count: %d", len(service.RequiredFields))
	}
	if service.RequiredFields[0].Name != "owner" || service.RequiredFields[0].Type != "entityRef" {
		t.Fatalf("unexpected first field rule: %#v", service.RequiredFields[0])
	}
	if !reflect.DeepEqual(service.RequiredFields[0].RefTypes, []string{"person"}) {
		t.Fatalf("unexpected owner ref types: %#v", service.RequiredFields[0].RefTypes)
	}
	if service.RequiredFields[1].Name != "tags" || !service.RequiredFields[1].HasItemType {
		t.Fatalf("unexpected second field rule: %#v", service.RequiredFields[1])
	}
	if !service.RequiredFields[1].UniqueItems || !service.RequiredFields[1].HasMinItems || !service.RequiredFields[1].HasMaxItems {
		t.Fatalf("unexpected array constraints: %#v", service.RequiredFields[1])
	}
	if service.RequiredFields[1].MinItems != 1 || service.RequiredFields[1].MaxItems != 2 {
		t.Fatalf("unexpected min/max items: %#v", service.RequiredFields[1])
	}
	if len(service.RequiredSections) != 1 || service.RequiredSections[0].Name != "summary" {
		t.Fatalf("unexpected section rules: %#v", service.RequiredSections)
	}
	if service.RequiredSections[0].Title != "Summary" {
		t.Fatalf("unexpected summary title rule: %#v", service.RequiredSections[0])
	}
	if len(service.PathPattern.Cases) != 1 || service.PathPattern.Cases[0].Use != "services/${slug}.md" {
		t.Fatalf("unexpected path pattern rules: %#v", service.PathPattern.Cases)
	}
}
