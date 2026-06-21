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
			{Path: "id", Direction: model.SortDirectionAsc},
		},
		ActiveTypeSet: []string{"service", "feature"},
		Limit:         100,
		Offset:        0,
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

	if !reflect.DeepEqual(resultA.RootFields, resultB.RootFields) {
		t.Fatalf("root fields must be deterministic:\nA=%#v\nB=%#v", resultA.RootFields, resultB.RootFields)
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

	limitZero, limitZeroErr := Execute(model.QueryPlan{SelectTree: tree, EffectiveSort: []model.SortTerm{{Path: "id", Direction: model.SortDirectionAsc}}, ActiveTypeSet: []string{"feature"}, Limit: 0, Offset: 0}, entities)
	if limitZeroErr != nil {
		t.Fatalf("unexpected execute error: %v", limitZeroErr)
	}
	if len(limitZero.RootFields) != 1 {
		t.Fatalf("expected one root field, got %#v", limitZero.RootFields)
	}
	limitZeroRoot := limitZero.RootFields[0]
	if len(limitZeroRoot.Items) != 0 || limitZeroRoot.TotalCount != 2 || !limitZeroRoot.PageInfo.HasMore || limitZeroRoot.PageInfo.NextOffset != 0 {
		t.Fatalf("unexpected limit=0 response: %#v", limitZero)
	}

	offsetOutside, offsetOutsideErr := Execute(model.QueryPlan{SelectTree: tree, EffectiveSort: []model.SortTerm{{Path: "id", Direction: model.SortDirectionAsc}}, ActiveTypeSet: []string{"feature"}, Limit: 100, Offset: 10}, entities)
	if offsetOutsideErr != nil {
		t.Fatalf("unexpected execute error: %v", offsetOutsideErr)
	}
	offsetOutsideRoot := offsetOutside.RootFields[0]
	if len(offsetOutsideRoot.Items) != 0 || offsetOutsideRoot.PageInfo.Returned != 0 || offsetOutsideRoot.PageInfo.HasMore || offsetOutsideRoot.PageInfo.NextOffset != nil {
		t.Fatalf("unexpected offset>=matched response: %#v", offsetOutside)
	}
}

func TestExecute_IncludesNoMatchRootField(t *testing.T) {
	index := newEngineTestCapability()
	tree, err := buildSelectTree([]string{"type", "id"}, index, []string{"service", "feature"})
	if err != nil {
		t.Fatalf("unexpected select error: %v", err)
	}

	result, execErr := Execute(model.QueryPlan{
		SelectTree:    tree,
		EffectiveSort: []model.SortTerm{{Path: "id", Direction: model.SortDirectionAsc}},
		ActiveTypeSet: []string{"service", "feature"},
		Limit:         100,
		Offset:        0,
	}, []model.EntityView{
		{Type: "feature", ID: "FEAT-1", View: map[string]any{"type": "feature", "id": "FEAT-1"}},
	})
	if execErr != nil {
		t.Fatalf("unexpected execute error: %v", execErr)
	}
	if len(result.RootFields) != 2 {
		t.Fatalf("expected two root fields, got %#v", result.RootFields)
	}
	if result.RootFields[0].EntityType != "service" || len(result.RootFields[0].Items) != 0 || result.RootFields[0].TotalCount != 0 {
		t.Fatalf("unexpected empty service root: %#v", result.RootFields[0])
	}
	if result.RootFields[1].EntityType != "feature" || result.RootFields[1].TotalCount != 1 {
		t.Fatalf("unexpected feature root: %#v", result.RootFields[1])
	}
}
