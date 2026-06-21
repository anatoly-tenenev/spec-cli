package engine

import (
	"reflect"
	"testing"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
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
