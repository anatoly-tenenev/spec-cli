package engine

import (
	"testing"

	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func TestBindWhereNode_ValidEnumEq(t *testing.T) {
	index := newEngineTestIndex()
	raw, err := parseWhereJSON(`{"field":"meta.status","op":"eq","value":"active"}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	bound, bindErr := bindWhereNode(raw, index)
	if bindErr != nil {
		t.Fatalf("unexpected bind error: %v", bindErr)
	}
	if bound.Spec.Path != "meta.status" {
		t.Fatalf("unexpected spec path: %s", bound.Spec.Path)
	}
}

func TestBindWhereNode_RejectsUnknownField(t *testing.T) {
	index := newEngineTestIndex()
	raw, err := parseWhereJSON(`{"field":"meta.unknown","op":"eq","value":"x"}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	_, bindErr := bindWhereNode(raw, index)
	if bindErr == nil {
		t.Fatal("expected bind error")
	}
	if bindErr.Code != domainerrors.CodeInvalidQuery {
		t.Fatalf("unexpected code: %s", bindErr.Code)
	}
}

func TestBindWhereNode_RejectsTypeMismatch(t *testing.T) {
	index := newEngineTestIndex()
	raw, err := parseWhereJSON(`{"field":"meta.score","op":"eq","value":"high"}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	_, bindErr := bindWhereNode(raw, index)
	if bindErr == nil {
		t.Fatal("expected bind error")
	}
}

func TestBindWhereNode_RejectsEnumMismatch(t *testing.T) {
	index := newEngineTestIndex()
	raw, err := parseWhereJSON(`{"field":"meta.status","op":"eq","value":"ACTIVE"}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	_, bindErr := bindWhereNode(raw, index)
	if bindErr == nil {
		t.Fatal("expected bind error")
	}
}

func TestBindWhereNode_RejectsInvalidDateValueForRange(t *testing.T) {
	index := newEngineTestIndex()
	raw, err := parseWhereJSON(`{"field":"updated_date","op":"gte","value":"2026/03/01"}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	_, bindErr := bindWhereNode(raw, index)
	if bindErr == nil {
		t.Fatal("expected bind error")
	}
}

func TestBindWhereNode_RejectsContentRawField(t *testing.T) {
	index := newEngineTestIndex()
	raw, err := parseWhereJSON(`{"field":"content.raw","op":"contains","value":"retry"}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, bindErr := bindWhereNode(raw, index)
	if bindErr == nil {
		t.Fatal("expected bind error")
	}
	if bindErr.Code != domainerrors.CodeInvalidQuery {
		t.Fatalf("unexpected code: %s", bindErr.Code)
	}
	if bindErr.Details["reason"] != "forbidden_field" {
		t.Fatalf("unexpected reason: %#v", bindErr.Details["reason"])
	}
	if bindErr.Details["field"] != "content.raw" {
		t.Fatalf("unexpected field: %#v", bindErr.Details["field"])
	}
	if bindErr.Details["arg"] != "--where-json" {
		t.Fatalf("unexpected arg: %#v", bindErr.Details["arg"])
	}
}

func TestBindWhereNode_AllowsContentSectionContains(t *testing.T) {
	index := newEngineTestIndex()
	raw, err := parseWhereJSON(`{"field":"content.sections.summary","op":"contains","value":"retry"}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, bindErr := bindWhereNode(raw, index)
	if bindErr != nil {
		t.Fatalf("unexpected bind error: %v", bindErr)
	}
}

func TestBindWhereNode_RejectsContentSectionEq(t *testing.T) {
	index := newEngineTestIndex()
	raw, err := parseWhereJSON(`{"field":"content.sections.summary","op":"eq","value":"Retry window"}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, bindErr := bindWhereNode(raw, index)
	if bindErr == nil {
		t.Fatal("expected bind error")
	}
	if bindErr.Code != domainerrors.CodeInvalidQuery {
		t.Fatalf("unexpected code: %s", bindErr.Code)
	}
	if bindErr.Details["reason"] != "forbidden_operator_for_field" {
		t.Fatalf("unexpected reason: %#v", bindErr.Details["reason"])
	}
	if bindErr.Details["field"] != "content.sections.summary" {
		t.Fatalf("unexpected field: %#v", bindErr.Details["field"])
	}
	if bindErr.Details["operator"] != "eq" {
		t.Fatalf("unexpected operator: %#v", bindErr.Details["operator"])
	}
	if bindErr.Details["arg"] != "--where-json" {
		t.Fatalf("unexpected arg: %#v", bindErr.Details["arg"])
	}
}
