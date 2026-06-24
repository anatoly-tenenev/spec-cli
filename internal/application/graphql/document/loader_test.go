package document

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func TestLoad_UsesInlineQueryAndVariablesJSON(t *testing.T) {
	loaded, appErr := Load(Request{
		Query:         "query { service { totalCount } }",
		VariablesJSON: `{"status":"active","limit":2}`,
	})
	if appErr != nil {
		t.Fatalf("unexpected load error: %v", appErr)
	}

	if loaded.Query != "query { service { totalCount } }" {
		t.Fatalf("unexpected query: %q", loaded.Query)
	}
	expectedVars := map[string]any{"status": "active", "limit": float64(2)}
	if !reflect.DeepEqual(loaded.Variables, expectedVars) {
		t.Fatalf("unexpected variables:\nwant=%#v\ngot=%#v", expectedVars, loaded.Variables)
	}
}

func TestLoad_FileInputsOverrideInlineInputs(t *testing.T) {
	dir := t.TempDir()
	queryPath := filepath.Join(dir, "query.graphql")
	varsPath := filepath.Join(dir, "variables.json")
	if err := os.WriteFile(queryPath, []byte("query FromFile { service { totalCount } }\n"), 0o600); err != nil {
		t.Fatalf("write query fixture: %v", err)
	}
	if err := os.WriteFile(varsPath, []byte(`{"status":"from-file"}`), 0o600); err != nil {
		t.Fatalf("write variables fixture: %v", err)
	}

	loaded, appErr := Load(Request{
		Query:         "query Inline { service { totalCount } }",
		File:          queryPath,
		VariablesJSON: `{"status":"inline"}`,
		VariablesFile: varsPath,
	})
	if appErr != nil {
		t.Fatalf("unexpected load error: %v", appErr)
	}

	if loaded.Query != "query FromFile { service { totalCount } }\n" {
		t.Fatalf("unexpected query: %q", loaded.Query)
	}
	expectedVars := map[string]any{"status": "from-file"}
	if !reflect.DeepEqual(loaded.Variables, expectedVars) {
		t.Fatalf("unexpected variables:\nwant=%#v\ngot=%#v", expectedVars, loaded.Variables)
	}
}

func TestLoad_RejectsVariablesRootThatIsNotObject(t *testing.T) {
	_, appErr := Load(Request{
		Query:         "query { service { totalCount } }",
		VariablesJSON: `["not-object"]`,
	})
	if appErr == nil {
		t.Fatal("expected variables error")
	}
	if appErr.Code != domainerrors.CodeInvalidQuery {
		t.Fatalf("unexpected error code: %s", appErr.Code)
	}
}
