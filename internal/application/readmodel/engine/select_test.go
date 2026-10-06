package engine

import (
	"reflect"
	"testing"

	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/internal/testsupport"
)

func TestBuildSelectTree_RejectsUnknownSelector(t *testing.T) {
	index := testsupport.NewCapability()
	_, err := buildSelectTree([]string{"meta.unknown"}, index, []string{"feature", "service"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProjectEntity_ObjectSelectorAndMissingSection(t *testing.T) {
	index := testsupport.NewCapability()
	tree, err := buildSelectTree([]string{"type", "id", "refs.owner", "content.sections.summary"}, index, []string{"feature", "service"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entity := map[string]any{
		"type": "feature",
		"id":   "FEAT-1",
		"refs": map[string]any{
			"owner": map[string]any{"type": "service", "id": "SVC-1", "slug": "api", "resolved": true},
		},
		"content": map[string]any{
			"sections": map[string]any{},
		},
	}

	projected := projectEntity(entity, tree)
	expected := map[string]any{
		"type": "feature",
		"id":   "FEAT-1",
		"refs": map[string]any{
			"owner": map[string]any{"type": "service", "id": "SVC-1", "slug": "api", "resolved": true},
		},
		"content": map[string]any{
			"sections": map[string]any{"summary": nil},
		},
	}

	if !reflect.DeepEqual(projected, expected) {
		t.Fatalf("projection mismatch:\nexpected: %#v\nactual: %#v", expected, projected)
	}
}

func TestProjectEntity_OverlappingSelectorsMerged(t *testing.T) {
	index := testsupport.NewCapability()
	tree, err := buildSelectTree([]string{"meta.status", "meta.score"}, index, []string{"feature", "service"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entity := map[string]any{"meta": map[string]any{"status": "active", "score": 9.5}}
	projected := projectEntity(entity, tree)

	expected := map[string]any{"meta": map[string]any{"status": "active", "score": 9.5}}
	if !reflect.DeepEqual(projected, expected) {
		t.Fatalf("projection mismatch:\nexpected: %#v\nactual: %#v", expected, projected)
	}
}
