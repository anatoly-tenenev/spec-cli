package engine

import (
	"reflect"
	"testing"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func TestBuildPlan_ActiveTypeSetOrder(t *testing.T) {
	capability := newEngineTestCapability()

	defaultPlan, err := BuildPlan(model.Options{Limit: 100}, capability)
	if err != nil {
		t.Fatalf("unexpected default plan error: %v", err)
	}
	if !reflect.DeepEqual(defaultPlan.ActiveTypeSet, []string{"service", "feature"}) {
		t.Fatalf("default active type order must follow capability order, got %#v", defaultPlan.ActiveTypeSet)
	}

	filteredPlan, err := BuildPlan(model.Options{TypeFilters: []string{"feature", "service", "feature"}, Limit: 100}, capability)
	if err != nil {
		t.Fatalf("unexpected filtered plan error: %v", err)
	}
	if !reflect.DeepEqual(filteredPlan.ActiveTypeSet, []string{"feature", "service"}) {
		t.Fatalf("filtered active type order must follow CLI order with dedupe, got %#v", filteredPlan.ActiveTypeSet)
	}
	if len(filteredPlan.RootPlans) != 2 || filteredPlan.RootPlans[0].EntityType != "feature" || filteredPlan.RootPlans[1].EntityType != "service" {
		t.Fatalf("root plans must follow active type order, got %#v", filteredPlan.RootPlans)
	}
}

func TestBuildPlan_ExplicitSelectDoesNotAddType(t *testing.T) {
	capability := newEngineTestCapability()

	plan, err := BuildPlan(model.Options{TypeFilters: []string{"feature"}, Selects: []string{"id"}, Limit: 100}, capability)
	if err != nil {
		t.Fatalf("unexpected plan error: %v", err)
	}

	projected := ProjectEntity(map[string]any{"type": "feature", "id": "FEAT-1", "slug": "retry-window"}, plan.SelectTree)
	expected := map[string]any{"id": "FEAT-1"}
	if !reflect.DeepEqual(projected, expected) {
		t.Fatalf("explicit select must not add type:\nexpected=%#v\nactual=%#v", expected, projected)
	}
}

func TestBuildPlan_GlobalLimitOffsetCopiedToEveryRootPlan(t *testing.T) {
	capability := newEngineTestCapability()

	plan, err := BuildPlan(model.Options{Limit: 25, Offset: 3}, capability)
	if err != nil {
		t.Fatalf("unexpected plan error: %v", err)
	}
	if len(plan.RootPlans) != 2 {
		t.Fatalf("unexpected root plans: %#v", plan.RootPlans)
	}
	for _, rootPlan := range plan.RootPlans {
		if rootPlan.Limit != 25 || rootPlan.Offset != 3 {
			t.Fatalf("global pagination not copied to root plan: %#v", rootPlan)
		}
	}
}

func TestBuildPlan_ScopedLimitOffsetOverrideOnlyMatchingRoot(t *testing.T) {
	capability := newEngineTestCapability()

	plan, err := BuildPlan(model.Options{
		TypeFilters:   []string{"feature", "service"},
		Limit:         100,
		ScopedLimits:  map[string]int{"feature": 10},
		Offset:        0,
		ScopedOffsets: map[string]int{"service": 20},
	}, capability)
	if err != nil {
		t.Fatalf("unexpected plan error: %v", err)
	}

	feature := findRootPlan(t, plan, "feature")
	if feature.Limit != 10 || feature.Offset != 0 {
		t.Fatalf("unexpected feature plan: %#v", feature)
	}
	service := findRootPlan(t, plan, "service")
	if service.Limit != 100 || service.Offset != 20 {
		t.Fatalf("unexpected service plan: %#v", service)
	}
}

func TestBuildPlan_ScopedSortReplacesGlobalOnlyForMatchingRoot(t *testing.T) {
	capability := newEngineTestCapability()

	plan, err := BuildPlan(model.Options{
		TypeFilters: []string{"feature", "service"},
		Sorts:       []model.SortTerm{{Path: "updatedDate", Direction: model.SortDirectionDesc}},
		ScopedSorts: map[string][]model.SortTerm{
			"feature": {{Path: "refs.owner.id", Direction: model.SortDirectionAsc}},
		},
		Limit:  100,
		Offset: 0,
	}, capability)
	if err != nil {
		t.Fatalf("unexpected plan error: %v", err)
	}

	feature := findRootPlan(t, plan, "feature")
	if !reflect.DeepEqual(feature.EffectiveSort, []model.SortTerm{
		{Path: "refs.owner.id", Direction: model.SortDirectionAsc},
		{Path: "id", Direction: model.SortDirectionAsc},
	}) {
		t.Fatalf("unexpected scoped feature sort: %#v", feature.EffectiveSort)
	}

	service := findRootPlan(t, plan, "service")
	if !reflect.DeepEqual(service.EffectiveSort, []model.SortTerm{
		{Path: "updatedDate", Direction: model.SortDirectionDesc},
		{Path: "id", Direction: model.SortDirectionAsc},
	}) {
		t.Fatalf("unexpected global service sort: %#v", service.EffectiveSort)
	}
}

func TestBuildPlan_UnknownScope(t *testing.T) {
	capability := newEngineTestCapability()

	_, err := BuildPlan(model.Options{
		Limit:        100,
		ScopedLimits: map[string]int{"unknown": 10},
	}, capability)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != domainerrors.CodeEntityTypeUnknown {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func TestBuildPlan_InactiveScope(t *testing.T) {
	capability := newEngineTestCapability()

	_, err := BuildPlan(model.Options{
		TypeFilters:  []string{"feature"},
		Limit:        100,
		ScopedLimits: map[string]int{"service": 10},
	}, capability)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != domainerrors.CodeInvalidArgs {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func findRootPlan(t *testing.T, plan model.QueryPlan, entityType string) model.RootPlan {
	t.Helper()
	for _, rootPlan := range plan.RootPlans {
		if rootPlan.EntityType == entityType {
			return rootPlan
		}
	}
	t.Fatalf("root plan not found for %s: %#v", entityType, plan.RootPlans)
	return model.RootPlan{}
}
