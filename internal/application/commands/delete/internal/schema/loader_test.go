package schema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/delete/internal/model"
)

func TestLoadExtractsOnlyEntityRefSlots(t *testing.T) {
	tempDir := t.TempDir()
	schemaPath := filepath.Join(tempDir, "spec.schema.yaml")
	raw := `version: "0.0.4"
entity:
  feature:
    id_prefix: FEAT
    path_pattern: "features/{slug}.md"
    meta:
      fields:
        container:
          schema:
            type: entity_ref
            refTypes: [service]
        related:
          schema:
            type: array
            items:
              type: entity_ref
              refTypes: [feature]
        status:
          schema:
            type: string
        tags:
          schema:
            type: array
            items:
              type: string
`
	if err := os.WriteFile(schemaPath, []byte(raw), 0o644); err != nil {
		t.Fatalf("write schema file: %v", err)
	}

	loaded, appErr := Load(schemaPath, "spec.schema.yaml")
	if appErr != nil {
		t.Fatalf("Load returned error: %v", appErr)
	}

	slots := loaded.ReferenceSlotsByType["feature"]
	if len(slots) != 2 {
		t.Fatalf("expected 2 reference slots, got %d", len(slots))
	}

	expected := map[string]model.ReferenceSlotKind{
		"container": model.ReferenceSlotScalar,
		"related":   model.ReferenceSlotArray,
	}
	for _, slot := range slots {
		kind, ok := expected[slot.FieldName]
		if !ok {
			t.Fatalf("unexpected slot: %s", slot.FieldName)
		}
		if slot.Kind != kind {
			t.Fatalf("slot %s: expected kind %s, got %s", slot.FieldName, kind, slot.Kind)
		}
		delete(expected, slot.FieldName)
	}
	if len(expected) != 0 {
		t.Fatalf("missing expected slots: %v", expected)
	}
}
