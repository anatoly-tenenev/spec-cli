package storage

import (
	"os"
	"path/filepath"

	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func WriteAtomically(targetPath string, payload []byte) *domainerrors.AppError {
	parentDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to create parent directories for target path",
			map[string]any{"reason": err.Error()},
		)
	}

	tempFile, err := os.CreateTemp(parentDir, ".spec-cli-add-*.tmp")
	if err != nil {
		return domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to create temporary file",
			map[string]any{"reason": err.Error()},
		)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if _, err := tempFile.Write(payload); err != nil {
		_ = tempFile.Close()
		return domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to write temporary file",
			map[string]any{"reason": err.Error()},
		)
	}
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to flush temporary file",
			map[string]any{"reason": err.Error()},
		)
	}
	if err := tempFile.Close(); err != nil {
		return domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to close temporary file",
			map[string]any{"reason": err.Error()},
		)
	}

	if err := os.Rename(tempPath, targetPath); err != nil {
		return domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to atomically write target file",
			map[string]any{"reason": err.Error()},
		)
	}

	return nil
}

func IsPathConflict(targetPath string, existingPaths map[string]struct{}) bool {
	if targetPath == "" {
		return false
	}

	cleanTarget := filepath.Clean(targetPath)
	if _, exists := existingPaths[cleanTarget]; exists {
		return true
	}
	if _, err := os.Stat(cleanTarget); err == nil {
		return true
	}

	return false
}
