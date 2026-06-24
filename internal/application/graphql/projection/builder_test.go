package projection

import (
	"reflect"
	"testing"

	gqlmodel "github.com/anatoly-tenenev/spec-cli/internal/application/graphql/model"
	readcap "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/read"
	schemaexpressions "github.com/anatoly-tenenev/spec-cli/internal/application/schema/expressions"
	schemamodel "github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func TestBuild_ProjectsGraphQLEntityMetadataAndSortFields(t *testing.T) {
	proj, appErr := Build(testCompiledSchema(), testReadCapability())
	if appErr != nil {
		t.Fatalf("unexpected projection error: %v", appErr)
	}
	if !reflect.DeepEqual(proj.EntityOrder, []string{"service", "team"}) {
		t.Fatalf("unexpected entity order: %#v", proj.EntityOrder)
	}

	service := proj.Entities["service"]
	if service.TypeName != "Service" || service.ListName != "ServiceList" || service.WhereName != "ServiceWhere" {
		t.Fatalf("unexpected generated names: %#v", service)
	}
	if service.Description != "Service documentation" {
		t.Fatalf("unexpected description: %q", service.Description)
	}

	status := service.MetaFields[0]
	if status.Name != "status" || status.TypeName != "ServiceStatus" || status.EnumName != "ServiceStatus" {
		t.Fatalf("unexpected status field: %#v", status)
	}
	if !status.Required || !reflect.DeepEqual(status.EnumValues, []string{"active", "deprecated"}) {
		t.Fatalf("unexpected status field constraints: %#v", status)
	}

	releaseDate := service.MetaFields[1]
	if releaseDate.Name != "releaseDate" || releaseDate.TypeName != "Date" || releaseDate.RequiredWhen != "meta.status == 'active'" {
		t.Fatalf("unexpected releaseDate field: %#v", releaseDate)
	}

	tags := service.MetaFields[2]
	if tags.Name != "tags" || tags.TypeName != "String" || !tags.IsArray || tags.MinItems == nil || *tags.MinItems != 1 || !tags.UniqueItems {
		t.Fatalf("unexpected tags field: %#v", tags)
	}

	owner := service.RefFields[0]
	if owner.Name != "owner" || owner.TypeName != "ServiceOwnerRef" || owner.EnumName != "ServiceOwnerRefType" || !owner.Required {
		t.Fatalf("unexpected owner ref field: %#v", owner)
	}
	if !reflect.DeepEqual(owner.AllowedTypes, []string{"team"}) {
		t.Fatalf("unexpected owner allowed types: %#v", owner.AllowedTypes)
	}

	expectedSorts := []gqlSortField{
		{Name: "id", Path: "id"},
		{Name: "slug", Path: "slug"},
		{Name: "revision", Path: "revision"},
		{Name: "createdDate", Path: "createdDate"},
		{Name: "updatedDate", Path: "updatedDate"},
		{Name: "meta_status", Path: "meta.status"},
		{Name: "meta_releaseDate", Path: "meta.releaseDate"},
		{Name: "refs_owner_id", Path: "refs.owner.id"},
		{Name: "refs_owner_type", Path: "refs.owner.type"},
		{Name: "refs_owner_slug", Path: "refs.owner.slug"},
		{Name: "refs_owner_resolved", Path: "refs.owner.resolved"},
		{Name: "content_sections_summary", Path: "content.sections.summary"},
	}
	if !reflect.DeepEqual(sortFields(service.SortFields), expectedSorts) {
		t.Fatalf("unexpected sort fields:\nwant=%#v\ngot=%#v", expectedSorts, sortFields(service.SortFields))
	}
}

