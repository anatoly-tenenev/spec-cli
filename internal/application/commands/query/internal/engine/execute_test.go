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
		RootPlans: []model.RootPlan{
			newTestRootPlan("service", 100, 0, []model.SortTerm{{Path: "id", Direction: model.SortDirectionAsc}}),
			newTestRootPlan("feature", 100, 0, []model.SortTerm{{Path: "id", Direction: model.SortDirectionAsc}}),
		},
		ActiveTypeSet: []string{"service", "feature"},
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

	limitZero, limitZeroErr := Execute(model.QueryPlan{SelectTree: tree, RootPlans: []model.RootPlan{
		newTestRootPlan("feature", 0, 0, []model.SortTerm{{Path: "id", Direction: model.SortDirectionAsc}}),
	}, ActiveTypeSet: []string{"feature"}}, entities)
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

	offsetOutside, offsetOutsideErr := Execute(model.QueryPlan{SelectTree: tree, RootPlans: []model.RootPlan{
		newTestRootPlan("feature", 100, 10, []model.SortTerm{{Path: "id", Direction: model.SortDirectionAsc}}),
	}, ActiveTypeSet: []string{"feature"}}, entities)
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
		SelectTree: tree,
		RootPlans: []model.RootPlan{
			newTestRootPlan("service", 100, 0, []model.SortTerm{{Path: "id", Direction: model.SortDirectionAsc}}),
			newTestRootPlan("feature", 100, 0, []model.SortTerm{{Path: "id", Direction: model.SortDirectionAsc}}),
		},
		ActiveTypeSet: []string{"service", "feature"},
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

func TestExecute_UsesIndependentRootPlans(t *testing.T) {
	index := newEngineTestCapability()
	tree, err := buildSelectTree([]string{"id"}, index, []string{"feature", "service"})
	if err != nil {
		t.Fatalf("unexpected select error: %v", err)
	}

	result, execErr := Execute(model.QueryPlan{
		SelectTree: tree,
		RootPlans: []model.RootPlan{
			newTestRootPlan("feature", 1, 1, []model.SortTerm{{Path: "id", Direction: model.SortDirectionDesc}}),
			newTestRootPlan("service", 1, 0, []model.SortTerm{{Path: "id", Direction: model.SortDirectionAsc}}),
		},
		ActiveTypeSet: []string{"feature", "service"},
	}, []model.EntityView{
		{Type: "feature", ID: "FEAT-1", View: map[string]any{"id": "FEAT-1"}},
		{Type: "feature", ID: "FEAT-2", View: map[string]any{"id": "FEAT-2"}},
		{Type: "feature", ID: "FEAT-3", View: map[string]any{"id": "FEAT-3"}},
		{Type: "service", ID: "SVC-2", View: map[string]any{"id": "SVC-2"}},
		{Type: "service", ID: "SVC-1", View: map[string]any{"id": "SVC-1"}},
	})
	if execErr != nil {
		t.Fatalf("unexpected execute error: %v", execErr)
	}
	if len(result.RootFields) != 2 {
		t.Fatalf("expected two root fields, got %#v", result.RootFields)
	}

	feature := result.RootFields[0]
	if feature.EntityType != "feature" || feature.PageInfo.Limit != 1 || feature.PageInfo.Offset != 1 || feature.PageInfo.Returned != 1 || !feature.PageInfo.HasMore || feature.PageInfo.NextOffset != 2 {
		t.Fatalf("unexpected feature page info: %#v", feature.PageInfo)
	}
	if feature.Items[0]["id"] != "FEAT-2" || !reflect.DeepEqual(feature.PageInfo.EffectiveSort, []string{"id:desc"}) {
		t.Fatalf("unexpected feature items/sort: %#v", feature)
	}

	service := result.RootFields[1]
	if service.EntityType != "service" || service.PageInfo.Limit != 1 || service.PageInfo.Offset != 0 || service.PageInfo.Returned != 1 || !service.PageInfo.HasMore || service.PageInfo.NextOffset != 1 {
		t.Fatalf("unexpected service page info: %#v", service.PageInfo)
	}
	if service.Items[0]["id"] != "SVC-1" || !reflect.DeepEqual(service.PageInfo.EffectiveSort, []string{"id:asc"}) {
		t.Fatalf("unexpected service items/sort: %#v", service)
	}
}

func newTestRootPlan(entityType string, limit int, offset int, sort []model.SortTerm) model.RootPlan {
	return model.RootPlan{
		EntityType:    entityType,
		Limit:         limit,
		Offset:        offset,
		EffectiveSort: sort,
	}
}
