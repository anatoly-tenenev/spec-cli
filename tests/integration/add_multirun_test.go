package integration_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAddHappy07DryRunMatchesRealRevision(t *testing.T) {
	caseDir := filepath.Join("cases", "add", "10_happy", "0007_ok_add_dry_run_success_json")
	testCase, err := loadCase(caseDir)
	if err != nil {
		t.Fatalf("load case: %v", err)
	}

	dryRunPayload, _, dryExitCode := runAddCommandOnFreshWorkspace(t, caseDir, testCase.Args, "")
	if dryExitCode != 0 {
		t.Fatalf("dry-run exit code: expected 0, got %d", dryExitCode)
	}

	realArgs := removeArgToken(testCase.Args, "--dry-run")
	realPayload, _, realExitCode := runAddCommandOnFreshWorkspace(t, caseDir, realArgs, "")
	if realExitCode != 0 {
		t.Fatalf("real-run exit code: expected 0, got %d", realExitCode)
	}

	dryRevision := nestedString(t, dryRunPayload, "entity", "revision")
	realRevision := nestedString(t, realPayload, "entity", "revision")
	if dryRevision != realRevision {
		t.Fatalf("dry-run and real-run revisions must match: dry=%s real=%s", dryRevision, realRevision)
	}

	dryComparable := cloneMap(dryRunPayload)
	realComparable := cloneMap(realPayload)
	delete(dryComparable, "dry_run")
	delete(realComparable, "dry_run")
	if !reflect.DeepEqual(dryComparable, realComparable) {
		t.Fatalf("dry-run payload must match real-run payload except dry_run flag")
	}
}

func TestAddFS04DifferentBodyProducesDifferentRevision(t *testing.T) {
	caseDir := filepath.Join("cases", "add", "10_happy", "0001_ok_add_create_feature_ref_dir_path_json")

	argsA := []string{
		"--workspace", "${WORKSPACE}",
		"--schema", "${SCHEMA}",
		"add",
		"--type", "feature",
		"--slug", "revision-a",
		"--set", "meta.status=draft",
		"--set", "refs.container=SVC-1",
		"--set", "content.sections.summary=A",
	}
	argsB := []string{
		"--workspace", "${WORKSPACE}",
		"--schema", "${SCHEMA}",
		"add",
		"--type", "feature",
		"--slug", "revision-a",
		"--set", "meta.status=draft",
		"--set", "refs.container=SVC-1",
		"--set", "content.sections.summary=B",
	}

	payloadA, _, exitA := runAddCommandOnFreshWorkspace(t, caseDir, argsA, "")
	payloadB, _, exitB := runAddCommandOnFreshWorkspace(t, caseDir, argsB, "")
	if exitA != 0 || exitB != 0 {
		t.Fatalf("expected successful runs, got exitA=%d exitB=%d", exitA, exitB)
	}

	revisionA := nestedString(t, payloadA, "entity", "revision")
	revisionB := nestedString(t, payloadB, "entity", "revision")
	if revisionA == revisionB {
		t.Fatalf("revisions must differ when markdown bytes differ: revision=%s", revisionA)
	}
}

func TestAddFS05IdenticalInputsDeterministicRevisionAndOrder(t *testing.T) {
	caseDir := filepath.Join("cases", "add", "10_happy", "0001_ok_add_create_feature_ref_dir_path_json")

	args := []string{
		"--workspace", "${WORKSPACE}",
		"--schema", "${SCHEMA}",
		"add",
		"--type", "feature",
		"--slug", "deterministic-revision",
		"--set", "meta.status=draft",
		"--set", "refs.container=SVC-1",
		"--set", "content.sections.summary=Stable summary",
	}

	payloadA, workspaceA, exitA := runAddCommandOnFreshWorkspace(t, caseDir, args, "")
	payloadB, workspaceB, exitB := runAddCommandOnFreshWorkspace(t, caseDir, args, "")
	if exitA != 0 || exitB != 0 {
		t.Fatalf("expected successful runs, got exitA=%d exitB=%d", exitA, exitB)
	}

	revisionA := nestedString(t, payloadA, "entity", "revision")
	revisionB := nestedString(t, payloadB, "entity", "revision")
	if revisionA != revisionB {
		t.Fatalf("revisions must match for identical inputs: revA=%s revB=%s", revisionA, revisionB)
	}

	targetRel := filepath.Join("services", "ledger-api", "features", "deterministic-revision.md")
	rawA, err := os.ReadFile(filepath.Join(workspaceA, targetRel))
	if err != nil {
		t.Fatalf("read output file A: %v", err)
	}
	rawB, err := os.ReadFile(filepath.Join(workspaceB, targetRel))
	if err != nil {
		t.Fatalf("read output file B: %v", err)
	}
	if string(rawA) != string(rawB) {
		t.Fatalf("serialized markdown must be byte-identical for identical inputs")
	}

	keys := extractFrontmatterKeys(string(rawA))
	expectedOrder := []string{"type", "id", "slug", "created_date", "updated_date", "status", "container"}
	if !reflect.DeepEqual(keys, expectedOrder) {
		t.Fatalf("unexpected frontmatter key order: expected=%v actual=%v", expectedOrder, keys)
	}
}

func runAddCommandOnFreshWorkspace(
	t *testing.T,
	caseDir string,
	args []string,
	stdinValue string,
) (map[string]any, string, int) {
	t.Helper()

	tempRoot := t.TempDir()
	workspacePath := filepath.Join(tempRoot, "workspace")
	if err := copyDir(filepath.Join(caseDir, "workspace.in"), workspacePath); err != nil {
		t.Fatalf("copy workspace.in: %v", err)
	}

	schemaPath := filepath.Join(tempRoot, "spec.schema.yaml")
	if err := copyFile(filepath.Join(caseDir, "spec.schema.yaml"), schemaPath); err != nil {
		t.Fatalf("copy spec.schema.yaml: %v", err)
	}

	replacedArgs := replacePlaceholders(args, workspacePath, schemaPath)
	result, err := runCLIProcess(context.Background(), replacedArgs, "2026-03-10", stdinValue)
	if err != nil {
		t.Fatalf("run subprocess: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(result.Stdout), &payload); err != nil {
		t.Fatalf("decode stdout json: %v", err)
	}

	return payload, workspacePath, result.ExitCode
}

func removeArgToken(args []string, token string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == token {
			continue
		}
		out = append(out, arg)
	}
	return out
}

func nestedString(t *testing.T, payload map[string]any, path ...string) string {
	t.Helper()

	current := any(payload)
	for _, segment := range path {
		obj, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("path %v: expected object at segment %q", path, segment)
		}
		next, exists := obj[segment]
		if !exists {
			t.Fatalf("path %v: missing segment %q", path, segment)
		}
		current = next
	}

	value, ok := current.(string)
	if !ok {
		t.Fatalf("path %v: expected string value", path)
	}
	return value
}

func cloneMap(source map[string]any) map[string]any {
	encoded, _ := json.Marshal(source)
	clone := map[string]any{}
	_ = json.Unmarshal(encoded, &clone)
	return clone
}

func extractFrontmatterKeys(serialized string) []string {
	lines := strings.Split(serialized, "\n")
	keys := []string{}
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return keys
	}

	for idx := 1; idx < len(lines); idx++ {
		line := lines[idx]
		if strings.TrimSpace(line) == "---" {
			break
		}
		if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		colon := strings.Index(line, ":")
		if colon <= 0 {
			continue
		}
		keys = append(keys, strings.TrimSpace(line[:colon]))
	}
	return keys
}
