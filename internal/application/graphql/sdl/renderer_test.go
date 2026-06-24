package sdl

import (
	"strings"
	"testing"

	gqlmodel "github.com/anatoly-tenenev/spec-cli/internal/application/graphql/model"
)

func TestRender_SelectedEntitySDLIncludesDirectivesAndSkipsUnselectedTypes(t *testing.T) {
	rendered := Render(testProjection(), []string{"service"})

	requiredFragments := []string{
		"directive @requiredWhen(expr: String!) on FIELD_DEFINITION",
		"directive @arrayConstraints(minItems: Int, maxItems: Int, uniqueItems: Boolean) on FIELD_DEFINITION",
		"enum EntityType {\n  service\n  team\n}",
		"  service(where: ServiceWhere, sort: [ServiceSort!], limit: Int = 100, offset: Int = 0): ServiceList!",
		"enum ServiceStatus {\n  active\n  deprecated\n}",
		`  status: ServiceStatus! @requiredWhen(expr: "meta.status == \"active\"\nmeta.kind == \"critical\"")`,
		"  tags: [String!] @arrayConstraints(minItems: 1, maxItems: 3, uniqueItems: true)",
		"type ServiceOwnerRef {",
		"  summary: String!",
		"enum ServiceSortField {\n  id\n  meta_status\n  content_sections_summary\n}",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(rendered, fragment) {
			t.Fatalf("rendered SDL is missing fragment %q:\n%s", fragment, rendered)
		}
	}

	forbiddenFragments := []string{
		"  team(where: TeamWhere",
		"type Team {",
	}
	for _, fragment := range forbiddenFragments {
		if strings.Contains(rendered, fragment) {
			t.Fatalf("rendered SDL contains unselected fragment %q:\n%s", fragment, rendered)
		}
	}
}

func TestRender_NilProjectionReturnsEmptyString(t *testing.T) {
	if rendered := Render(nil, nil); rendered != "" {
		t.Fatalf("expected empty SDL for nil projection, got %q", rendered)
	}
}

func testProjection() *gqlmodel.Projection {
	minItems := 1
	maxItems := 3
	return &gqlmodel.Projection{
		EntityOrder: []string{"service", "team"},
		Entities: map[string]gqlmodel.Entity{
			"service": {
				Name:          "service",
				TypeName:      "Service",
				ListName:      "ServiceList",
				MetaName:      "ServiceMeta",
				RefsName:      "ServiceRefs",
				ContentName:   "ServiceContent",
				SectionsName:  "ServiceSections",
				WhereName:     "ServiceWhere",
				SortName:      "ServiceSort",
				SortFieldName: "ServiceSortField",
				Description:   `Service """docs"""`,
				MetaFields: []gqlmodel.MetaField{
					{
						Name:         "status",
						TypeName:     "ServiceStatus",
						Required:     true,
						RequiredWhen: "meta.status == \"active\"\nmeta.kind == \"critical\"",
						Description:  "Lifecycle status",
						EnumName:     "ServiceStatus",
						EnumValues:   []string{"active", "deprecated"},
					},
					{
						Name:        "tags",
						TypeName:    "String",
						IsArray:     true,
						MinItems:    &minItems,
						MaxItems:    &maxItems,
						UniqueItems: true,
					},
				},
				RefFields: []gqlmodel.RefField{
					{
						Name:         "owner",
						TypeName:     "ServiceOwnerRef",
						EnumName:     "ServiceOwnerRefType",
						Required:     true,
						Description:  "Owning team",
						Cardinality:  "scalar",
						AllowedTypes: []string{"team"},
					},
				},
				Sections: []gqlmodel.Section{
					{Name: "summary", Required: true},
				},
				SortFields: []gqlmodel.SortField{
					{Name: "id", Path: "id"},
					{Name: "meta_status", Path: "meta.status"},
					{Name: "content_sections_summary", Path: "content.sections.summary"},
				},
			},
			"team": {
				Name:          "team",
				TypeName:      "Team",
				ListName:      "TeamList",
				MetaName:      "TeamMeta",
				RefsName:      "TeamRefs",
				ContentName:   "TeamContent",
				SectionsName:  "TeamSections",
				WhereName:     "TeamWhere",
				SortName:      "TeamSort",
				SortFieldName: "TeamSortField",
				SortFields:    []gqlmodel.SortField{{Name: "id", Path: "id"}},
			},
		},
	}
}
