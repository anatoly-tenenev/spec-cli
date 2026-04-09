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
		assertHelpSchemaSectionContract(t, execResult.Stdout)
		assertSchemaResolvedPathContract(t, execResult.Stdout)
		assertGeneralHelpCommandOrderConsistency(t, execResult.Stdout)
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
		assertContains(t, result.Stdout, "CLI\n  spec-cli is a schema-aware CLI for working with specification entities as JSON-like documents.")
		assertContains(t, result.Stdout, "Global options")
		assertContains(t, result.Stdout, "--config <path>: optional; JSON config with \"schema\"/\"workspace\"; if omitted, auto-loads \"./spec-cli.json\" when present")
		assertContains(t, result.Stdout, "Commands")
		assertHelpSchemaSectionContract(t, result.Stdout)
		assertSchemaResolvedPathContract(t, result.Stdout)
		assertContains(t, result.Stdout, "Command details")
		assertContains(t, result.Stdout, "help\n  Syntax")
		assertContains(t, result.Stdout, "query\n  Syntax")
		assertContains(t, result.Stdout, "version\n  Syntax")
		assertGeneralHelpCommandOrderConsistency(t, result.Stdout)
		assertContains(t, result.Stdout, "\"x-kind\": \"entityRef\"")
		assertContains(t, result.Stdout, "\"x-requiredWhen\": \"meta.status == 'active'\"")
		assertContains(t, result.Stdout, "\"x-refTypes\": [\"service\"]")
		assertContains(t, result.Stdout, "\"tier\": {")
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
		assertHelpSchemaSectionContract(t, result.Stdout)
		assertSchemaResolvedPathContract(t, result.Stdout)
		assertContains(t, result.Stdout, "Command details")
		assertGeneralHelpCommandOrderConsistency(t, result.Stdout)
	})

	t.Run("help_text_non_default_schema_display_paths", func(t *testing.T) {
		root := t.TempDir()
		workspace := filepath.Join(root, "workspace-custom")
		if err := os.MkdirAll(filepath.Join(workspace, "schemas"), 0o755); err != nil {
			t.Fatalf("create workspace fixture: %v", err)
		}
		schema := filepath.Join(workspace, "schemas", "custom.schema.yaml")
		if err := os.WriteFile(schema, []byte(helpSchemaFixture), 0o644); err != nil {
			t.Fatalf("write schema fixture: %v", err)
		}

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
		if strings.TrimSpace(result.Stderr) != "" {
			t.Fatalf("stderr must be empty, got: %s", result.Stderr)
		}

		assertHelpSchemaSectionContract(t, result.Stdout)
		assertContains(t, result.Stdout, "  Workspace: /spec-cli-fixed-root/workspace")
		assertContains(t, result.Stdout, "  Status: loaded")
		assertContains(t, result.Stdout, "  ResolvedPath: /spec-cli-fixed-root/workspace/schemas/custom.schema.yaml")
		assertSchemaResolvedPathContract(t, result.Stdout)
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
		assertContains(t, result.Stdout, "Command\n  query: read many entities through a schema-derived read model")
		assertHelpSchemaSectionContract(t, result.Stdout)
		assertSchemaResolvedPathContract(t, result.Stdout)
		assertContains(t, result.Stdout, "Status: missing")
		assertContains(t, result.Stdout, "ReasonCode: SCHEMA_NOT_FOUND")
		assertContains(t, result.Stdout, "Impact: schema-derived entity types, read/write paths, enum values and specification projection are unavailable; do not infer these values heuristically")
		assertContains(t, result.Stdout, "RecoveryClass: provide_explicit_schema")
		assertContains(t, result.Stdout, "RetryCommand: spec-cli --schema <path> help")
		assertContains(t, result.Stdout, "Operation model")
		assertContains(t, result.Stdout, "Active type set")
		assertContains(t, result.Stdout, "Read model")
		assertContains(t, result.Stdout, "Read-model path forms")
		assertContains(t, result.Stdout, "Where language")
		assertContains(t, result.Stdout, "Operation constraints")
		assertContains(t, result.Stdout, "Defaults")
		assertContains(t, result.Stdout, "schema_derived: true")
		assertContains(t, result.Stdout, "options marked schema_derived (--type, --where, --select, --sort) keep derivation rules, but concrete schema-derived values are intentionally not listed.")
		if strings.Contains(result.Stdout, "Specification projection\n") {
			t.Fatalf("degraded command help must not render specification projection:\n%s", result.Stdout)
		}
	})

	t.Run("help_general_schema_parse_error_keeps_schema_neutral_sections", func(t *testing.T) {
		workspace, schema := prepareHelpFixture(t)
		if err := os.WriteFile(schema, []byte("entity: ["), 0o644); err != nil {
			t.Fatalf("write invalid schema fixture: %v", err)
		}

		result, err := integrationharness.RunCLIProcess(
			context.Background(),
			[]string{"--workspace", workspace, "--schema", schema, "help"},
			"",
			"",
			map[string]string{helpFixedPathRootEnv: helpFixedPathRoot},
		)
		if err != nil {
			t.Fatalf("run degraded general help: %v", err)
		}
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d\nstdout:\n%s\nstderr:\n%s", result.ExitCode, result.Stdout, result.Stderr)
		}

		assertHelpSchemaSectionContract(t, result.Stdout)
		assertContains(t, result.Stdout, "Status: invalid")
		assertContains(t, result.Stdout, "ReasonCode: SCHEMA_PARSE_ERROR")
		assertContains(t, result.Stdout, "Execution model")
		assertContains(t, result.Stdout, "Specification model")
		assertContains(t, result.Stdout, "Projection conventions")
		assertContains(t, result.Stdout, "Reference value model")
		assertContains(t, result.Stdout, "Command details")
		if strings.Contains(result.Stdout, "Specification projection\n") {
			t.Fatalf("degraded general help must not render specification projection:\n%s", result.Stdout)
		}
	})

	t.Run("help_general_schema_validation_error_missing_path_template_is_degraded", func(t *testing.T) {
		workspace, schema := prepareHelpFixture(t)
		if err := os.WriteFile(schema, []byte(helpSchemaMissingPathTemplateFixture), 0o644); err != nil {
			t.Fatalf("write schema fixture without pathTemplate: %v", err)
		}

		result, err := integrationharness.RunCLIProcess(
			context.Background(),
			[]string{"--workspace", workspace, "--schema", schema, "help"},
			"",
			"",
			map[string]string{helpFixedPathRootEnv: helpFixedPathRoot},
		)
		if err != nil {
			t.Fatalf("run degraded general help for schema validation error: %v", err)
		}
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d\nstdout:\n%s\nstderr:\n%s", result.ExitCode, result.Stdout, result.Stderr)
		}

		assertHelpSchemaSectionContract(t, result.Stdout)
		assertContains(t, result.Stdout, "Status: invalid")
		assertContains(t, result.Stdout, "ReasonCode: SCHEMA_VALIDATION_ERROR")
		assertContains(t, result.Stdout, "Impact: schema-derived entity types, read/write paths, enum values and specification projection are unavailable; do not infer these values heuristically")
		assertContains(t, result.Stdout, "RecoveryClass: fix_schema_file")
		assertContains(t, result.Stdout, "RetryCommand: spec-cli schema check --schema ")
		if strings.Contains(result.Stdout, "Status: loaded") {
			t.Fatalf("schema validation error must not be rendered as loaded:\n%s", result.Stdout)
		}
		if strings.Contains(result.Stdout, "Specification projection\n") {
			t.Fatalf("degraded general help must not render specification projection:\n%s", result.Stdout)
		}
	})

	t.Run("help_command_schema_missing_show_projection_does_not_render_projection", func(t *testing.T) {
		workspace, _ := prepareHelpFixture(t)
		missingSchema := filepath.Join(workspace, "missing.schema.yaml")
		result, err := integrationharness.RunCLIProcess(
			context.Background(),
			[]string{"--workspace", workspace, "--schema", missingSchema, "help", "query", "--show-schema-projection"},
			"",
			"",
			map[string]string{helpFixedPathRootEnv: helpFixedPathRoot},
		)
		if err != nil {
			t.Fatalf("run degraded help query --show-schema-projection: %v", err)
		}
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d\nstdout:\n%s\nstderr:\n%s", result.ExitCode, result.Stdout, result.Stderr)
		}
		if strings.Contains(result.Stdout, "Specification projection\n") {
			t.Fatalf("degraded command help must not render projection for --show-schema-projection:\n%s", result.Stdout)
		}
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
		t.Fatalf("schema resolved path line %q must be present in output:\n%s", resolvedPrefix, text)
	}

	resolvedValue := strings.TrimSpace(strings.TrimPrefix(resolvedLine, resolvedPrefix))
	if strings.Contains(resolvedValue, "<workspace>/") || strings.Contains(resolvedValue, "<workspace>") {
		t.Fatalf("schema resolved path must not contain workspace placeholder, got %q", resolvedValue)
	}
	if !filepath.IsAbs(resolvedValue) {
		t.Fatalf("schema resolved path must be absolute, got %q", resolvedValue)
	}
}

