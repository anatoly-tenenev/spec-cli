package engine

import (
	"testing"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
)

func TestEvaluateFilter_MissingValueSemantics(t *testing.T) {
	view := map[string]any{"meta": map[string]any{}}

	eq := model.FilterNode{Kind: model.FilterNodeLeaf, Field: "meta.status", Op: "eq", Value: "active", HasValue: true, Spec: model.SchemaFieldSpec{Path: "meta.status", Kind: model.FieldKindString}}
	if EvaluateFilter(eq, view) {
		t.Fatal("eq with missing field must be false")
	}

	exists := model.FilterNode{Kind: model.FilterNodeLeaf, Field: "meta.status", Op: "exists", HasValue: false, Spec: model.SchemaFieldSpec{Path: "meta.status", Kind: model.FieldKindString}}
	if EvaluateFilter(exists, view) {
		t.Fatal("exists with missing field must be false")
	}

	notExists := model.FilterNode{Kind: model.FilterNodeLeaf, Field: "meta.status", Op: "not_exists", HasValue: false, Spec: model.SchemaFieldSpec{Path: "meta.status", Kind: model.FieldKindString}}
	if !EvaluateFilter(notExists, view) {
		t.Fatal("not_exists with missing field must be true")
	}
}

func TestEvaluateFilter_ContainsStringAndArray(t *testing.T) {
	view := map[string]any{
		"content": map[string]any{
			"sections": map[string]any{
				"summary": "retry with backoff",
			},
		},
		"meta": map[string]any{"tags": []any{"core", "billing"}},
	}

	containsText := model.FilterNode{Kind: model.FilterNodeLeaf, Field: "content.sections.summary", Op: "contains", Value: "backoff", HasValue: true, Spec: model.SchemaFieldSpec{Path: "content.sections.summary", Kind: model.FieldKindString}}
	if !EvaluateFilter(containsText, view) {
		t.Fatal("string contains expected true")
	}

	containsArray := model.FilterNode{Kind: model.FilterNodeLeaf, Field: "meta.tags", Op: "contains", Value: "billing", HasValue: true, Spec: model.SchemaFieldSpec{Path: "meta.tags", Kind: model.FieldKindArray}}
	if !EvaluateFilter(containsArray, view) {
		t.Fatal("array contains expected true")
	}
}

func TestEvaluateFilter_DateAndLogicalNodes(t *testing.T) {
	view := map[string]any{
		"type":         "feature",
		"updated_date": "2026-03-10",
		"meta":         map[string]any{"status": "active"},
	}

	left := model.FilterNode{Kind: model.FilterNodeLeaf, Field: "updated_date", Op: "gte", Value: "2026-03-01", HasValue: true, Spec: model.SchemaFieldSpec{Path: "updated_date", Kind: model.FieldKindDate}}
	right := model.FilterNode{Kind: model.FilterNodeLeaf, Field: "meta.status", Op: "eq", Value: "active", HasValue: true, Spec: model.SchemaFieldSpec{Path: "meta.status", Kind: model.FieldKindString}}
	notNode := model.FilterNode{Kind: model.FilterNodeNot, Filter: &model.FilterNode{Kind: model.FilterNodeLeaf, Field: "type", Op: "eq", Value: "service", HasValue: true, Spec: model.SchemaFieldSpec{Path: "type", Kind: model.FieldKindString}}}

	root := model.FilterNode{Kind: model.FilterNodeAnd, Filters: []model.FilterNode{left, right, notNode}}
	if !EvaluateFilter(root, view) {
		t.Fatal("expected complex logical filter to be true")
	}
}
