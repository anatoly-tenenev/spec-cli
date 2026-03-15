package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildSnapshotIgnoresUnparseableButKeepsDuplicateTargetMatches(t *testing.T) {
	root := t.TempDir()

	writeFixtureFile(t, filepath.Join(root, "features", "one.md"), `---
type: feature
id: FEAT-1
slug: one
status: draft
container: SVC-1
---

## Summary {#summary}
Feature one.
`)
	writeFixtureFile(t, filepath.Join(root, "features", "two.md"), `---
type: feature
id: FEAT-1
slug: two
status: active
container: SVC-1
---

## Summary {#summary}
Feature two.
`)
	writeFixtureFile(t, filepath.Join(root, "docs", "broken.md"), `---
type: feature
id: FEAT-BROKEN
slug: broken
status: draft
container: SVC-1

## Summary {#summary}
Missing closing delimiter.
`)

	snapshot, appErr := BuildSnapshot(root, "FEAT-1")
	if appErr != nil {
		t.Fatalf("BuildSnapshot returned error: %v", appErr)
	}

	if len(snapshot.TargetMatches) != 2 {
		t.Fatalf("expected duplicate target matches=2, got %d", len(snapshot.TargetMatches))
	}
	if len(snapshot.Documents) != 2 {
		t.Fatalf("expected parsed documents=2 (broken ignored), got %d", len(snapshot.Documents))
	}
}

func writeFixtureFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
