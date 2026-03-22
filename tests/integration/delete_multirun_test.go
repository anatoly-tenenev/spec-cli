package integration_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	integrationharness "github.com/anatoly-tenenev/spec-cli/tests/integration/internal/harness"
)

// Kept as a dynamic black-box test because this contract compares two independent
// CLI runs on separate clean workspace copies (dry-run vs real-run). A single
// data-first case fixture cannot express this cross-run equivalence assertion.
func TestDeleteHappy02DryRunMatchesRealRevision(t *testing.T) {
	caseDir := filepath.Join("cases", "delete", "10_happy", "0002_ok_delete_dry_run_with_expect_revision_json")
	testCase, err := loadCase(caseDir)
	if err != nil {
		t.Fatalf("load case: %v", err)
	}

	dryPayload, dryExitCode := runDeleteCommandOnFreshWorkspace(t, caseDir, testCase.Args)
	if dryExitCode != 0 {
		t.Fatalf("dry-run exit code: expected 0, got %d", dryExitCode)
	}

	realArgs := removeArgToken(testCase.Args, "--dry-run")
	realPayload, realExitCode := runDeleteCommandOnFreshWorkspace(t, caseDir, realArgs)
	if realExitCode != 0 {
		t.Fatalf("real-run exit code: expected 0, got %d", realExitCode)
	}

	dryRevision := nestedString(t, dryPayload, "target", "revision")
	realRevision := nestedString(t, realPayload, "target", "revision")
	if dryRevision != realRevision {
		t.Fatalf("dry-run and real-run revisions must match: dry=%s real=%s", dryRevision, realRevision)
	}

	dryComparable := cloneMap(dryPayload)
	realComparable := cloneMap(realPayload)
	delete(dryComparable, "dry_run")
	delete(realComparable, "dry_run")
	if !reflect.DeepEqual(dryComparable, realComparable) {
		t.Fatalf("dry-run payload must match real-run payload except dry_run flag")
	}
}

func runDeleteCommandOnFreshWorkspace(
	t *testing.T,
	caseDir string,
	args []string,
) (map[string]any, int) {
	t.Helper()

	tempRoot := t.TempDir()
	workspacePath := filepath.Join(tempRoot, "workspace")
	if err := integrationharness.CopyDir(filepath.Join(caseDir, "workspace.in"), workspacePath); err != nil {
		t.Fatalf("copy workspace.in: %v", err)
	}

	schemaPath := filepath.Join(tempRoot, "spec.schema.yaml")
	if err := integrationharness.CopyFile(filepath.Join(caseDir, "spec.schema.yaml"), schemaPath); err != nil {
		t.Fatalf("copy spec.schema.yaml: %v", err)
	}

	replacedArgs := integrationharness.ReplacePlaceholders(args, workspacePath, schemaPath)
	result, err := integrationharness.RunCLIProcess(context.Background(), replacedArgs, "", "", nil)
	if err != nil {
		t.Fatalf("run subprocess: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(result.Stdout), &payload); err != nil {
		t.Fatalf("decode stdout json: %v", err)
	}

	return payload, result.ExitCode
}
