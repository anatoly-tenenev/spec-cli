package engine

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/delete/internal/model"
	schemacapreferences "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/references"
	schemamodel "github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/responses"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func TestExecuteReverseRefChecksDeclaredSlotsOnly(t *testing.T) {
	target := model.ParsedDocument{
		PathAbs:  "/tmp/target.md",
		Type:     "service",
		ID:       "SVC-1",
		Revision: "sha256:target",
	}
	source := model.ParsedDocument{
		PathAbs:  "/tmp/source.md",
		Type:     "note",
		ID:       "NOTE-1",
		Revision: "sha256:source",
		Frontmatter: map[string]any{
			"title":     "SVC-1",
			"body_text": "SVC-1 mentioned in plain text",
			"container": "SVC-1",
		},
	}
	snapshot := model.Snapshot{
		Documents:     []model.ParsedDocument{target, source},
		TargetMatches: []model.TargetMatch{{PathAbs: target.PathAbs}},
	}

	t.Run("no declared slots means no blocking", func(t *testing.T) {
		payload, appErr := Execute(
			model.Options{ID: "SVC-1", DryRun: true},
			emptyReferencesCapability(),
			snapshot,
		)
		if appErr != nil {
			t.Fatalf("unexpected error: %v", appErr)
		}
		if payload["result_state"] != responses.ResultStateValid {
			t.Fatalf("expected valid result_state, got %#v", payload["result_state"])
		}
	})

	t.Run("declared scalar slot blocks delete", func(t *testing.T) {
		_, appErr := Execute(
			model.Options{ID: "SVC-1", DryRun: true},
			capabilityWithSourceSlots(map[string][]schemacapreferences.SourceSlot{
				"note": {
					{FieldName: "container", Cardinality: schemamodel.RefCardinalityScalar},
				},
			}),
			snapshot,
		)
		if appErr == nil {
			t.Fatalf("expected blocking error")
		}
		if appErr.Code != domainerrors.CodeDeleteBlockedByRefs {
			t.Fatalf("expected %s, got %s", domainerrors.CodeDeleteBlockedByRefs, appErr.Code)
		}
	})
}

func TestExecuteDryRunAndRealRunSharePipelineUntilCommit(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	if err := os.WriteFile(targetPath, []byte("target"), 0o644); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	snapshot := model.Snapshot{
		Documents: []model.ParsedDocument{{
			PathAbs:  targetPath,
			Type:     "service",
			ID:       "SVC-2",
			Revision: "sha256:stable",
		}},
		TargetMatches: []model.TargetMatch{{PathAbs: targetPath}},
	}
	referencesCapability := emptyReferencesCapability()

	dryPayload, dryErr := Execute(model.Options{ID: "SVC-2", DryRun: true}, referencesCapability, snapshot)
	if dryErr != nil {
		t.Fatalf("dry-run failed: %v", dryErr)
	}
	if _, statErr := os.Stat(targetPath); statErr != nil {
		t.Fatalf("dry-run must not delete target, stat err=%v", statErr)
	}

	realPayload, realErr := Execute(model.Options{ID: "SVC-2", DryRun: false}, referencesCapability, snapshot)
	if realErr != nil {
		t.Fatalf("real run failed: %v", realErr)
	}
	if _, statErr := os.Stat(targetPath); !os.IsNotExist(statErr) {
		t.Fatalf("real run must delete target, stat err=%v", statErr)
	}

	dryComparable := clonePayload(dryPayload)
	realComparable := clonePayload(realPayload)
	delete(dryComparable, "dry_run")
	delete(realComparable, "dry_run")
	if !reflect.DeepEqual(dryComparable, realComparable) {
		t.Fatalf("dry-run and real-run payloads must match except dry_run")
	}
}

func TestExecuteReverseRefsDoNotFilterByTargetType(t *testing.T) {
	targetPath := "/tmp/target-filter.md"
	capability := schemacapreferences.Build(schemamodel.CompiledSchema{
		Entities: map[string]schemamodel.EntityType{
			"feature": {
				MetaFields: map[string]schemamodel.MetaField{
					"container": {
						Value: schemamodel.ValueSpec{
							Kind: schemamodel.ValueKindEntityRef,
							Ref: &schemamodel.RefSpec{
								Cardinality:  schemamodel.RefCardinalityScalar,
								AllowedTypes: []string{"feature"},
							},
						},
					},
				},
			},
			"service": {},
		},
	})

	_, appErr := Execute(
		model.Options{ID: "SVC-1", DryRun: true},
		capability,
		model.Snapshot{
			Documents: []model.ParsedDocument{
				{
					PathAbs:  targetPath,
					Type:     "service",
					ID:       "SVC-1",
					Revision: "sha256:target",
				},
				{
					PathAbs:  "/tmp/source-filter.md",
					Type:     "feature",
					ID:       "FEAT-1",
					Revision: "sha256:source",
					Frontmatter: map[string]any{
						"container": "SVC-1",
					},
				},
			},
			TargetMatches: []model.TargetMatch{{PathAbs: targetPath}},
		},
	)
	assertErrorCode(t, appErr, domainerrors.CodeDeleteBlockedByRefs)
}

