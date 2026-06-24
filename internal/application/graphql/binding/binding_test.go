package binding

import (
	"reflect"
	"testing"

	gqlmodel "github.com/anatoly-tenenev/spec-cli/internal/application/graphql/model"
	readmodel "github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func TestBuildAndExecute_MergesSelectionsAndAppliesGraphQLArguments(t *testing.T) {
	roots, buildErr := Build(testProjection(), testQuery(), map[string]any{
		"status":    "active",
		"withOwner": true,
		"withPage":  true,
	}, "Fetch")
	if buildErr != nil {
		t.Fatalf("unexpected build error: %v", buildErr)
	}
	if len(roots) != 1 {
		t.Fatalf("expected one merged root plan, got %#v", roots)
	}
	root := roots[0]
	if root.ResponseKey != "active" || root.EntityType != "service" || root.Limit != 1 || root.Offset != 0 {
		t.Fatalf("unexpected root plan: %#v", root)
	}
	expectedSort := []readmodel.SortTerm{
		{Path: "meta.priority", Direction: readmodel.SortDirectionDesc},
		{Path: "id", Direction: readmodel.SortDirectionAsc},
	}
	if !reflect.DeepEqual(root.Sort, expectedSort) {
		t.Fatalf("unexpected sort:\nwant=%#v\ngot=%#v", expectedSort, root.Sort)
	}
	for _, path := range []string{"id", "slug", "meta.state", "refs.owner.id", "content.sections.summary"} {
		if !containsString(root.NonNullPaths, path) {
			t.Fatalf("expected non-null path %q in %#v", path, root.NonNullPaths)
		}
	}

	data, execErr := Execute(roots, []readmodel.EntityView{
		serviceEntity("SVC-1", "billing", "active", 20, "billing retry window", map[string]any{
			"id": "TEAM-1", "type": "team", "slug": "platform", "resolved": true,
		}),
		serviceEntity("SVC-2", "ledger", "active", 10, "billing ledger bridge", map[string]any{
			"id": "TEAM-2", "type": "team", "slug": "ledger", "resolved": true,
		}),
		serviceEntity("SVC-3", "archive", "deprecated", 30, "billing archive", map[string]any{
			"id": "TEAM-3", "type": "team", "slug": "archive", "resolved": true,
		}),
	})
	if execErr != nil {
		t.Fatalf("unexpected execute error: %v", execErr)
	}

	expected := map[string]any{
		"active": map[string]any{
			"items": []any{
				map[string]any{
					"content": map[string]any{
						"sections": map[string]any{"summary": "billing retry window"},
					},
					"id": "SVC-1",
					"meta": map[string]any{
						"priority": float64(20),
						"state":    "active",
					},
					"refs": map[string]any{
						"owner": map[string]any{
							"id": "TEAM-1", "type": "team", "slug": "platform", "resolved": true,
						},
					},
					"slug": "billing",
				},
			},
			"pageInfo":   map[string]any{"hasMore": true, "nextOffset": 1, "returned": 1},
			"totalCount": 2,
		},
	}
	if !reflect.DeepEqual(data, expected) {
		t.Fatalf("unexpected data:\nwant=%#v\ngot=%#v", expected, data)
	}
}

func TestExecute_ReturnsInvalidResultForSelectedRequiredFieldMissing(t *testing.T) {
	roots, buildErr := Build(testProjection(), `query { service { items { id meta { status } } } }`, nil, "")
	if buildErr != nil {
		t.Fatalf("unexpected build error: %v", buildErr)
	}

	_, execErr := Execute(roots, []readmodel.EntityView{
		{
			Type: "service",
			ID:   "SVC-1",
			View: map[string]any{
				"id":   "SVC-1",
				"meta": map[string]any{},
			},
			WhereContext: map[string]any{
				"id":   "SVC-1",
				"meta": map[string]any{},
			},
		},
	})
	if execErr == nil {
		t.Fatal("expected invalid result error")
	}
	if execErr.Code != domainerrors.CodeInvalidQueryResult {
		t.Fatalf("unexpected error code: %s", execErr.Code)
	}
}

func testQuery() string {
	return `query Other {
  service {
    totalCount
  }
}

query Fetch($status: ServiceStatus!, $withOwner: Boolean!, $withPage: Boolean!) {
  active: service(
    where: { and: [
      { meta: { status: { eq: $status } } },
      { content: { sections: { summary: { contains: "billing" } } } }
    ] }
    sort: [{ field: meta_priority, direction: desc }]
    limit: 1
  ) {
    items {
      id
      meta {
        state: status
      }
      ...ServiceContent
    }
  }
  active: service(
    where: { and: [
      { meta: { status: { eq: $status } } },
      { content: { sections: { summary: { contains: "billing" } } } }
    ] }
    sort: [{ field: meta_priority, direction: desc }]
    limit: 1
  ) {
    items {
      slug
      meta {
        priority
      }
      refs {
        owner @include(if: $withOwner) {
          id
          type
          slug
          resolved
        }
      }
    }
    totalCount
    pageInfo @include(if: $withPage) {
      returned
      hasMore
      nextOffset
    }
  }
}

fragment ServiceContent on Service {
  content {
    sections {
      summary
    }
  }
}`
}

func testProjection() *gqlmodel.Projection {
	return &gqlmodel.Projection{
		EntityOrder: []string{"service"},
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
				MetaFields: []gqlmodel.MetaField{
					{
						Name:       "status",
						TypeName:   "ServiceStatus",
						Required:   true,
						EnumName:   "ServiceStatus",
						EnumValues: []string{"active", "deprecated"},
					},
					{
						Name:     "priority",
						TypeName: "Float",
					},
				},
				RefFields: []gqlmodel.RefField{
					{
						Name:         "owner",
						TypeName:     "ServiceOwnerRef",
						EnumName:     "ServiceOwnerRefType",
						Required:     true,
						Cardinality:  "scalar",
						AllowedTypes: []string{"team"},
					},
				},
				Sections: []gqlmodel.Section{
					{Name: "summary", Required: true},
				},
				SortFields: []gqlmodel.SortField{
					{Name: "id", Path: "id"},
					{Name: "meta_priority", Path: "meta.priority"},
				},
			},
		},
	}
}

func serviceEntity(id string, slug string, status string, priority float64, summary string, owner map[string]any) readmodel.EntityView {
	view := map[string]any{
		"type":        "service",
		"id":          id,
		"slug":        slug,
		"revision":    "rev-" + id,
		"createdDate": "2026-01-01",
		"updatedDate": "2026-01-02",
		"meta": map[string]any{
			"status":   status,
			"priority": priority,
		},
		"refs": map[string]any{
			"owner": owner,
		},
		"content": map[string]any{
			"sections": map[string]any{"summary": summary},
		},
	}
	return readmodel.EntityView{
		Type:         "service",
		ID:           id,
		View:         view,
		WhereContext: view,
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
