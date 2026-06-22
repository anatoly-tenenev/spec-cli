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
	if opts.ScopedLimits == nil || opts.ScopedOffsets == nil || opts.ScopedSorts == nil {
		t.Fatalf("scoped option maps must be initialized: %#v", opts)
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

func TestParse_ScopedLimit(t *testing.T) {
	opts, err := Parse([]string{"--limit", "feature=10"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Limit != 100 || opts.ScopedLimits["feature"] != 10 {
		t.Fatalf("unexpected scoped limit: %#v", opts)
	}
}

func TestParse_GlobalAndScopedLimit(t *testing.T) {
	opts, err := Parse([]string{"--limit", "50", "--limit", "feature=10"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Limit != 50 || opts.ScopedLimits["feature"] != 10 {
		t.Fatalf("unexpected limits: %#v", opts)
	}
}

func TestParse_DuplicateScopedLimit(t *testing.T) {
	_, err := Parse([]string{"--limit", "feature=10", "--limit", "feature=20"})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != domainerrors.CodeInvalidArgs {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func TestParse_ScopedOffset(t *testing.T) {
	opts, err := Parse([]string{"--offset", "service=20"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Offset != 0 || opts.ScopedOffsets["service"] != 20 {
		t.Fatalf("unexpected scoped offset: %#v", opts)
	}
}

func TestParse_DuplicateScopedOffset(t *testing.T) {
	_, err := Parse([]string{"--offset", "service=10", "--offset", "service=20"})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != domainerrors.CodeInvalidArgs {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func TestParse_InvalidScopedNumeric(t *testing.T) {
	_, err := Parse([]string{"--limit", "feature=-1"})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != domainerrors.CodeInvalidArgs {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func TestParse_EmptyScope(t *testing.T) {
	_, err := Parse([]string{"--limit", "=10"})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != domainerrors.CodeInvalidArgs {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func TestParse_EmptyScopedValue(t *testing.T) {
	_, err := Parse([]string{"--limit", "feature="})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != domainerrors.CodeInvalidArgs {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func TestParse_ScopedSort(t *testing.T) {
	opts, err := Parse([]string{"--sort", "feature=updatedDate:desc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts.Sorts) != 0 || len(opts.ScopedSorts["feature"]) != 1 {
		t.Fatalf("unexpected scoped sort: %#v", opts)
	}
	if opts.ScopedSorts["feature"][0] != (model.SortTerm{Path: "updatedDate", Direction: model.SortDirectionDesc}) {
		t.Fatalf("unexpected scoped sort term: %#v", opts.ScopedSorts["feature"][0])
	}
}

func TestParse_MultipleScopedSortTermsPreserveOrder(t *testing.T) {
	opts, err := Parse([]string{"--sort", "feature=meta.status:desc", "--sort", "feature=id:asc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []model.SortTerm{
		{Path: "meta.status", Direction: model.SortDirectionDesc},
		{Path: "id", Direction: model.SortDirectionAsc},
	}
	if len(opts.ScopedSorts["feature"]) != len(expected) {
		t.Fatalf("unexpected scoped sort terms: %#v", opts.ScopedSorts["feature"])
	}
	for idx := range expected {
		if opts.ScopedSorts["feature"][idx] != expected[idx] {
			t.Fatalf("unexpected scoped sort terms: %#v", opts.ScopedSorts["feature"])
		}
	}
}

func TestParse_InvalidScopedSortDirection(t *testing.T) {
	_, err := Parse([]string{"--sort", "feature=id:up"})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != domainerrors.CodeInvalidArgs {
		t.Fatalf("unexpected error code: %s", err.Code)
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

func TestParse_WhereEmpty(t *testing.T) {
	_, err := Parse([]string{"--where", "   "})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != domainerrors.CodeInvalidQuery {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func TestParse_WhereDuplicate(t *testing.T) {
	_, err := Parse([]string{"--where", "type == 'feature'", "--where", "id == 'FEAT-1'"})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != domainerrors.CodeInvalidArgs {
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