func assertHelpSchemaSectionContract(t *testing.T, text string) {
	t.Helper()

	headings := topLevelHeadings(text)
	if len(headings) < 2 {
		t.Fatalf("help output must contain top-level sections:\n%s", text)
	}
	schemaSectionCount := 0
	for _, heading := range headings {
		if heading == "Schema" {
			schemaSectionCount++
		}
	}
	if schemaSectionCount != 1 {
		t.Fatalf("help output must contain exactly one Schema section, got %d\n%s", schemaSectionCount, text)
	}

	switch headings[0] {
	case "CLI":
		if headings[1] != "Schema" {
			t.Fatalf("general help must place Schema right after CLI, headings: %v", headings)
		}
	case "Command":
		if headings[1] != "Schema" {
			t.Fatalf("command help must place Schema right after Command, headings: %v", headings)
		}
		if len(headings) < 3 || headings[2] != "Syntax" {
			t.Fatalf("command help must place Syntax right after Schema, headings: %v", headings)
		}
	default:
		t.Fatalf("unexpected first top-level section %q", headings[0])
	}

	schemaLines := sectionLines(text, "Schema")
	if len(schemaLines) == 0 {
		t.Fatalf("Schema section must be present:\n%s", text)
	}
	if len(sectionLines(text, "Environment")) > 0 {
		t.Fatalf("legacy Environment section must be absent:\n%s", text)
	}
	if strings.Contains(text, "SchemaPath:") {
		t.Fatalf("legacy SchemaPath field must be absent:\n%s", text)
	}
	if strings.Contains(text, "SchemaStatus:") {
		t.Fatalf("legacy SchemaStatus field must be absent:\n%s", text)
	}
	if strings.Contains(text, "  Specification projection:") {
		t.Fatalf("schema block must not embed projection payload:\n%s", text)
	}

	status := ""
	for _, line := range schemaLines {
		switch {
		case strings.HasPrefix(line, "  Workspace: "):
		case strings.HasPrefix(line, "  ResolvedPath: "):
		case strings.HasPrefix(line, "  Status: "):
			status = strings.TrimSpace(strings.TrimPrefix(line, "  Status: "))
		}
	}

	requireSchemaLine := func(prefix string) {
		for _, line := range schemaLines {
			if strings.HasPrefix(line, prefix) {
				return
			}
		}
		t.Fatalf("Schema section must contain %q line:\n%s", prefix, text)
	}
	ensureSchemaLineAbsent := func(prefix string) {
		for _, line := range schemaLines {
			if strings.HasPrefix(line, prefix) {
				t.Fatalf("Schema section must not contain %q line for status=%q:\n%s", prefix, status, text)
			}
		}
	}

	requireSchemaLine("  Workspace: ")
	requireSchemaLine("  ResolvedPath: ")
	requireSchemaLine("  Status: ")

	if status == "loaded" {
		ensureSchemaLineAbsent("  ReasonCode: ")
		ensureSchemaLineAbsent("  Impact: ")
		ensureSchemaLineAbsent("  RecoveryClass: ")
		ensureSchemaLineAbsent("  RetryCommand: ")
		return
	}

	requireSchemaLine("  ReasonCode: ")
	requireSchemaLine("  Impact: ")
	requireSchemaLine("  RecoveryClass: ")
	requireSchemaLine("  RetryCommand: ")
}

