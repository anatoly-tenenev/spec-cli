package engine

import (
	"testing"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func TestCompileWhereExpression_Valid(t *testing.T) {
	index := newEngineTestCapability()
	compiled, err := compileWhereExpression("meta.status == 'active'", index, []string{"feature", "service"})
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}
	if compiled == nil || compiled.Query == nil {
		t.Fatal("compiled where plan is nil")
	}
}

func TestCompileWhereExpression_RejectsContentRoot(t *testing.T) {
	index := newEngineTestCapability()
	_, err := compileWhereExpression("keys(content)", index, []string{"feature", "service"})
	if err == nil {
		t.Fatal("expected compile error")
	}
	if err.Code != domainerrors.CodeInvalidQuery {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func TestCompileWhereExpression_AllowsContentRaw(t *testing.T) {
	index := newEngineTestCapability()
	compiled, err := compileWhereExpression("contains(content.raw || '', 'x')", index, []string{"feature", "service"})
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}
	if compiled == nil || compiled.Query == nil {
		t.Fatal("compiled where plan is nil")
	}
}

func TestCompileWhereExpression_RejectsUnknownContentPath(t *testing.T) {
	index := newEngineTestCapability()
	_, err := compileWhereExpression("content.unknown == 'x'", index, []string{"feature", "service"})
	if err == nil {
		t.Fatal("expected compile error")
	}
	if err.Code != domainerrors.CodeInvalidQuery {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func TestCompileWhereExpression_RejectsMetaEntityRef(t *testing.T) {
	index := newEngineTestCapability()
	_, err := compileWhereExpression("meta.owner == 'SVC-1'", index, []string{"feature"})
	if err == nil {
		t.Fatal("expected compile error")
	}
	if err.Code != domainerrors.CodeInvalidQuery {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func TestExecute_WhereContentRaw(t *testing.T) {
	index := newEngineTestCapability()
	tree, err := buildSelectTree([]string{"id"}, index, []string{"feature"})
	if err != nil {
		t.Fatalf("select build error: %v", err)
	}
	wherePlan, whereErr := compileWhereExpression("contains(content.raw || '', 'backoff')", index, []string{"feature"})
	if whereErr != nil {
		t.Fatalf("where compile error: %v", whereErr)
	}

	plan := model.QueryPlan{
		SelectTree: tree,
		RootPlans: []model.RootPlan{
			newTestRootPlan("feature", 100, 0, []model.SortTerm{{Path: "id", Direction: model.SortDirectionAsc}}),
		},
		Where:         wherePlan,
		ActiveTypeSet: []string{"feature"},
	}

	entities := []model.EntityView{
		{
			Type: "feature",
			ID:   "FEAT-1",
			View: map[string]any{"id": "FEAT-1"},
			WhereContext: map[string]any{
				"content": map[string]any{"raw": "Retry window uses backoff."},
			},
		},
		{
			Type: "feature",
			ID:   "FEAT-2",
			View: map[string]any{"id": "FEAT-2"},
			WhereContext: map[string]any{
				"content": map[string]any{"raw": "Login flow draft."},
			},
		},
	}

	result, execErr := Execute(plan, entities)
	if execErr != nil {
		t.Fatalf("unexpected execute error: %v", execErr)
	}
	if len(result.RootFields) != 1 || result.RootFields[0].TotalCount != 1 || len(result.RootFields[0].Items) != 1 {
		t.Fatalf("unexpected match result: %#v", result)
	}
}

func TestExecute_WhereTruthinessJMESPath(t *testing.T) {
	index := newEngineTestCapability()
	tree, err := buildSelectTree([]string{"id"}, index, []string{"feature"})
	if err != nil {
		t.Fatalf("select build error: %v", err)
	}
	wherePlan, whereErr := compileWhereExpression("meta.tags", index, []string{"feature"})
	if whereErr != nil {
		t.Fatalf("where compile error: %v", whereErr)
	}

	plan := model.QueryPlan{
		SelectTree: tree,
		RootPlans: []model.RootPlan{
			newTestRootPlan("feature", 100, 0, []model.SortTerm{{Path: "id", Direction: model.SortDirectionAsc}}),
		},
		Where:         wherePlan,
		ActiveTypeSet: []string{"feature"},
	}

	entities := []model.EntityView{
		{
			Type: "feature",
			ID:   "FEAT-1",
			View: map[string]any{"id": "FEAT-1"},
			WhereContext: map[string]any{
				"meta": map[string]any{"tags": []any{"billing"}},
			},
		},
		{
			Type: "feature",
			ID:   "FEAT-2",
			View: map[string]any{"id": "FEAT-2"},
			WhereContext: map[string]any{
				"meta": map[string]any{"tags": []any{}},
			},
		},
	}

	result, execErr := Execute(plan, entities)
	if execErr != nil {
		t.Fatalf("unexpected execute error: %v", execErr)
	}
	if len(result.RootFields) != 1 || result.RootFields[0].TotalCount != 1 || len(result.RootFields[0].Items) != 1 {
		t.Fatalf("unexpected match result: %#v", result)
	}
}

func TestExecute_WhereRuntimeErrorMappedToReadFailed(t *testing.T) {
	index := newEngineTestCapability()
	tree, err := buildSelectTree([]string{"id"}, index, []string{"feature"})
	if err != nil {
		t.Fatalf("select build error: %v", err)
	}
	wherePlan, whereErr := compileWhereExpression("length(meta.tags) > `0`", index, []string{"feature"})
	if whereErr != nil {
		t.Fatalf("where compile error: %v", whereErr)
	}

	plan := model.QueryPlan{
		SelectTree: tree,
		RootPlans: []model.RootPlan{
			newTestRootPlan("feature", 100, 0, []model.SortTerm{{Path: "id", Direction: model.SortDirectionAsc}}),
		},
		Where:         wherePlan,
		ActiveTypeSet: []string{"feature"},
	}

	entities := []model.EntityView{
		{
			Type: "feature",
			ID:   "FEAT-1",
			View: map[string]any{"id": "FEAT-1"},
			WhereContext: map[string]any{
				"meta": map[string]any{"tags": 10},
			},
		},
	}

	_, execErr := Execute(plan, entities)
	if execErr == nil {
		t.Fatal("expected runtime error")
	}
	if execErr.Code != domainerrors.CodeReadFailed {
		t.Fatalf("unexpected error code: %s", execErr.Code)
	}
}
