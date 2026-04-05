package add

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
)

func TestHandleCompileFailureIncludesTopLevelSchema(t *testing.T) {
	workspacePath := t.TempDir()
	schemaPath := filepath.Join(t.TempDir(), "spec.schema.yaml")
	if err := os.WriteFile(schemaPath, []byte(""), 0o644); err != nil {
		t.Fatalf("write schema fixture: %v", err)
	}

	handler := NewHandler(func() time.Time {
		return time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	})

	output, appErr := handler.Handle(context.Background(), requests.Command{
		Args: []string{"--type", "note", "--slug", "compile-fail", "--dry-run"},
		Global: requests.GlobalOptions{
			Workspace:  workspacePath,
			SchemaPath: schemaPath,
			Format:     requests.FormatJSON,
		},
	})
	if appErr != nil {
		t.Fatalf("expected command output, got appErr: %#v", appErr)
	}

	if output.ExitCode != 4 {
		t.Fatalf("unexpected exit code: %d", output.ExitCode)
	}

	errorPayload := requireMap(t, output.JSON["error"], "error")
	if got := fmt.Sprint(errorPayload["code"]); got != "SCHEMA_PARSE_ERROR" {
		t.Fatalf("unexpected error code: %#v", got)
	}
	if _, hasDetails := errorPayload["details"]; hasDetails {
		t.Fatalf("compile error must not duplicate schema diagnostics in error.details")
	}

	schemaPayload := requireMap(t, output.JSON["schema"], "schema")
	if valid, ok := schemaPayload["valid"].(bool); !ok || valid {
		t.Fatalf("expected schema.valid=false, got %#v", schemaPayload["valid"])
	}
}

func TestHandleUnknownTypeIncludesTopLevelSchemaAfterCompile(t *testing.T) {
	workspacePath := t.TempDir()
	schemaPath := filepath.Join(t.TempDir(), "spec.schema.yaml")
	writeFile(t, schemaPath, `
version: "0.0.4"
entity:
  note:
    idPrefix: NOTE
    pathTemplate: "notes/${slug}.md"
`)

	handler := NewHandler(func() time.Time {
		return time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	})

	output, appErr := handler.Handle(context.Background(), requests.Command{
		Args: []string{"--type", "unknown", "--slug", "x", "--dry-run"},
		Global: requests.GlobalOptions{
			Workspace:  workspacePath,
			SchemaPath: schemaPath,
			Format:     requests.FormatJSON,
		},
	})
	if appErr != nil {
		t.Fatalf("expected command output, got appErr: %#v", appErr)
	}

	if output.ExitCode != 2 {
		t.Fatalf("unexpected exit code: %d", output.ExitCode)
	}

	errorPayload := requireMap(t, output.JSON["error"], "error")
	if got := fmt.Sprint(errorPayload["code"]); got != "ENTITY_TYPE_UNKNOWN" {
		t.Fatalf("unexpected error code: %#v", got)
	}

	schemaPayload := requireMap(t, output.JSON["schema"], "schema")
	if valid, ok := schemaPayload["valid"].(bool); !ok || !valid {
		t.Fatalf("expected schema.valid=true, got %#v", schemaPayload["valid"])
	}
}

func TestHandleSuccessIncludesTopLevelSchema(t *testing.T) {
	workspacePath := t.TempDir()
	schemaPath := filepath.Join(t.TempDir(), "spec.schema.yaml")
	writeFile(t, schemaPath, `
version: "0.0.4"
entity:
  note:
    idPrefix: NOTE
    pathTemplate: "notes/${slug}.md"
`)

	handler := NewHandler(func() time.Time {
		return time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	})

	output, appErr := handler.Handle(context.Background(), requests.Command{
		Args: []string{"--type", "note", "--slug", "cli-baseline", "--dry-run"},
		Global: requests.GlobalOptions{
			Workspace:  workspacePath,
			SchemaPath: schemaPath,
			Format:     requests.FormatJSON,
		},
	})
	if appErr != nil {
		t.Fatalf("expected success output, got appErr: %#v", appErr)
	}
	if output.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %d", output.ExitCode)
	}
	if got := fmt.Sprint(output.JSON["result_state"]); got != "valid" {
		t.Fatalf("unexpected result_state: %#v", got)
	}

	schemaPayload := requireMap(t, output.JSON["schema"], "schema")
	if valid, ok := schemaPayload["valid"].(bool); !ok || !valid {
		t.Fatalf("expected schema.valid=true, got %#v", schemaPayload["valid"])
	}
	if created, ok := output.JSON["created"].(bool); !ok || !created {
		t.Fatalf("expected created=true, got %#v", output.JSON["created"])
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func requireMap(t *testing.T, value any, label string) map[string]any {
	t.Helper()

	typed, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s must be an object, got %#v", label, value)
	}
	return typed
}