func assertGeneralHelpCommandOrderConsistency(t *testing.T, text string) {
	t.Helper()

	commandsSection := sectionLines(text, "Commands")
	if len(commandsSection) == 0 {
		return
	}

	commandOrder := make([]string, 0, len(commandsSection)-1)
	commandSet := map[string]struct{}{}
	for _, line := range commandsSection[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		name, _, ok := strings.Cut(trimmed, ":")
		if !ok {
			t.Fatalf("Commands section must use '<name>: <summary>' lines, got %q", trimmed)
		}
		name = strings.TrimSpace(name)
		if name == "" {
			t.Fatalf("Commands section command name must not be empty, line %q", trimmed)
		}
		commandOrder = append(commandOrder, name)
		commandSet[name] = struct{}{}
	}
	if len(commandOrder) == 0 {
		t.Fatalf("Commands section must list at least one command:\n%s", text)
	}

	lines := strings.Split(text, "\n")
	commandDetailsStart := -1
	for idx, line := range lines {
		if line == "Command details" {
			commandDetailsStart = idx
			break
		}
	}
	if commandDetailsStart == -1 {
		t.Fatalf("general help must contain Command details section:\n%s", text)
	}

	detailsOrder := make([]string, 0, len(commandOrder))
	for idx := commandDetailsStart + 1; idx < len(lines); idx++ {
		line := lines[idx]
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, " ") {
			continue
		}
		if _, exists := commandSet[line]; !exists {
			break
		}
		detailsOrder = append(detailsOrder, line)
	}

	if len(detailsOrder) != len(commandOrder) {
		t.Fatalf("Commands and Command details must contain the same command set/order:\ncommands=%v\ndetails=%v\n%s", commandOrder, detailsOrder, text)
	}
	for idx := range commandOrder {
		if commandOrder[idx] != detailsOrder[idx] {
			t.Fatalf("Commands and Command details order mismatch at index %d: commands=%v details=%v", idx, commandOrder, detailsOrder)
		}
	}
}

