package compile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompilerCachesWithinProcess(t *testing.T) {
	schemaPath := writeSchema(t, "entity:\n  service:\n    idPrefix: SVC\n    pathTemplate: services/index.md\n")

	compiler := NewCompiler()
	first := compiler.Compile(schemaPath, "spec.schema.yaml")
	if !first.Valid {
		t.Fatalf("expected first compile to be valid, got %#v", first.Issues)
	}

	if err := os.WriteFile(schemaPath, []byte("entity: []\n"), 0o644); err != nil {
		t.Fatalf("rewrite schema file: %v", err)
	}

	second := compiler.Compile(schemaPath, "spec.schema.yaml")
	if !second.Valid {
		t.Fatalf("expected cached compile result, got invalid %#v", second.Issues)
	}
}

func writeSchema(t *testing.T, content string) string {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "spec.schema.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write schema fixture: %v", err)
	}
	return path
}
