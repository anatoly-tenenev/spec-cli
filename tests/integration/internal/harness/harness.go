package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	integrationrunner "github.com/anatoly-tenenev/spec-cli/tests/integration/internal/runner"
)

type Case struct {
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
		InputDir     string                `json:"input_dir"`
		OutputDir    string                `json:"output_dir"`
		AssertOutput bool                  `json:"assert_output"`
		Permissions  []WorkspacePermission `json:"permissions"`
	} `json:"workspace"`
	Runtime struct {
		FixedNowUTC string            `json:"fixed_now_utc"`
		StdinFile   string            `json:"stdin_file"`
		Env         map[string]string `json:"env"`
		CWD         string            `json:"cwd"`
	} `json:"runtime"`
}

type WorkspacePermission struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
}

type CLIExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

var (
	buildCLIBinaryOnce sync.Once
	cliBinaryPath      string
	cliBinaryBuildErr  error
	repoRootOnce       sync.Once
	cachedRepoRoot     string
	repoRootErr        error
)

func LoadCase(caseDir string) (Case, error) {
	casePath := filepath.Join(caseDir, "case.json")
	raw, err := os.ReadFile(casePath)
	if err != nil {
		return Case{}, fmt.Errorf("read case.json: %w", err)
	}

	var testCase Case
	if err := json.Unmarshal(raw, &testCase); err != nil {
		return Case{}, fmt.Errorf("decode case.json: %w", err)
	}
	return testCase, nil
}

func RunCase(t *testing.T, caseDir string, testCase Case) {
	t.Helper()

	tempRoot := t.TempDir()
	workspacePath := filepath.Join(tempRoot, "workspace")
	if err := CopyDir(filepath.Join(caseDir, testCase.Workspace.InputDir), workspacePath); err != nil {
		t.Fatalf("copy workspace.in: %v", err)
	}
	restorePermissions := func() {}
	if len(testCase.Workspace.Permissions) > 0 {
		rollback, err := ApplyWorkspacePermissions(workspacePath, testCase.Workspace.Permissions)
		if err != nil {
			t.Fatalf("apply workspace permissions: %v", err)
		}
		restorePermissions = rollback
	}
	defer restorePermissions()

	schemaPath := filepath.Join(tempRoot, "spec.schema.yaml")
	if err := CopyFile(filepath.Join(caseDir, "spec.schema.yaml"), schemaPath); err != nil {
		t.Fatalf("copy spec.schema.yaml: %v", err)
	}

	args := ReplacePlaceholders(testCase.Args, workspacePath, schemaPath)
	runCWD := ReplaceRuntimePlaceholder(testCase.Runtime.CWD, workspacePath, schemaPath)
	stdinValue := ""
	if strings.TrimSpace(testCase.Runtime.StdinFile) != "" {
		stdinRaw, readErr := os.ReadFile(filepath.Join(caseDir, testCase.Runtime.StdinFile))
		if readErr != nil {
			t.Fatalf("read stdin file: %v", readErr)
		}
		stdinValue = string(stdinRaw)
	}

	execResult, runErr := RunCLIProcessInDir(context.Background(), runCWD, args, testCase.Runtime.FixedNowUTC, stdinValue, testCase.Runtime.Env)
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

	AssertStderr(t, caseDir, testCase, execResult.Stderr)
	AssertResponse(t, caseDir, testCase, []byte(execResult.Stdout))
	AssertWorkspaceOutput(t, caseDir, testCase, workspacePath)
}

func RunCLIProcess(ctx context.Context, args []string, fixedNowUTC string, stdinValue string, extraEnv map[string]string) (CLIExecResult, error) {
	return RunCLIProcessInDir(ctx, "", args, fixedNowUTC, stdinValue, extraEnv)
}

func RunCLIProcessInDir(
	ctx context.Context,
	cwd string,
	args []string,
	fixedNowUTC string,
	stdinValue string,
	extraEnv map[string]string,
) (CLIExecResult, error) {
	binaryPath, binErr := ensureCLIBinary()
	if binErr != nil {
		return CLIExecResult{}, binErr
	}

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	if strings.TrimSpace(cwd) != "" {
		cmd.Dir = cwd
	} else {
		repoRoot, rootErr := ensureRepoRoot()
		if rootErr != nil {
			return CLIExecResult{}, rootErr
		}
		cmd.Dir = repoRoot
	}

	env := append([]string{}, os.Environ()...)
	if strings.TrimSpace(fixedNowUTC) != "" {
		env = append(env, "SPEC_CLI_FIXED_NOW_UTC="+strings.TrimSpace(fixedNowUTC))
	}
	for key, value := range extraEnv {
		name := strings.TrimSpace(key)
		if name == "" {
			continue
		}
		env = append(env, name+"="+value)
	}
	cmd.Env = env

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if stdinValue != "" {
		cmd.Stdin = strings.NewReader(stdinValue)
	}

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return CLIExecResult{}, err
		}
		return CLIExecResult{
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: exitErr.ExitCode(),
		}, nil
	}

	return CLIExecResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}, nil
}

func ReplaceRuntimePlaceholder(raw string, workspace string, schema string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	return ReplacePlaceholders([]string{value}, workspace, schema)[0]
}