func topLevelHeadings(text string) []string {
	headings := make([]string, 0)
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, " ") {
			continue
		}
		headings = append(headings, line)
	}
	return headings
}

func sectionLines(text string, title string) []string {
	lines := strings.Split(text, "\n")
	for idx, line := range lines {
		if line != title {
			continue
		}

		section := []string{line}
		for next := idx + 1; next < len(lines); next++ {
			candidate := lines[next]
			if strings.TrimSpace(candidate) == "" {
				continue
			}
			if !strings.HasPrefix(candidate, " ") {
				break
			}
			section = append(section, candidate)
		}
		return section
	}
	return nil
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
    idPrefix: FEAT
    pathTemplate: "features/{slug}.md"
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
              type: entityRef
              refTypes:
                - service
        owner:
          description: Parent service
          required: ${meta.status == 'active'}
          schema:
            type: entityRef
            refTypes:
              - service
    content:
      sections:
        summary:
          description: Short summary
          required: ${meta.status == 'active'}
          title: Summary
  service:
    idPrefix: SVC
    pathTemplate: "services/{slug}.md"
    meta:
      fields:
        tier:
          required: true
          schema:
            type: string
`

const helpSchemaMissingPathTemplateFixture = `version: "0.0.4"
description: Workspace specification schema
entity:
  feature:
    description: Feature specifications
    idPrefix: FEAT
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
              type: entityRef
              refTypes:
                - service
        owner:
          description: Parent service
          required: ${meta.status == 'active'}
          schema:
            type: entityRef
            refTypes:
              - service
    content:
      sections:
        summary:
          description: Short summary
          required: ${meta.status == 'active'}
          title: Summary
  service:
    idPrefix: SVC
    meta:
      fields:
        tier:
          required: true
          schema:
            type: string
`
