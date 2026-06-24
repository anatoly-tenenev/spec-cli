package entitysort

import (
	"testing"

	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/model"
)

func TestSortEntities_MissingValueOrdering(t *testing.T) {
	entities := []model.EntityView{
		{Type: "feature", ID: "FEAT-1", View: map[string]any{"type": "feature", "id": "FEAT-1", "meta": map[string]any{"score": 7.0}}},
		{Type: "feature", ID: "FEAT-2", View: map[string]any{"type": "feature", "id": "FEAT-2", "meta": map[string]any{}}},
	}

	SortEntities(entities, []model.SortTerm{{Path: "meta.score", Direction: model.SortDirectionAsc}})
	if entities[0].ID != "FEAT-2" {
		t.Fatalf("missing value must come first for asc, got %s", entities[0].ID)
	}

	SortEntities(entities, []model.SortTerm{{Path: "meta.score", Direction: model.SortDirectionDesc}})
	if entities[0].ID != "FEAT-1" {
		t.Fatalf("missing value must come last for desc, got %s", entities[0].ID)
	}
}
