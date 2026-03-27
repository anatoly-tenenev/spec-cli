package options

import (
	"testing"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func TestParse_Defaults(t *testing.T) {
	opts, err := Parse(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Limit != 100 || opts.Offset != 0 {
		t.Fatalf("unexpected defaults: %#v", opts)
	}
}

func TestParse_CollectsRepeatableFlags(t *testing.T) {
	opts, err := Parse([]string{"--type", "feature", "--type", "service", "--select", "id", "--select", "meta.status", "--sort", "updatedDate:desc", "--limit", "50", "--offset", "2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts.TypeFilters) != 2 || len(opts.Selects) != 2 || len(opts.Sorts) != 1 {
		t.Fatalf("unexpected parsed options: %#v", opts)
	}
	if opts.Sorts[0] != (model.SortTerm{Path: "updatedDate", Direction: model.SortDirectionDesc}) {
		t.Fatalf("unexpected sort term: %#v", opts.Sorts[0])
	}
}

func TestParse_InvalidLimit(t *testing.T) {
	_, err := Parse([]string{"--limit", "-1"})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != domainerrors.CodeInvalidArgs {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func TestParse_InvalidSortDirection(t *testing.T) {
	_, err := Parse([]string{"--sort", "id:up"})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != domainerrors.CodeInvalidArgs {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func TestParse_WhereJSONEmpty(t *testing.T) {
	_, err := Parse([]string{"--where-json", "   "})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != domainerrors.CodeInvalidQuery {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func TestParse_UnknownOption(t *testing.T) {
	_, err := Parse([]string{"--unknown"})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != domainerrors.CodeInvalidArgs {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func TestParse_HelpFlagUnsupported(t *testing.T) {
	_, err := Parse([]string{"--help"})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != domainerrors.CodeInvalidArgs {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}
