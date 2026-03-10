package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
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
	caseDirs, err := listValidateCaseDirs(caseRoot)
	if err != nil {
		t.Fatalf("list validate case directories: %v", err)
	}

	for _, caseDir := range caseDirs {
		testCase, err := loadCase(caseDir)
		if err != nil {
			t.Fatalf("load case %s: %v", caseDir, err)
		}
		if err := validateCaseNaming(caseDir, testCase); err != nil {
			t.Fatalf("validate case naming %s: %v", caseDir, err)
		}

		tc := testCase
		t.Run(tc.ID, func(t *testing.T) {
			runCase(t, caseDir, tc)
		})
	}
}

func listValidateCaseDirs(caseRoot string) ([]string, error) {
	groupEntries, err := os.ReadDir(caseRoot)
	if err != nil {
		return nil, err
	}

	groupNames := make([]string, 0, len(groupEntries))
	for _, entry := range groupEntries {
		if entry.IsDir() {
			groupNames = append(groupNames, entry.Name())
		}
	}
	sort.Strings(groupNames)

	caseDirs := make([]string, 0)
	for _, groupName := range groupNames {
		groupDir := filepath.Join(caseRoot, groupName)
		caseEntries, err := os.ReadDir(groupDir)
		if err != nil {
			return nil, fmt.Errorf("read group directory %s: %w", groupDir, err)
		}

		caseNames := make([]string, 0, len(caseEntries))
		for _, entry := range caseEntries {
			if entry.IsDir() {
				caseNames = append(caseNames, entry.Name())
			}
		}
		sort.Strings(caseNames)

		for _, caseName := range caseNames {
			caseDirs = append(caseDirs, filepath.Join(groupDir, caseName))
		}
	}

	return caseDirs, nil
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

func validateCaseNaming(caseDir string, testCase integrationCase) error {
	caseName := filepath.Base(caseDir)
	caseParts := strings.SplitN(caseName, "_", 3)
	if len(caseParts) != 3 {
		return fmt.Errorf("case directory must match <NNNN>_<ok|err>_<case-id>, got %q", caseName)
	}

	caseNumber := caseParts[0]
	if len(caseNumber) != 4 || !isDigits(caseNumber) {
		return fmt.Errorf("case directory must start with 4-digit number, got %q", caseName)
	}

	outcome := caseParts[1]
	switch outcome {
	case "ok":
		if testCase.Expect.ExitCode != 0 {
			return fmt.Errorf("case outcome prefix %q requires exit_code 0, got %d", outcome, testCase.Expect.ExitCode)
		}
	case "err":
		if testCase.Expect.ExitCode == 0 {
			return fmt.Errorf("case outcome prefix %q requires non-zero exit_code", outcome)
		}
	default:
		return fmt.Errorf("case outcome prefix must be ok|err, got %q", outcome)
	}

	format, err := caseOutputFormat(testCase.Args)
	if err != nil {
		return err
	}
	if format == "json" && !strings.HasSuffix(caseName, "_json") {
		return fmt.Errorf("case directory for --format json must end with _json, got %q", caseName)
	}

	groupName := filepath.Base(filepath.Dir(caseDir))
	groupParts := strings.SplitN(groupName, "_", 2)
	if len(groupParts) != 2 || len(groupParts[0]) != 2 || !isDigits(groupParts[0]) {
		return fmt.Errorf("group directory must match <GG>_<group-name>, got %q", groupName)
	}

	expectedID := fmt.Sprintf("validate_%s_%s", groupParts[0], caseName)
	if testCase.ID != expectedID {
		return fmt.Errorf("case id mismatch: expected %q, got %q", expectedID, testCase.ID)
	}

	return nil
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, symbol := range value {
		if symbol < '0' || symbol > '9' {
			return false
		}
	}
	return true
}

func caseOutputFormat(args []string) (string, error) {
	for idx := 0; idx < len(args); idx++ {
		if args[idx] != "--format" {
			continue
		}
		if idx+1 >= len(args) {
			return "", fmt.Errorf("case args: --format requires a value")
		}

		format := args[idx+1]
		switch format {
		case "json":
			return format, nil
		default:
			return "", fmt.Errorf("case args: unsupported --format value %q", format)
		}
	}

	return "json", nil
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
	assertResponse(t, caseDir, testCase, stdout.Bytes())
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

func assertResponse(t *testing.T, caseDir string, testCase integrationCase, actualOutput []byte) {
	t.Helper()

	expectedRaw, err := os.ReadFile(filepath.Join(caseDir, testCase.Expect.ResponseFile))
	if err != nil {
		t.Fatalf("read expected response: %v", err)
	}

	actualValue, err := parseJSON(actualOutput)
	if err != nil {
		t.Fatalf("decode actual response: %v", err)
	}

	expectedValue, err := parseJSON(expectedRaw)
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

func parseJSON(raw []byte) (any, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
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
