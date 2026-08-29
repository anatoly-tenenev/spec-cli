// Package storage persists the updated document. Rewriting in place goes
// through a temporary file and a rename, so an interrupted run never leaves a
// half-written entity. When the schema-computed path changed, the new content
// is staged first, the old file is moved aside to a backup, and only then
// renamed into place; a failure at that point restores the backup, so the
// entity is never left in two places or in none.
//
// SPEC_CLI_TEST_INJECT_WRITE_FAILURE makes the commit fail on purpose: the
// integration suite is black-box and cannot make a real filesystem fail on
// demand, and what update does after a failed write is part of the contract.
package storage

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/entitydoc"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

const writeFailureInjectEnv = "SPEC_CLI_TEST_INJECT_WRITE_FAILURE"
const writeFailureInjectModeAfterValidateBeforeCommit = "after_validate_before_commit"

func Persist(sourcePath string, targetPath string, payload []byte) *domainerrors.AppError {
	cleanSource := filepath.Clean(sourcePath)
	cleanTarget := filepath.Clean(targetPath)
	if cleanSource == cleanTarget {
		if shouldInjectWriteFailure() {
			return injectedWriteFailureError("injected write failure before atomic write commit", nil)
		}
		return WriteAtomically(cleanTarget, payload)
	}
	return WriteWithMove(cleanSource, cleanTarget, payload)
}

func WriteAtomically(targetPath string, payload []byte) *domainerrors.AppError {
	parentDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to create parent directories for target path",
			entitydoc.IOFailureDetails(err),
		)
	}

	tempPath, writeErr := writeTempFile(parentDir, payload, ".spec-cli-update-*.tmp")
	if writeErr != nil {
		return writeErr
	}
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if err := os.Rename(tempPath, targetPath); err != nil {
		return domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to atomically write target file",
			entitydoc.IOFailureDetails(err),
		)
	}

	return nil
}

func WriteWithMove(sourcePath string, targetPath string, payload []byte) *domainerrors.AppError {
	targetParent := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetParent, 0o755); err != nil {
		return domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to create parent directories for target path",
			entitydoc.IOFailureDetails(err),
		)
	}

	tempPath, writeErr := writeTempFile(targetParent, payload, ".spec-cli-update-*.tmp")
	if writeErr != nil {
		return writeErr
	}
	defer func() {
		_ = os.Remove(tempPath)
	}()

	backupPath, backupErr := reserveTempPath(filepath.Dir(sourcePath), ".spec-cli-update-backup-*.tmp")
	if backupErr != nil {
		return backupErr
	}
	defer func() {
		_ = os.Remove(backupPath)
	}()

	if err := os.Rename(sourcePath, backupPath); err != nil {
		return domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to prepare source file move",
			entitydoc.IOFailureDetails(err),
		)
	}
	if shouldInjectWriteFailure() {
		rollbackErr := os.Rename(backupPath, sourcePath)
		if rollbackErr != nil {
			return injectedWriteFailureError(
				"injected write failure after source backup before move commit; rollback failed",
				rollbackErr,
			)
		}
		return injectedWriteFailureError("injected write failure after source backup before move commit", nil)
	}

	if err := os.Rename(tempPath, targetPath); err != nil {
		rollbackErr := os.Rename(backupPath, sourcePath)
		if rollbackErr != nil {
			return domainerrors.New(
				domainerrors.CodeWriteFailed,
				"failed to move updated document and rollback source",
				map[string]any{
					"reason":          err.Error(),
					"rollback_reason": rollbackErr.Error(),
				},
			)
		}
		return domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to move updated document to target path",
			map[string]any{"reason": err.Error()},
		)
	}

	if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
		return domainerrors.New(
			domainerrors.CodeWriteFailed,
			"updated document moved but failed to cleanup source backup",
			entitydoc.IOFailureDetails(err),
		)
	}

	return nil
}

func IsPathConflict(targetPath string, existingPaths map[string]struct{}, ignorePath string) bool {
	if targetPath == "" {
		return false
	}

	cleanTarget := filepath.Clean(targetPath)
	cleanIgnore := filepath.Clean(ignorePath)
	if cleanTarget != cleanIgnore {
		if _, exists := existingPaths[cleanTarget]; exists {
			return true
		}
	}
	if info, err := os.Stat(cleanTarget); err == nil && (cleanTarget != cleanIgnore || info != nil) {
		if cleanTarget != cleanIgnore {
			return true
		}
	}

	return false
}

func writeTempFile(parentDir string, payload []byte, pattern string) (string, *domainerrors.AppError) {
	tempFile, err := os.CreateTemp(parentDir, pattern)
	if err != nil {
		return "", domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to create temporary file",
			entitydoc.IOFailureDetails(err),
		)
	}
	tempPath := tempFile.Name()

	if _, err := tempFile.Write(payload); err != nil {
		_ = tempFile.Close()
		return "", domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to write temporary file",
			entitydoc.IOFailureDetails(err),
		)
	}
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return "", domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to flush temporary file",
			entitydoc.IOFailureDetails(err),
		)
	}
	if err := tempFile.Close(); err != nil {
		return "", domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to close temporary file",
			entitydoc.IOFailureDetails(err),
		)
	}

	return tempPath, nil
}

func reserveTempPath(parentDir string, pattern string) (string, *domainerrors.AppError) {
	tempFile, err := os.CreateTemp(parentDir, pattern)
	if err != nil {
		return "", domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to create temporary rollback file path",
			entitydoc.IOFailureDetails(err),
		)
	}
	path := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		return "", domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to close temporary rollback file path",
			entitydoc.IOFailureDetails(err),
		)
	}
	if err := os.Remove(path); err != nil {
		return "", domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to prepare temporary rollback file path",
			entitydoc.IOFailureDetails(err),
		)
	}
	return path, nil
}

func shouldInjectWriteFailure() bool {
	return strings.EqualFold(
		strings.TrimSpace(os.Getenv(writeFailureInjectEnv)),
		writeFailureInjectModeAfterValidateBeforeCommit,
	)
}

func injectedWriteFailureError(message string, rollbackErr error) *domainerrors.AppError {
	details := map[string]any{"reason": message}
	if rollbackErr != nil {
		details["rollback_reason"] = rollbackErr.Error()
	}
	return domainerrors.New(domainerrors.CodeWriteFailed, message, details)
}
