package engine

import (
	"testing"

	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func TestParseWhereJSON_ValidLeaf(t *testing.T) {
	node, err := parseWhereJSON(`{"field":"meta.status","op":"eq","value":"active"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Field != "meta.status" || node.Op != "eq" || node.Value != "active" || !node.HasValue {
		t.Fatalf("unexpected node: %#v", node)
	}
}

func TestParseWhereJSON_ValidLogical(t *testing.T) {
	node, err := parseWhereJSON(`{"op":"and","filters":[{"field":"type","op":"eq","value":"feature"},{"op":"not","filter":{"field":"meta.status","op":"eq","value":"deprecated"}}]}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Kind != "and" || len(node.Filters) != 2 {
		t.Fatalf("unexpected node: %#v", node)
	}
}

func TestParseWhereJSON_InvalidJSON(t *testing.T) {
	_, err := parseWhereJSON(`oops`)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != domainerrors.CodeInvalidQuery {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func TestParseWhereJSON_RejectsMixedNodeShape(t *testing.T) {
	_, err := parseWhereJSON(`{"field":"meta.status","op":"eq","value":"active","filters":[{"field":"id","op":"eq","value":"FEAT-1"}]}`)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseWhereJSON_RejectsExistsWithValue(t *testing.T) {
	_, err := parseWhereJSON(`{"field":"meta.status","op":"exists","value":true}`)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseWhereJSON_RejectsInWithoutArrayValue(t *testing.T) {
	_, err := parseWhereJSON(`{"field":"meta.status","op":"in","value":"active"}`)
	if err == nil {
		t.Fatal("expected error")
	}
}