func TestBuild_RejectsGeneratedTypeNameCollision(t *testing.T) {
	compiled := schemamodel.CompiledSchema{
		Entities: map[string]schemamodel.EntityType{
			"query": {
				Name:       "query",
				MetaFields: map[string]schemamodel.MetaField{},
				Sections:   map[string]schemamodel.Section{},
			},
		},
		EntityOrder: []string{"query"},
	}
	capability := readcap.Capability{
		EntityOrder: []string{"query"},
		EntityTypes: map[string]readcap.EntityReadModel{
			"query": {
				MetaFields: map[string]readcap.MetaField{},
				RefFields:  map[string]readcap.RefField{},
				Sections:   map[string]readcap.Section{},
			},
		},
	}

	_, appErr := Build(compiled, capability)
	if appErr == nil {
		t.Fatal("expected projection error")
	}
	if appErr.Code != domainerrors.CodeGraphQLProjectionError {
		t.Fatalf("unexpected error code: %s", appErr.Code)
	}
}

type gqlSortField struct {
	Name string
	Path string
}

func sortFields(fields []gqlmodel.SortField) []gqlSortField {
	out := make([]gqlSortField, 0, len(fields))
	for _, field := range fields {
		out = append(out, gqlSortField{Name: field.Name, Path: field.Path})
	}
	return out
}

func testCompiledSchema() schemamodel.CompiledSchema {
	minItems := 1
	return schemamodel.CompiledSchema{
		Entities: map[string]schemamodel.EntityType{
			"service": {
				Name:           "service",
				Description:    "Service documentation",
				MetaFieldOrder: []string{"status", "releaseDate", "tags", "owner"},
				MetaFields: map[string]schemamodel.MetaField{
					"status": {
						Name:        "status",
						Description: "Lifecycle status",
						Required:    schemamodel.Requirement{Always: true},
						Value: schemamodel.ValueSpec{
							Kind: schemamodel.ValueKindString,
						},
					},
					"releaseDate": {
						Name:     "releaseDate",
						Required: schemamodel.Requirement{Expr: &schemaexpressions.CompiledExpression{Source: "meta.status == 'active'"}},
						Value: schemamodel.ValueSpec{
							Kind:   schemamodel.ValueKindString,
							Format: "date",
						},
					},
					"tags": {
						Name: "tags",
						Value: schemamodel.ValueSpec{
							Kind:        schemamodel.ValueKindArray,
							Items:       &schemamodel.ValueSpec{Kind: schemamodel.ValueKindString},
							MinItems:    &minItems,
							UniqueItems: true,
						},
					},
					"owner": {
						Name:        "owner",
						Description: "Owning team",
						Required:    schemamodel.Requirement{Always: true},
						Value: schemamodel.ValueSpec{
							Kind: schemamodel.ValueKindEntityRef,
							Ref:  &schemamodel.RefSpec{AllowedTypes: []string{"team"}},
						},
					},
				},
				SectionOrder: []string{"summary"},
				Sections: map[string]schemamodel.Section{
					"summary": {
						Name:     "summary",
						Required: schemamodel.Requirement{Always: true},
					},
				},
			},
			"team": {
				Name:       "team",
				MetaFields: map[string]schemamodel.MetaField{},
				Sections:   map[string]schemamodel.Section{},
			},
		},
		EntityOrder: []string{"service", "team"},
	}
}

func testReadCapability() readcap.Capability {
	return readcap.Capability{
		EntityOrder: []string{"service", "team"},
		EntityTypes: map[string]readcap.EntityReadModel{
			"service": {
				MetaFields: map[string]readcap.MetaField{
					"status": {
						Kind:       readcap.FieldKindString,
						EnumValues: []any{"active", "deprecated", "active"},
						Required:   true,
					},
					"releaseDate": {
						Kind: readcap.FieldKindDate,
					},
					"tags": {
						Kind:     readcap.FieldKindArray,
						ItemKind: readcap.FieldKindString,
					},
				},
				RefFields: map[string]readcap.RefField{
					"owner": {
						Cardinality:  readcap.RefCardinalityScalar,
						AllowedTypes: []string{"team"},
					},
				},
				Sections: map[string]readcap.Section{"summary": {Required: true}},
			},
			"team": {
				MetaFields: map[string]readcap.MetaField{},
				RefFields:  map[string]readcap.RefField{},
				Sections:   map[string]readcap.Section{},
			},
		},
	}
}
