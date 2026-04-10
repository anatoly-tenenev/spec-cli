package delete

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
)

func TestHandleCompileFailureIncludesTopLevelSchema(t *testing.T) {
	workspacePath := t.TempDir()
	schemaPath := filepath.Join(t.TempDir(), "spec.schema.yaml")
	if err := os.WriteFile(schemaPath, []byte(""), 0o644); err != nil {
		t.Fatalf("write schema fixture: %v", err)
	}

	handler := NewHandler()

	output, appErr := handler.Handle(context.Background(), requests.Command{
		Args: []string{"--id", "SVC-1"},
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

func TestHandlePostCompileDomainErrorIncludesTopLevelSchema(t *testing.T) {
	workspacePath := t.TempDir()
	schemaPath := filepath.Join(t.TempDir(), "spec.schema.yaml")
	writeFile(t, schemaPath, `
version: "0.0.7"
entity:
  service:
    idPrefix: SVC
    pathTemplate: "services/${slug}/index.md"
`)

	handler := NewHandler()

	output, appErr := handler.Handle(context.Background(), requests.Command{
		Args: []string{"--id", "SVC-1"},
		Global: requests.GlobalOptions{
			Workspace:  workspacePath,
			SchemaPath: schemaPath,
			Format:     requests.FormatJSON,
		},
	})
	if appErr != nil {
		t.Fatalf("expected command output, got appErr: %#v", appErr)
	}

	if output.ExitCode != 1 {
		t.Fatalf("unexpected exit code: %d", output.ExitCode)
	}

	errorPayload := requireMap(t, output.JSON["error"], "error")
	if got := fmt.Sprint(errorPayload["code"]); got != "ENTITY_NOT_FOUND" {
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
version: "0.0.7"
entity:
  service:
    idPrefix: SVC
    pathTemplate: "services/${slug}/index.md"
`)
	writeFile(t, filepath.Join(workspacePath, "services", "payments", "index.md"), `---
type: service
id: SVC-1
slug: payments
---

## Summary {#summary}
Payments service.
`)

	handler := NewHandler()

	output, appErr := handler.Handle(context.Background(), requests.Command{
		Args: []string{"--id", "SVC-1", "--dry-run"},
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
	if deleted, ok := output.JSON["deleted"].(bool); !ok || !deleted {
		t.Fatalf("expected deleted=true, got %#v", output.JSON["deleted"])
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
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
