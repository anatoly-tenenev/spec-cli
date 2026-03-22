package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	integrationharness "github.com/anatoly-tenenev/spec-cli/tests/integration/internal/harness"
)

const (
	helpFixedPathRootEnv = "SPEC_CLI_FIXED_PATH_ROOT"
	helpFixedPathRoot    = "/spec-cli-fixed-root"
)

func TestHelpGeneralCases(t *testing.T) {
	caseDirs := collectHelpCaseDirs(t, filepath.Join("cases", "help", "10_general"), "help general")

	for _, caseDir := range caseDirs {
		tc, loadErr := loadCase(caseDir)
		if loadErr != nil {
			t.Fatalf("load case %s: %v", caseDir, loadErr)
		}

		testCase := tc
		t.Run(testCase.ID, func(t *testing.T) {
			runHelpTextCase(t, caseDir, testCase)
		})
	}
}

func TestHelpSchemaRecoveryCases(t *testing.T) {
	caseDirs := collectHelpCaseDirs(t, filepath.Join("cases", "help", "15_schema_recovery"), "help schema recovery")

	for _, caseDir := range caseDirs {
		tc, loadErr := loadCase(caseDir)
		if loadErr != nil {
			t.Fatalf("load case %s: %v", caseDir, loadErr)
		}

		testCase := tc
		t.Run(testCase.ID, func(t *testing.T) {
			runHelpTextCase(t, caseDir, testCase)
		})
	}
}

func TestHelpErrorCases(t *testing.T) {
	caseDirs := collectHelpCaseDirs(t, filepath.Join("cases", "help", "20_errors"), "help error")

	for _, caseDir := range caseDirs {
		tc, loadErr := loadCase(caseDir)
		if loadErr != nil {
			t.Fatalf("load case %s: %v", caseDir, loadErr)
		}

		testCase := tc
		t.Run(testCase.ID, func(t *testing.T) {
			runHelpCase(t, caseDir, testCase)
		})
	}
}

func collectHelpCaseDirs(t *testing.T, caseRoot string, label string) []string {
	t.Helper()

	entries, err := os.ReadDir(caseRoot)
	if err != nil {
		t.Fatalf("read %s cases: %v", label, err)
	}

	caseDirs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		caseDirs = append(caseDirs, filepath.Join(caseRoot, entry.Name()))
	}
	sort.Strings(caseDirs)
	return caseDirs
}

func runHelpTextCase(t *testing.T, caseDir string, testCase integrationCase) {
	t.Helper()
	runHelpCase(t, caseDir, testCase)
}

func runHelpCase(t *testing.T, caseDir string, testCase integrationCase) {
	t.Helper()

	tempRoot := t.TempDir()
	workspacePath := filepath.Join(tempRoot, "workspace")
	if err := prepareHelpWorkspace(caseDir, testCase.Workspace.InputDir, workspacePath); err != nil {
		t.Fatalf("prepare workspace.in: %v", err)
	}

	restorePermissions := func() {}
	if len(testCase.Workspace.Permissions) > 0 {
		rollback, err := integrationharness.ApplyWorkspacePermissions(workspacePath, testCase.Workspace.Permissions)
		if err != nil {
			t.Fatalf("apply workspace permissions: %v", err)
		}
		restorePermissions = rollback
	}
	defer restorePermissions()

	schemaPath := filepath.Join(tempRoot, "spec.schema.yaml")
	if err := integrationharness.CopyFile(filepath.Join(caseDir, "spec.schema.yaml"), schemaPath); err != nil {
		t.Fatalf("copy spec.schema.yaml: %v", err)
	}

	args := integrationharness.ReplacePlaceholders(testCase.Args, workspacePath, schemaPath)
	runtimeEnv := map[string]string{
		helpFixedPathRootEnv: helpFixedPathRoot,
	}
	for key, value := range testCase.Runtime.Env {
		runtimeEnv[key] = value
	}
	execResult, runErr := integrationharness.RunCLIProcess(context.Background(), args, testCase.Runtime.FixedNowUTC, "", runtimeEnv)
	if runErr != nil {
		t.Fatalf("execute cli process: %v", runErr)
	}
	if execResult.ExitCode != testCase.Expect.ExitCode {
		t.Fatalf(
			"exit code mismatch:\nexpected: %d\nactual: %d\nstdout:\n%s\nstderr:\n%s",
			testCase.Expect.ExitCode,
			execResult.ExitCode,
			execResult.Stdout,
			execResult.Stderr,
		)
	}
	integrationharness.AssertStderr(t, caseDir, testCase, execResult.Stderr)
	if strings.HasSuffix(strings.ToLower(testCase.Expect.ResponseFile), ".txt") {
		assertSchemaResolvedPathContract(t, execResult.Stdout)
		expectedRaw, readErr := os.ReadFile(filepath.Join(caseDir, testCase.Expect.ResponseFile))
		if readErr != nil {
			t.Fatalf("read expected response: %v", readErr)
		}
		expected := string(expectedRaw)
		if execResult.Stdout != expected {
			t.Fatalf("response mismatch:\nexpected:\n%s\nactual:\n%s", expected, execResult.Stdout)
		}
		return
	}

	integrationharness.AssertResponse(t, caseDir, testCase, []byte(execResult.Stdout))
}

