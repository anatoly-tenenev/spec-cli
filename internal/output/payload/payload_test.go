package payload

import (
	"reflect"
	"testing"

	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/compile"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/diagnostics"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func TestBuildSchemaPayload(t *testing.T) {
	result := compile.Result{
		Issues: []diagnostics.Issue{
			{
				Level:   diagnostics.LevelError,
				Class:   diagnostics.ClassSchemaError,
				Code:    "schema.test.error",
				Message: "test error",
				Path:    "schema",
			},
		},
		Summary: diagnostics.Summary{Errors: 1, Warnings: 2},
		Valid:   false,
	}

	payload := BuildSchemaPayload(result)

	if valid, ok := payload["valid"].(bool); !ok || valid {
		t.Fatalf("expected payload.valid=false, got %#v", payload["valid"])
	}

	summary, ok := payload["summary"].(diagnostics.Summary)
	if !ok {
		t.Fatalf("expected diagnostics.Summary type, got %#v", payload["summary"])
	}
	if summary.Errors != 1 || summary.Warnings != 2 {
		t.Fatalf("unexpected summary: %#v", summary)
	}

	issues, ok := payload["issues"].([]diagnostics.Issue)
	if !ok {
		t.Fatalf("expected []diagnostics.Issue type, got %#v", payload["issues"])
	}
	if len(issues) != 1 || issues[0].Code != "schema.test.error" {
		t.Fatalf("unexpected issues: %#v", issues)
	}
}

func TestBuildErrorPayload(t *testing.T) {
	appErr := domainerrors.New(
		domainerrors.CodeSchemaInvalid,
		"schema contains validation errors",
		map[string]any{"path": "spec.schema.yaml"},
	)

	payload := BuildErrorPayload(appErr)

	if payload["code"] != domainerrors.CodeSchemaInvalid {
		t.Fatalf("expected code %s, got %#v", domainerrors.CodeSchemaInvalid, payload["code"])
	}
	if payload["message"] != "schema contains validation errors" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
	if payload["exit_code"] != appErr.ExitCode {
		t.Fatalf("expected exit_code=%d, got %#v", appErr.ExitCode, payload["exit_code"])
	}

	expectedDetails := map[string]any{"path": "spec.schema.yaml"}
	if !reflect.DeepEqual(payload["details"], expectedDetails) {
		t.Fatalf("unexpected details: %#v", payload["details"])
	}
}
