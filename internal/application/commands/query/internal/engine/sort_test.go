package engine

import (
	"testing"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func TestBuildEffectiveSort_DefaultAndTail(t *testing.T) {
	index := newEngineTestIndex()

	terms, err := buildEffectiveSort(nil, index)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(terms) != 2 || terms[0].Path != "type" || terms[1].Path != "id" {
		t.Fatalf("unexpected default sort: %#v", terms)
	}

	custom := []model.SortTerm{{Path: "updatedDate", Direction: model.SortDirectionDesc}}
	effective, err := buildEffectiveSort(custom, index)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(effective) != 3 || effective[1].Path != "type" || effective[2].Path != "id" {
		t.Fatalf("tail not appended: %#v", effective)
	}
}

func TestBuildEffectiveSort_InvalidField(t *testing.T) {
	index := newEngineTestIndex()
	_, err := buildEffectiveSort([]model.SortTerm{{Path: "meta", Direction: model.SortDirectionAsc}}, index)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != domainerrors.CodeInvalidArgs {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

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
