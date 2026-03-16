package version

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/responses"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func TestHandleReturnsJSONPayload(t *testing.T) {
	handler := newHandler(func() (string, error) {
		return "1.2.3", nil
	})

	out, err := handler.Handle(context.Background(), requests.Command{
		Name: "version",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.JSON["result_state"] != responses.ResultStateValid {
		t.Fatalf("expected result_state valid, got %#v", out.JSON["result_state"])
	}
	if out.JSON["version"] != "1.2.3" {
		t.Fatalf("expected version 1.2.3, got %#v", out.JSON["version"])
	}
}

func TestHandleReturnsInvalidArgsOnCommandFormatOption(t *testing.T) {
	handler := newHandler(func() (string, error) {
		return "2.0.0", nil
	})

	_, err := handler.Handle(context.Background(), requests.Command{
		Name:   "version",
		Args:   []string{"--format", "json"},
		Global: requests.GlobalOptions{Format: requests.FormatJSON},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Code != domainerrors.CodeInvalidArgs {
		t.Fatalf("expected INVALID_ARGS, got %s", err.Code)
	}
	if err.Message != "unknown version option: --format" {
		t.Fatalf("unexpected error message: %q", err.Message)
	}
}

func TestHandleReturnsInvalidArgsOnUnknownOption(t *testing.T) {
	handler := newHandler(func() (string, error) {
		return "dev", nil
	})

	_, err := handler.Handle(context.Background(), requests.Command{
		Name: "version",
		Args: []string{"--unknown"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Code != domainerrors.CodeInvalidArgs {
		t.Fatalf("expected INVALID_ARGS, got %s", err.Code)
	}
}

func TestHandleReturnsInternalErrorWhenProviderFails(t *testing.T) {
	handler := newHandler(func() (string, error) {
		return "", errors.New("boom")
	})

	_, err := handler.Handle(context.Background(), requests.Command{Name: "version"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Code != domainerrors.CodeInternalError {
		t.Fatalf("expected INTERNAL_ERROR, got %s", err.Code)
	}
	if err.ExitCode != 5 {
		t.Fatalf("expected exit code 5, got %d", err.ExitCode)
	}
	if !strings.Contains(err.Message, "failed to resolve build version") {
		t.Fatalf("unexpected error message: %q", err.Message)
	}
}