func prepareHelpWorkspace(caseDir string, inputDir string, workspacePath string) error {
	input := strings.TrimSpace(inputDir)
	if input == "" {
		return os.MkdirAll(workspacePath, 0o755)
	}

	sourcePath := filepath.Join(caseDir, input)
	if _, err := os.Stat(sourcePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return os.MkdirAll(workspacePath, 0o755)
		}
		return err
	}

	return integrationharness.CopyDir(sourcePath, workspacePath)
}

func TestHelpSelectedCases(t *testing.T) {
	t.Run("help_text_explicit_format", func(t *testing.T) {
		workspace, schema := prepareHelpFixture(t)
		result, err := integrationharness.RunCLIProcess(
			context.Background(),
			[]string{"--workspace", workspace, "--schema", schema, "--format", "text", "help"},
			"",
			"",
			map[string]string{helpFixedPathRootEnv: helpFixedPathRoot},
		)
		if err != nil {
			t.Fatalf("run help: %v", err)
		}
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d\nstdout:\n%s\nstderr:\n%s", result.ExitCode, result.Stdout, result.Stderr)
		}
		if strings.TrimSpace(result.Stderr) != "" {
			t.Fatalf("stderr must be empty, got: %s", result.Stderr)
		}
		assertContains(t, result.Stdout, "CLI\n  spec-cli is a machine-first CLI utility for spec documents.")
		assertContains(t, result.Stdout, "Global options")
		assertContains(t, result.Stdout, "--config <path>: optional; JSON config with \"schema\"/\"workspace\"; if omitted, auto-loads \"./spec-cli.json\" when present")
		assertContains(t, result.Stdout, "Commands")
		assertContains(t, result.Stdout, "Schema")
		assertSchemaResolvedPathContract(t, result.Stdout)
		assertContains(t, result.Stdout, "Status: loaded")
		assertContains(t, result.Stdout, "Command details")
		assertContains(t, result.Stdout, "help\n  Syntax")
		assertContains(t, result.Stdout, "query\n  Syntax")
		assertContains(t, result.Stdout, "version\n  Syntax")
	})

	t.Run("help_text_default_format", func(t *testing.T) {
		workspace, schema := prepareHelpFixture(t)
		result, err := integrationharness.RunCLIProcess(
			context.Background(),
			[]string{"--workspace", workspace, "--schema", schema, "help"},
			"",
			"",
			map[string]string{helpFixedPathRootEnv: helpFixedPathRoot},
		)
		if err != nil {
			t.Fatalf("run help: %v", err)
		}
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d\nstdout:\n%s\nstderr:\n%s", result.ExitCode, result.Stdout, result.Stderr)
		}
		if strings.HasPrefix(strings.TrimSpace(result.Stdout), "{") {
			t.Fatalf("expected text output, got json-like payload:\n%s", result.Stdout)
		}
		assertSchemaResolvedPathContract(t, result.Stdout)
		assertContains(t, result.Stdout, "Status: loaded")
		assertContains(t, result.Stdout, "Command details")
	})

	t.Run("help_format_json_unsupported", func(t *testing.T) {
		workspace, schema := prepareHelpFixture(t)
		result, err := integrationharness.RunCLIProcess(
			context.Background(),
			[]string{"--workspace", workspace, "--schema", schema, "--format", "json", "help"},
			"",
			"",
			nil,
		)
		if err != nil {
			t.Fatalf("run help: %v", err)
		}
		if result.ExitCode != 1 {
			t.Fatalf("expected exit code 1, got %d\nstdout:\n%s\nstderr:\n%s", result.ExitCode, result.Stdout, result.Stderr)
		}
		assertJSONError(t, result.Stdout, "unsupported", "CAPABILITY_UNSUPPORTED", 1)
	})

	t.Run("help_unknown_command", func(t *testing.T) {
		workspace, schema := prepareHelpFixture(t)
		result, err := integrationharness.RunCLIProcess(
			context.Background(),
			[]string{"--workspace", workspace, "--schema", schema, "help", "unknown"},
			"",
			"",
			nil,
		)
		if err != nil {
			t.Fatalf("run help: %v", err)
		}
		if result.ExitCode != 2 {
			t.Fatalf("expected exit code 2, got %d\nstdout:\n%s\nstderr:\n%s", result.ExitCode, result.Stdout, result.Stderr)
		}
		assertJSONError(t, result.Stdout, "invalid", "INVALID_ARGS", 2)
	})

	t.Run("help_command_schema_missing_returns_recovery_contract", func(t *testing.T) {
		workspace, _ := prepareHelpFixture(t)
		missingSchema := filepath.Join(workspace, "missing.schema.yaml")
		result, err := integrationharness.RunCLIProcess(
			context.Background(),
			[]string{"--workspace", workspace, "--schema", missingSchema, "help", "query"},
			"",
			"",
			map[string]string{helpFixedPathRootEnv: helpFixedPathRoot},
		)
		if err != nil {
			t.Fatalf("run help query: %v", err)
		}
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d\nstdout:\n%s\nstderr:\n%s", result.ExitCode, result.Stdout, result.Stderr)
		}
		if strings.TrimSpace(result.Stderr) != "" {
			t.Fatalf("stderr must be empty, got: %s", result.Stderr)
		}
		assertContains(t, result.Stdout, "Command\n  query: read many entities with structural filters and deterministic pagination")
		assertContains(t, result.Stdout, "Schema")
		assertSchemaResolvedPathContract(t, result.Stdout)
		assertContains(t, result.Stdout, "Status: missing")
		assertContains(t, result.Stdout, "ReasonCode: SCHEMA_NOT_FOUND")
		assertContains(t, result.Stdout, "Impact: schema-derived entity types, namespace paths, enum values and CLI projection of schema-derived fields are unavailable; do not infer these values heuristically")
		assertContains(t, result.Stdout, "RecoveryClass: provide_explicit_schema")
		assertContains(t, result.Stdout, "RetryCommand: spec-cli --schema <path> help")
		assertContains(t, result.Stdout, "schema_derived: true")
		assertContains(t, result.Stdout, "options marked schema_derived (--type, --where-json, --select, --sort) keep derivation rules, but concrete schema-derived values are intentionally not listed.")
	})
}

func prepareHelpFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("create workspace fixture: %v", err)
	}
	schemaPath := filepath.Join(workspace, "spec.schema.yaml")
	if err := os.WriteFile(schemaPath, []byte(helpSchemaFixture), 0o644); err != nil {
		t.Fatalf("write schema fixture: %v", err)
	}
	return workspace, schemaPath
}

func assertSchemaResolvedPathContract(t *testing.T, text string) {
	t.Helper()

	legacyPathPrefixes := []string{
		"  Path (relative to --workspace): ",
		"  Path: ",
	}
	resolvedPrefix := "  ResolvedPath: "
	resolvedLine := ""
	for _, line := range strings.Split(text, "\n") {
		for _, pathPrefix := range legacyPathPrefixes {
			if strings.HasPrefix(line, pathPrefix) {
				t.Fatalf("legacy schema path line %q must be absent in output:\n%s", pathPrefix, text)
			}
		}
		if strings.HasPrefix(line, resolvedPrefix) {
			resolvedLine = line
		}
	}

	if resolvedLine == "" {
		t.Fatalf("missing schema resolved path line %q in output:\n%s", resolvedPrefix, text)
	}

	resolvedValue := strings.TrimSpace(strings.TrimPrefix(resolvedLine, resolvedPrefix))
	if strings.Contains(resolvedValue, "<workspace>/") || strings.Contains(resolvedValue, "<workspace>") {
		t.Fatalf("schema resolved path must not contain workspace placeholder, got %q", resolvedValue)
	}
	if !filepath.IsAbs(resolvedValue) {
		t.Fatalf("schema resolved path must be absolute, got %q", resolvedValue)
	}
}

func assertContains(t *testing.T, text string, expected string) {
	t.Helper()
	if !strings.Contains(text, expected) {
		t.Fatalf("expected output to contain %q\nactual:\n%s", expected, text)
	}
}

func assertJSONError(t *testing.T, raw string, resultState string, errorCode string, exitCode int) {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("decode json output: %v\nraw:\n%s", err, raw)
	}

	state, _ := payload["result_state"].(string)
	if state != resultState {
		t.Fatalf("unexpected result_state: expected %q, got %q", resultState, state)
	}

	errorPayload, _ := payload["error"].(map[string]any)
	if errorPayload == nil {
		t.Fatalf("missing error payload: %#v", payload)
	}

	code, _ := errorPayload["code"].(string)
	if code != errorCode {
		t.Fatalf("unexpected error.code: expected %q, got %q", errorCode, code)
	}

	exit, _ := errorPayload["exit_code"].(float64)
	if int(exit) != exitCode {
		t.Fatalf("unexpected error.exit_code: expected %d, got %v", exitCode, errorPayload["exit_code"])
	}
}

const helpSchemaFixture = `version: "0.0.4"
description: Workspace specification schema
entity:
  feature:
    description: Feature specifications
    id_prefix: FEAT
    meta:
      fields:
        status:
          description: Lifecycle status
          required: false
          schema:
            type: string
            enum:
              - draft
              - active
              - deprecated
        watchers:
          description: Related services
          required: false
          schema:
            type: array
            items:
              type: entity_ref
              refTypes:
                - service
        owner:
          description: Parent service
          required_when:
            eq?: [meta.status, active]
          schema:
            type: entity_ref
            refTypes:
              - service
    content:
      sections:
        summary:
          description: Short summary
          required_when:
            eq?: [meta.status, active]
          title: Summary
  service:
    id_prefix: SVC
    meta:
      fields:
        tier:
          required: true
          schema:
            type: string
`
