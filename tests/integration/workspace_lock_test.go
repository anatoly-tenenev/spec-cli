//go:build unix

package integration_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/application/workspacelock"
)

const (
	integrationLockHelperEnabledEnv   = "SPEC_CLI_TEST_INTEGRATION_LOCK_HELPER"
	integrationLockHelperWorkspaceEnv = "SPEC_CLI_TEST_INTEGRATION_LOCK_WORKSPACE"
	integrationLockHelperReadyLine    = "LOCK_READY"
)

func TestMutatingCommandsLockConflict(t *testing.T) {
	testCases := []struct {
		name    string
		caseDir string
	}{
		{
			name:    "add",
			caseDir: filepath.Join("cases", "workspace_lock", "10_conflict", "0001_err_workspace_lock_add_json"),
		},
		{
			name:    "update",
			caseDir: filepath.Join("cases", "workspace_lock", "10_conflict", "0002_err_workspace_lock_update_json"),
		},
		{
			name:    "delete",
			caseDir: filepath.Join("cases", "workspace_lock", "10_conflict", "0003_err_workspace_lock_delete_json"),
		},
	}

	for _, tc := range testCases {
		current := tc
		t.Run(current.name, func(t *testing.T) {
			runWorkspaceLockConflictCase(t, current.caseDir)
		})
	}
}

func TestMutatingCommandsDryRunRespectsWorkspaceLock(t *testing.T) {
	testCases := []struct {
		name    string
		caseDir string
	}{
		{
			name:    "add_dry_run",
			caseDir: filepath.Join("cases", "workspace_lock", "10_conflict", "0004_err_workspace_lock_add_dry_run_json"),
		},
		{
			name:    "update_dry_run",
			caseDir: filepath.Join("cases", "workspace_lock", "10_conflict", "0005_err_workspace_lock_update_dry_run_json"),
		},
		{
			name:    "delete_dry_run",
			caseDir: filepath.Join("cases", "workspace_lock", "10_conflict", "0006_err_workspace_lock_delete_dry_run_json"),
		},
	}

	for _, tc := range testCases {
		current := tc
		t.Run(current.name, func(t *testing.T) {
			runWorkspaceLockConflictCase(t, current.caseDir)
		})
	}
}

func runWorkspaceLockConflictCase(t *testing.T, caseDir string) {
	t.Helper()

	testCase, err := loadCase(caseDir)
	if err != nil {
		t.Fatalf("load case: %v", err)
	}

	tempRoot := t.TempDir()
	workspacePath := filepath.Join(tempRoot, "workspace")
	if err := copyDir(filepath.Join(caseDir, testCase.Workspace.InputDir), workspacePath); err != nil {
		t.Fatalf("copy workspace.in: %v", err)
	}

	schemaPath := filepath.Join(tempRoot, "spec.schema.yaml")
	if err := copyFile(filepath.Join(caseDir, "spec.schema.yaml"), schemaPath); err != nil {
		t.Fatalf("copy spec.schema.yaml: %v", err)
	}

	beforeFiles, err := collectWorkspaceFiles(workspacePath)
	if err != nil {
		t.Fatalf("collect workspace files before run: %v", err)
	}

	releaseLock := startIntegrationLockHolderProcess(t, workspacePath)
	defer releaseLock()

	args := replacePlaceholders(testCase.Args, workspacePath, schemaPath)
	execResult, runErr := runCLIProcess(context.Background(), args, testCase.Runtime.FixedNowUTC, "", testCase.Runtime.Env)
	if runErr != nil {
		t.Fatalf("run command: %v", runErr)
	}

	if execResult.ExitCode != 1 {
		t.Fatalf(
			"expected exit code 1 on lock contention, got %d\nstdout:\n%s\nstderr:\n%s",
			execResult.ExitCode,
			execResult.Stdout,
			execResult.Stderr,
		)
	}

	assertJSONError(t, execResult.Stdout, "invalid", "CONCURRENCY_CONFLICT", 1)

	var payload map[string]any
	if err := json.Unmarshal([]byte(execResult.Stdout), &payload); err != nil {
		t.Fatalf("decode json output: %v", err)
	}
	errorPayload, _ := payload["error"].(map[string]any)
	if errorPayload == nil {
		t.Fatalf("missing error payload")
	}

	message, _ := errorPayload["message"].(string)
	if message != "workspace is locked by another mutating operation" {
		t.Fatalf("unexpected lock conflict message: %q", message)
	}
	if _, hasDetails := errorPayload["details"]; hasDetails {
		t.Fatalf("lock conflict must not expose internal details: %s", execResult.Stdout)
	}

	afterFiles, err := collectWorkspaceFiles(workspacePath)
	if err != nil {
		t.Fatalf("collect workspace files after run: %v", err)
	}
	if !reflect.DeepEqual(beforeFiles, afterFiles) {
		t.Fatalf(
			"workspace must stay unchanged on lock conflict\nbefore:\n%s\nafter:\n%s",
			mustJSON(beforeFiles),
			mustJSON(afterFiles),
		)
	}
}

func TestIntegrationWorkspaceLockHolderProcess(t *testing.T) {
	if os.Getenv(integrationLockHelperEnabledEnv) != "1" {
		return
	}

	workspacePath := strings.TrimSpace(os.Getenv(integrationLockHelperWorkspaceEnv))
	if workspacePath == "" {
		t.Fatalf("helper workspace path is required")
	}

	lockGuard, lockErr := workspacelock.AcquireExclusive(workspacePath)
	if lockErr != nil {
		t.Fatalf("helper failed to acquire workspace lock: %v", lockErr)
	}
	defer lockGuard.Release()

	if _, err := io.WriteString(os.Stdout, integrationLockHelperReadyLine+"\n"); err != nil {
		t.Fatalf("helper failed to signal readiness: %v", err)
	}
	if _, err := io.ReadAll(os.Stdin); err != nil {
		t.Fatalf("helper failed to wait for release signal: %v", err)
	}
}

func startIntegrationLockHolderProcess(t *testing.T, workspacePath string) func() {
	t.Helper()

	executablePath, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable path: %v", err)
	}

	cmd := exec.Command(executablePath, "-test.run", "^TestIntegrationWorkspaceLockHolderProcess$")
	cmd.Env = append(
		os.Environ(),
		integrationLockHelperEnabledEnv+"=1",
		integrationLockHelperWorkspaceEnv+"="+workspacePath,
	)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("helper stdout pipe: %v", err)
	}
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("helper stdin pipe: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper process: %v", err)
	}

	readyCh := make(chan string, 1)
	readErrCh := make(chan error, 1)
	go func() {
		line, lineErr := bufio.NewReader(stdoutPipe).ReadString('\n')
		if lineErr != nil {
			readErrCh <- lineErr
			return
		}
		readyCh <- strings.TrimSpace(line)
	}()

	select {
	case readyLine := <-readyCh:
		if readyLine != integrationLockHelperReadyLine {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			t.Fatalf("helper unexpected ready line: %q; stderr: %s", readyLine, stderr.String())
		}
	case readErr := <-readErrCh:
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("helper failed before readiness: %v; stderr: %s", readErr, stderr.String())
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("helper readiness timeout; stderr: %s", stderr.String())
	}

	released := false
	release := func() {
		if released {
			return
		}
		released = true
		_ = stdinPipe.Close()
		if waitErr := cmd.Wait(); waitErr != nil {
			t.Fatalf("helper exited with error: %v; stderr: %s", waitErr, stderr.String())
		}
	}
	t.Cleanup(release)
	return release
}