func ApplyWorkspacePermissions(workspacePath string, permissions []WorkspacePermission) (func(), error) {
	return integrationrunner.ApplyWorkspacePermissions(workspacePath, toRunnerPermissions(permissions))
}

func AssertStderr(t *testing.T, caseDir string, testCase Case, actualStderr string) {
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

func AssertResponse(t *testing.T, caseDir string, testCase Case, actualOutput []byte) {
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

	actualValue = integrationrunner.NormalizeResponseValue(actualValue)
	expectedValue = integrationrunner.NormalizeResponseValue(expectedValue)

	if !reflect.DeepEqual(actualValue, expectedValue) {
		t.Fatalf(
			"response mismatch:\nexpected:\n%s\nactual:\n%s",
			MustJSON(expectedValue),
			MustJSON(actualValue),
		)
	}
}

func AssertWorkspaceOutput(t *testing.T, caseDir string, testCase Case, actualWorkspacePath string) {
	t.Helper()

	if !testCase.Workspace.AssertOutput {
		return
	}
	if strings.TrimSpace(testCase.Workspace.OutputDir) == "" {
		t.Fatalf("workspace.output_dir is required when assert_output=true")
	}

	expectedWorkspacePath := filepath.Join(caseDir, testCase.Workspace.OutputDir)
	expectedFiles, err := CollectWorkspaceFiles(expectedWorkspacePath)
	if err != nil {
		t.Fatalf("collect expected workspace.out files: %v", err)
	}

	actualFiles, err := CollectWorkspaceFiles(actualWorkspacePath)
	if err != nil {
		t.Fatalf("collect actual workspace files: %v", err)
	}

	if !reflect.DeepEqual(expectedFiles, actualFiles) {
		t.Fatalf(
			"workspace output mismatch:\nexpected:\n%s\nactual:\n%s",
			MustJSON(expectedFiles),
			MustJSON(actualFiles),
		)
	}
}

func CollectWorkspaceFiles(root string) (map[string]string, error) {
	files := map[string]string{}

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}

		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		normalizedRelative := filepath.ToSlash(relative)
		if isInternalWorkspaceLockFile(normalizedRelative) {
			return nil
		}

		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[normalizedRelative] = string(raw)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

func ReplacePlaceholders(args []string, workspace string, schema string) []string {
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

func CopyDir(from string, to string) error {
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
		return CopyFile(path, targetPath)
	})
}

func CopyFile(from string, to string) error {
	content, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	return os.WriteFile(to, content, 0o644)
}

func MustJSON(value any) string {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprintf("marshal error: %v", err)
	}
	return string(raw)
}

func ensureCLIBinary() (string, error) {
	buildCLIBinaryOnce.Do(func() {
		repoRoot, rootErr := ensureRepoRoot()
		if rootErr != nil {
			cliBinaryBuildErr = rootErr
			return
		}

		tempDir, err := os.MkdirTemp("", "spec-cli-integration-bin-*")
		if err != nil {
			cliBinaryBuildErr = err
			return
		}

		binPath := filepath.Join(tempDir, "spec-cli")
		if runtime.GOOS == "windows" {
			binPath += ".exe"
		}
		buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/spec-cli")
		buildCmd.Dir = repoRoot
		goCache := filepath.Join(repoRoot, "tmp", "go-build")
		goModCache := filepath.Join(repoRoot, "tmp", "go-mod")
		if mkErr := os.MkdirAll(goCache, 0o755); mkErr != nil {
			cliBinaryBuildErr = mkErr
			return
		}
		if mkErr := os.MkdirAll(goModCache, 0o755); mkErr != nil {
			cliBinaryBuildErr = mkErr
			return
		}
		buildCmd.Env = append([]string{}, os.Environ()...)
		buildCmd.Env = append(buildCmd.Env, "GOCACHE="+goCache, "GOMODCACHE="+goModCache)
		buildOutput, err := buildCmd.CombinedOutput()
		if err != nil {
			cliBinaryBuildErr = fmt.Errorf("go build failed: %w\n%s", err, string(buildOutput))
			return
		}

		cliBinaryPath = binPath
	})

	if cliBinaryBuildErr != nil {
		return "", cliBinaryBuildErr
	}
	return cliBinaryPath, nil
}

func ensureRepoRoot() (string, error) {
	repoRootOnce.Do(func() {
		workingDir, err := os.Getwd()
		if err != nil {
			repoRootErr = err
			return
		}

		current := workingDir
		for {
			if _, statErr := os.Stat(filepath.Join(current, "go.mod")); statErr == nil {
				cachedRepoRoot = current
				return
			}

			parent := filepath.Dir(current)
			if parent == current {
				repoRootErr = fmt.Errorf("failed to find repository root from %s", workingDir)
				return
			}
			current = parent
		}
	})

	if repoRootErr != nil {
		return "", repoRootErr
	}
	return cachedRepoRoot, nil
}

func isInternalWorkspaceLockFile(relativePath string) bool {
	return relativePath == ".spec-cli/workspace.lock"
}

func parseJSON(raw []byte) (any, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func toRunnerPermissions(permissions []WorkspacePermission) []integrationrunner.WorkspacePermission {
	out := make([]integrationrunner.WorkspacePermission, 0, len(permissions))
	for _, permission := range permissions {
		out = append(out, integrationrunner.WorkspacePermission{
			Path: permission.Path,
			Mode: permission.Mode,
		})
	}
	return out
}
