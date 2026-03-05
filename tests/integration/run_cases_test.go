package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/cli"
)

type integrationCase struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	Expect      struct {
		ExitCode     int    `json:"exit_code"`
		ResponseFile string `json:"response_file"`
		StderrFile   string `json:"stderr_file"`
	} `json:"expect"`
	Workspace struct {
		InputDir     string `json:"input_dir"`
		OutputDir    string `json:"output_dir"`
		AssertOutput bool   `json:"assert_output"`
	} `json:"workspace"`
}

func TestValidateCases(t *testing.T) {
	caseRoot := filepath.Join("cases", "validate")
	entries, err := os.ReadDir(caseRoot)
	if err != nil {
		t.Fatalf("read cases directory: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		caseDir := filepath.Join(caseRoot, entry.Name())
		testCase, err := loadCase(caseDir)
		if err != nil {
			t.Fatalf("load case %s: %v", caseDir, err)
		}

		tc := testCase
		t.Run(tc.ID, func(t *testing.T) {
			runCase(t, caseDir, tc)
		})
	}
}

func loadCase(caseDir string) (integrationCase, error) {
	casePath := filepath.Join(caseDir, "case.json")
	raw, err := os.ReadFile(casePath)
	if err != nil {
		return integrationCase{}, fmt.Errorf("read case.json: %w", err)
	}

	var testCase integrationCase
	if err := json.Unmarshal(raw, &testCase); err != nil {
		return integrationCase{}, fmt.Errorf("decode case.json: %w", err)
	}
	return testCase, nil
}

func runCase(t *testing.T, caseDir string, testCase integrationCase) {
	t.Helper()

	tempRoot := t.TempDir()
	workspacePath := filepath.Join(tempRoot, "workspace")
	if err := copyDir(filepath.Join(caseDir, testCase.Workspace.InputDir), workspacePath); err != nil {
		t.Fatalf("copy workspace.in: %v", err)
	}

	schemaPath := filepath.Join(tempRoot, "spec.schema.yaml")
	if err := copyFile(filepath.Join(caseDir, "spec.schema.yaml"), schemaPath); err != nil {
		t.Fatalf("copy spec.schema.yaml: %v", err)
	}

	args := replacePlaceholders(testCase.Args, workspacePath, schemaPath)
	format := outputFormat(args)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := cli.NewApp(&stdout, &stderr, time.Now)
	exitCode := app.Run(context.Background(), args)

	if exitCode != testCase.Expect.ExitCode {
		t.Fatalf(
			"exit code mismatch:\nexpected: %d\nactual: %d\nstdout:\n%s\nstderr:\n%s",
			testCase.Expect.ExitCode,
			exitCode,
			stdout.String(),
			stderr.String(),
		)
	}

	assertStderr(t, caseDir, testCase, stderr.String())
	assertResponse(t, caseDir, testCase, format, stdout.Bytes())
}

func assertStderr(t *testing.T, caseDir string, testCase integrationCase, actualStderr string) {
	t.Helper()

	if testCase.Expect.StderrFile == "" {
		if strings.TrimSpace(actualStderr) != "" {
			t.Fatalf("stderr must be empty, got: %q", actualStderr)
		}
		return
	}

	expectedStderrRaw, err := os.ReadFile(filepath.Join(caseDir, testCase.Expect.StderrFile))
	if err != nil {
		t.Fatalf("read expected stderr: %v", err)
	}

	if actualStderr != string(expectedStderrRaw) {
		t.Fatalf("stderr mismatch:\nexpected:\n%s\nactual:\n%s", string(expectedStderrRaw), actualStderr)
	}
}

func assertResponse(t *testing.T, caseDir string, testCase integrationCase, format string, actualOutput []byte) {
	t.Helper()

	expectedRaw, err := os.ReadFile(filepath.Join(caseDir, testCase.Expect.ResponseFile))
	if err != nil {
		t.Fatalf("read expected response: %v", err)
	}

	actualValue, err := parseActualResponse(format, actualOutput)
	if err != nil {
		t.Fatalf("decode actual response: %v", err)
	}

	expectedValue, err := parseExpectedResponse(format, expectedRaw)
	if err != nil {
		t.Fatalf("decode expected response: %v", err)
	}

	if !reflect.DeepEqual(actualValue, expectedValue) {
		t.Fatalf(
			"response mismatch:\nexpected:\n%s\nactual:\n%s",
			mustJSON(expectedValue),
			mustJSON(actualValue),
		)
	}
}

func parseActualResponse(format string, raw []byte) (any, error) {
	switch format {
	case "ndjson":
		return parseNDJSON(raw)
	default:
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		return value, nil
	}
}

func parseExpectedResponse(format string, raw []byte) (any, error) {
	var value any
	if format == "ndjson" {
		value = []any{}
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func parseNDJSON(raw []byte) ([]any, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return []any{}, nil
	}

	lines := strings.Split(trimmed, "\n")
	records := make([]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var record any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, nil
}

func outputFormat(args []string) string {
	format := "json"
	for idx, arg := range args {
		if arg == "--format" && idx+1 < len(args) {
			format = args[idx+1]
			continue
		}
		if strings.HasPrefix(arg, "--format=") {
			format = strings.TrimPrefix(arg, "--format=")
		}
	}
	return format
}

func replacePlaceholders(args []string, workspace string, schema string) []string {
	replacer := strings.NewReplacer(
		"${WORKSPACE}", workspace,
		"${SCHEMA}", schema,
	)

	replaced := make([]string, len(args))
	for idx, arg := range args {
		replaced[idx] = replacer.Replace(arg)
	}
	return replaced
}

func copyDir(from string, to string) error {
	return filepath.WalkDir(from, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relative, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(to, relative)

		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		return copyFile(path, targetPath)
	})
}

func copyFile(from string, to string) error {
	content, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	return os.WriteFile(to, content, 0o644)
}

func mustJSON(value any) string {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprintf("marshal error: %v", err)
	}
	return string(raw)
}
