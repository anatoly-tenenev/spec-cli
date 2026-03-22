//go:build unix

package flock

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

const (
	lockBusyMessage        = "workspace is locked by another mutating operation"
	lockUnsupportedMessage = "workspace lock is not supported on this platform"
)

type Lock struct {
	file *os.File
}

func AcquireExclusive(lockPath string) (*Lock, *domainerrors.AppError) {
	lockDir := filepath.Dir(lockPath)
	workspacePath := filepath.Dir(lockDir)
	workspaceInfo, workspaceErr := os.Stat(workspacePath)
	if workspaceErr != nil {
		return nil, domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to access workspace for lock acquisition",
			map[string]any{"reason": classifyIOReason(workspaceErr)},
		)
	}
	if !workspaceInfo.IsDir() {
		return nil, domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to access workspace for lock acquisition",
			map[string]any{"reason": "workspace path is not a directory"},
		)
	}

	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return nil, domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to prepare workspace lock directory",
			map[string]any{"reason": classifyIOReason(err)},
		)
	}

	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to open workspace lock file",
			map[string]any{"reason": classifyIOReason(err)},
		)
	}

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lockFile.Close()
		if isLockBusy(err) {
			return nil, domainerrors.New(domainerrors.CodeConcurrencyConflict, lockBusyMessage, nil)
		}
		if isCapabilityUnsupported(err) {
			return nil, domainerrors.New(domainerrors.CodeCapabilityUnsupported, lockUnsupportedMessage, nil)
		}

		return nil, domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to acquire workspace lock",
			map[string]any{"reason": "lock acquisition failed"},
		)
	}

	return &Lock{file: lockFile}, nil
}

func (l *Lock) Release() {
	if l == nil || l.file == nil {
		return
	}

	_ = syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	_ = l.file.Close()
	l.file = nil
}

func isLockBusy(err error) bool {
	return errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN)
}

func isCapabilityUnsupported(err error) bool {
	return errors.Is(err, syscall.ENOTSUP) || errors.Is(err, syscall.ENOSYS)
}

func classifyIOReason(err error) string {
	switch {
	case errors.Is(err, os.ErrPermission):
		return "permission denied"
	case errors.Is(err, os.ErrNotExist):
		return "not found"
	default:
		return "i/o error"
	}
}
