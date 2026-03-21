package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func TestParseGlobalOptionsLoadsExplicitConfigAndResolvesRelativePaths(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "cfg", "custom.json")
	writeTestFile(t, configPath, `{"schema":"../schemas/spec.schema.yaml","workspace":"../docs"}`)

	restore := chdirForTest(t, root)
	defer restore()

	opts, command, commandArgs, err := parseGlobalOptions([]string{"--config", "cfg/custom.json", "validate"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if command != "validate" {
		t.Fatalf("expected command validate, got %q", command)
	}
	if len(commandArgs) != 0 {
		t.Fatalf("expected empty command args, got %v", commandArgs)
	}

	expectedSchema := filepath.Clean(filepath.Join(root, "schemas", "spec.schema.yaml"))
	assertEquivalentPath(t, expectedSchema, opts.SchemaPath)
	expectedWorkspace := filepath.Clean(filepath.Join(root, "docs"))
	assertEquivalentPath(t, expectedWorkspace, opts.Workspace)
}

func TestParseGlobalOptionsExplicitCLIFlagsOverrideConfig(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "spec-cli.json")
	writeTestFile(t, configPath, `{"schema":"from-config.schema.yaml","workspace":"from-config"}`)

	restore := chdirForTest(t, root)
	defer restore()

	explicitSchema := filepath.Join(root, "custom.schema.yaml")
	explicitWorkspace := filepath.Join(root, "custom-workspace")
	opts, _, _, err := parseGlobalOptions([]string{"--config", "spec-cli.json", "--schema", explicitSchema, "--workspace", explicitWorkspace, "validate"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEquivalentPath(t, explicitSchema, opts.SchemaPath)
	assertEquivalentPath(t, explicitWorkspace, opts.Workspace)
}

func TestParseGlobalOptionsAutoConfigMissingIsNotAnError(t *testing.T) {
	root := t.TempDir()
	restore := chdirForTest(t, root)
	defer restore()

	opts, command, _, err := parseGlobalOptions([]string{"validate"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if command != "validate" {
		t.Fatalf("expected validate command, got %q", command)
	}
	if opts.SchemaPath != "spec.schema.yaml" {
		t.Fatalf("expected default schema path, got %q", opts.SchemaPath)
	}
	if opts.Workspace != "." {
		t.Fatalf("expected default workspace path, got %q", opts.Workspace)
	}
}

func TestParseGlobalOptionsUnknownConfigKeyReturnsInvalidConfig(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "spec-cli.json")
	writeTestFile(t, configPath, `{"schema":"spec.schema.yaml","workspace":".","unknown":"x"}`)

	restore := chdirForTest(t, root)
	defer restore()

	_, _, _, err := parseGlobalOptions([]string{"validate"})
	if err == nil {
		t.Fatalf("expected INVALID_CONFIG error")
	}
	if err.Code != domainerrors.CodeInvalidConfig {
		t.Fatalf("expected %s, got %s", domainerrors.CodeInvalidConfig, err.Code)
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent directories: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func chdirForTest(t *testing.T, path string) func() {
	t.Helper()

	current, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(path); err != nil {
		t.Fatalf("chdir to %s: %v", path, err)
	}

	return func() {
		if chdirErr := os.Chdir(current); chdirErr != nil {
			t.Fatalf("restore cwd: %v", chdirErr)
		}
	}
}

func assertEquivalentPath(t *testing.T, expected string, actual string) {
	t.Helper()

	expectedResolved := normalizePathAlias(filepath.Clean(expected))
	actualResolved := normalizePathAlias(filepath.Clean(actual))

	if resolvedExpected, err := filepath.EvalSymlinks(expectedResolved); err == nil {
		expectedResolved = resolvedExpected
	}
	if resolvedActual, err := filepath.EvalSymlinks(actualResolved); err == nil {
		actualResolved = resolvedActual
	}

	if expectedResolved != actualResolved {
		t.Fatalf("path mismatch: expected %q, got %q", expectedResolved, actualResolved)
	}
}

func normalizePathAlias(path string) string {
	if strings.HasPrefix(path, "/private/var/") {
		return strings.TrimPrefix(path, "/private")
	}
	return path
}
