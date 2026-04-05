package engine

import (
	"reflect"
	"testing"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
)

func TestExecute_DeterministicAcrossInputOrder(t *testing.T) {
	index := newEngineTestCapability()
	tree, err := buildSelectTree([]string{"type", "id"}, index, []string{"feature", "service"})
	if err != nil {
		t.Fatalf("unexpected select error: %v", err)
	}

	plan := model.QueryPlan{
		SelectTree: tree,
		EffectiveSort: []model.SortTerm{
			{Path: "type", Direction: model.SortDirectionAsc},
			{Path: "id", Direction: model.SortDirectionAsc},
		},
		Limit:  100,
		Offset: 0,
	}

	setA := []model.EntityView{
		{Type: "service", ID: "SVC-1", View: map[string]any{"type": "service", "id": "SVC-1"}},
		{Type: "feature", ID: "FEAT-2", View: map[string]any{"type": "feature", "id": "FEAT-2"}},
		{Type: "feature", ID: "FEAT-1", View: map[string]any{"type": "feature", "id": "FEAT-1"}},
	}
	setB := []model.EntityView{setA[2], setA[0], setA[1]}

	resultA, resultAErr := Execute(plan, setA)
	if resultAErr != nil {
		t.Fatalf("unexpected execute error: %v", resultAErr)
	}
	resultB, resultBErr := Execute(plan, setB)
	if resultBErr != nil {
		t.Fatalf("unexpected execute error: %v", resultBErr)
	}

	if !reflect.DeepEqual(resultA.Items, resultB.Items) {
		t.Fatalf("items must be deterministic:\nA=%#v\nB=%#v", resultA.Items, resultB.Items)
	}
}

func TestExecute_PaginationBoundaries(t *testing.T) {
	index := newEngineTestCapability()
	tree, err := buildSelectTree([]string{"type", "id"}, index, []string{"feature", "service"})
	if err != nil {
		t.Fatalf("unexpected select error: %v", err)
	}

	entities := []model.EntityView{
		{Type: "feature", ID: "FEAT-1", View: map[string]any{"type": "feature", "id": "FEAT-1"}},
		{Type: "feature", ID: "FEAT-2", View: map[string]any{"type": "feature", "id": "FEAT-2"}},
	}

	limitZero, limitZeroErr := Execute(model.QueryPlan{SelectTree: tree, EffectiveSort: []model.SortTerm{{Path: "type", Direction: model.SortDirectionAsc}, {Path: "id", Direction: model.SortDirectionAsc}}, Limit: 0, Offset: 0}, entities)
	if limitZeroErr != nil {
		t.Fatalf("unexpected execute error: %v", limitZeroErr)
	}
	if len(limitZero.Items) != 0 || limitZero.Matched != 2 || !limitZero.Page.HasMore || limitZero.Page.NextOffset != 0 {
		t.Fatalf("unexpected limit=0 response: %#v", limitZero)
	}

	offsetOutside, offsetOutsideErr := Execute(model.QueryPlan{SelectTree: tree, EffectiveSort: []model.SortTerm{{Path: "type", Direction: model.SortDirectionAsc}, {Path: "id", Direction: model.SortDirectionAsc}}, Limit: 100, Offset: 10}, entities)
	if offsetOutsideErr != nil {
		t.Fatalf("unexpected execute error: %v", offsetOutsideErr)
	}
	if len(offsetOutside.Items) != 0 || offsetOutside.Page.Returned != 0 || offsetOutside.Page.HasMore || offsetOutside.Page.NextOffset != nil {
		t.Fatalf("unexpected offset>=matched response: %#v", offsetOutside)
	}
}
