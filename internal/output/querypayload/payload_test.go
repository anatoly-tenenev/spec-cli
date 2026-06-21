package querypayload

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildSuccess_SerializesGraphQLStyleRootsInOrder(t *testing.T) {
	payload := BuildSuccess("valid", map[string]any{"valid": true}, []RootField{
		{
			EntityType: "service",
			Items:      []map[string]any{},
			TotalCount: 0,
			PageInfo: PageInfo{
				Mode:          "offset",
				Limit:         100,
				Offset:        0,
				Returned:      0,
				HasMore:       false,
				NextOffset:    nil,
				EffectiveSort: []string{"id:asc"},
			},
		},
		{
			EntityType: "feature",
			Items:      []map[string]any{{"type": "feature", "id": "FEAT-1"}},
			TotalCount: 1,
			PageInfo:   PageInfo{Mode: "offset", Limit: 100, Offset: 0, Returned: 1, HasMore: false, NextOffset: nil, EffectiveSort: []string{"updatedDate:desc", "id:asc"}},
		},
	})

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	serialized := string(raw)

	serviceIdx := strings.Index(serialized, `"service"`)
	featureIdx := strings.Index(serialized, `"feature"`)
	if serviceIdx < 0 || featureIdx < 0 || serviceIdx > featureIdx {
		t.Fatalf("data roots must preserve input order, got %s", serialized)
	}
	if strings.Contains(serialized, `"matched"`) || strings.Contains(serialized, `"page"`) {
		t.Fatalf("payload must not contain old top-level list fields: %s", serialized)
	}
	if !strings.Contains(serialized, `"totalCount"`) || !strings.Contains(serialized, `"pageInfo"`) {
		t.Fatalf("payload must contain GraphQL-style list fields: %s", serialized)
	}
	if !strings.Contains(serialized, `"hasMore"`) || !strings.Contains(serialized, `"nextOffset"`) || !strings.Contains(serialized, `"effectiveSort"`) {
		t.Fatalf("pageInfo must use camelCase fields: %s", serialized)
	}
}
