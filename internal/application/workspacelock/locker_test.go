//go:build unix

package workspacelock

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

const (
	unitHelperEnvEnabled   = "SPEC_CLI_TEST_WORKSPACE_LOCK_HELPER"
	unitHelperEnvWorkspace = "SPEC_CLI_TEST_WORKSPACE_LOCK_HELPER_WORKSPACE"
	unitHelperReadyLine    = "LOCK_READY"
)

func TestAcquireExclusiveRelease(t *testing.T) {
	workspacePath := t.TempDir()

	guard, appErr := AcquireExclusive(workspacePath)
	if appErr != nil {
		t.Fatalf("unexpected acquire error: %v", appErr)
	}
	if guard == nil {
		t.Fatalf("expected non-nil guard")
	}

	guard.Release()
	guard.Release()
}

func TestAcquireExclusiveContention(t *testing.T) {
	workspacePath := t.TempDir()
	releaseHolder := startUnitLockHolderProcess(t, workspacePath)
	defer releaseHolder()

	guard, appErr := AcquireExclusive(workspacePath)
	if guard != nil {
		guard.Release()
		t.Fatalf("expected nil guard on lock contention")
	}
	if appErr == nil {
		t.Fatalf("expected lock contention error")
	}
	if appErr.Code != domainerrors.CodeConcurrencyConflict {
		t.Fatalf("expected %s, got %s", domainerrors.CodeConcurrencyConflict, appErr.Code)
	}
	if appErr.Message != "workspace is locked by another mutating operation" {
		t.Fatalf("unexpected lock contention message: %q", appErr.Message)
	}
}

func TestAcquireExclusiveAfterRelease(t *testing.T) {
	workspacePath := t.TempDir()

	firstGuard, firstErr := AcquireExclusive(workspacePath)
	if firstErr != nil {
		t.Fatalf("first acquire failed: %v", firstErr)
	}
	firstGuard.Release()

	secondGuard, secondErr := AcquireExclusive(workspacePath)
	if secondErr != nil {
		t.Fatalf("second acquire failed: %v", secondErr)
	}
	secondGuard.Release()
}

func TestWorkspaceLockHelperProcess(t *testing.T) {
	if os.Getenv(unitHelperEnvEnabled) != "1" {
		return
	}

	workspacePath := strings.TrimSpace(os.Getenv(unitHelperEnvWorkspace))
	if workspacePath == "" {
		t.Fatalf("helper workspace path is required")
	}

	guard, appErr := AcquireExclusive(workspacePath)
	if appErr != nil {
		t.Fatalf("helper failed to acquire lock: %v", appErr)
	}
	defer guard.Release()

	if _, err := io.WriteString(os.Stdout, unitHelperReadyLine+"\n"); err != nil {
		t.Fatalf("helper failed to signal readiness: %v", err)
	}
	if _, err := io.ReadAll(os.Stdin); err != nil {
		t.Fatalf("helper failed to wait for release signal: %v", err)
	}
}

func startUnitLockHolderProcess(t *testing.T, workspacePath string) func() {
	t.Helper()

	executablePath, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable path: %v", err)
	}

	cmd := exec.Command(executablePath, "-test.run", "^TestWorkspaceLockHelperProcess$")
	cmd.Env = append(
		os.Environ(),
		unitHelperEnvEnabled+"=1",
		unitHelperEnvWorkspace+"="+workspacePath,
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
		if readyLine != unitHelperReadyLine {
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