func TestExecuteReturnsStableDomainCodes(t *testing.T) {
	t.Run("ENTITY_NOT_FOUND", func(t *testing.T) {
		_, appErr := Execute(
			model.Options{ID: "SVC-1", DryRun: true},
			emptyReferencesCapability(),
			model.Snapshot{},
		)
		assertErrorCode(t, appErr, domainerrors.CodeEntityNotFound)
	})

	t.Run("AMBIGUOUS_ENTITY_ID", func(t *testing.T) {
		_, appErr := Execute(
			model.Options{ID: "SVC-1", DryRun: true},
			emptyReferencesCapability(),
			model.Snapshot{
				TargetMatches: []model.TargetMatch{
					{PathAbs: "/tmp/a.md"},
					{PathAbs: "/tmp/b.md"},
				},
			},
		)
		assertErrorCode(t, appErr, domainerrors.CodeAmbiguousEntityID)
	})

	t.Run("REVISION_UNAVAILABLE", func(t *testing.T) {
		targetPath := "/tmp/target-revision-unavailable.md"
		_, appErr := Execute(
			model.Options{ID: "SVC-1", DryRun: true},
			emptyReferencesCapability(),
			model.Snapshot{
				Documents: []model.ParsedDocument{{
					PathAbs:  targetPath,
					Type:     "service",
					ID:       "SVC-1",
					Revision: "",
				}},
				TargetMatches: []model.TargetMatch{{PathAbs: targetPath}},
			},
		)
		assertErrorCode(t, appErr, domainerrors.CodeRevisionUnavailable)
	})

	t.Run("CONCURRENCY_CONFLICT", func(t *testing.T) {
		targetPath := "/tmp/target-concurrency.md"
		_, appErr := Execute(
			model.Options{ID: "SVC-1", ExpectRevision: "sha256:expected", DryRun: true},
			emptyReferencesCapability(),
			model.Snapshot{
				Documents: []model.ParsedDocument{{
					PathAbs:  targetPath,
					Type:     "service",
					ID:       "SVC-1",
					Revision: "sha256:actual",
				}},
				TargetMatches: []model.TargetMatch{{PathAbs: targetPath}},
			},
		)
		assertErrorCode(t, appErr, domainerrors.CodeConcurrencyConflict)
	})

	t.Run("DELETE_BLOCKED_BY_REFERENCES", func(t *testing.T) {
		targetPath := "/tmp/target-blocked.md"
		_, appErr := Execute(
			model.Options{ID: "SVC-1", DryRun: true},
			capabilityWithSourceSlots(map[string][]schemacapreferences.SourceSlot{
				"feature": {
					{FieldName: "container", Cardinality: schemamodel.RefCardinalityScalar},
				},
			}),
			model.Snapshot{
				Documents: []model.ParsedDocument{
					{
						PathAbs:  targetPath,
						Type:     "service",
						ID:       "SVC-1",
						Revision: "sha256:target",
					},
					{
						PathAbs:  "/tmp/source-blocked.md",
						Type:     "feature",
						ID:       "FEAT-1",
						Revision: "sha256:source",
						Frontmatter: map[string]any{
							"container": "SVC-1",
						},
					},
				},
				TargetMatches: []model.TargetMatch{{PathAbs: targetPath}},
			},
		)
		assertErrorCode(t, appErr, domainerrors.CodeDeleteBlockedByRefs)
	})

	t.Run("WRITE_FAILED", func(t *testing.T) {
		tempDir := t.TempDir()
		missingPath := filepath.Join(tempDir, "missing-target.md")
		_, appErr := Execute(
			model.Options{ID: "SVC-1", DryRun: false},
			emptyReferencesCapability(),
			model.Snapshot{
				Documents: []model.ParsedDocument{{
					PathAbs:  missingPath,
					Type:     "service",
					ID:       "SVC-1",
					Revision: "sha256:target",
				}},
				TargetMatches: []model.TargetMatch{{PathAbs: missingPath}},
			},
		)
		assertErrorCode(t, appErr, domainerrors.CodeWriteFailed)
	})
}

func assertErrorCode(t *testing.T, appErr *domainerrors.AppError, expected domainerrors.Code) {
	t.Helper()

	if appErr == nil {
		t.Fatalf("expected error code %s, got success", expected)
	}
	if appErr.Code != expected {
		t.Fatalf("expected error code %s, got %s", expected, appErr.Code)
	}
}

func clonePayload(source map[string]any) map[string]any {
	cloned := map[string]any{}
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func emptyReferencesCapability() schemacapreferences.Capability {
	return schemacapreferences.Capability{
		InboundByTargetType: map[string][]schemacapreferences.InboundSlot{},
		SlotsBySourceType:   map[string][]schemacapreferences.SourceSlot{},
	}
}

func capabilityWithSourceSlots(
	slotsBySource map[string][]schemacapreferences.SourceSlot,
) schemacapreferences.Capability {
	return schemacapreferences.Capability{
		InboundByTargetType: map[string][]schemacapreferences.InboundSlot{},
		SlotsBySourceType:   slotsBySource,
	}
}
