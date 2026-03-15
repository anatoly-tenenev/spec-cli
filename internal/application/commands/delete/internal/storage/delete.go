package storage

import (
	"os"
	"strings"

	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

const deleteFailureInjectEnv = "SPEC_CLI_TEST_INJECT_DELETE_FAILURE"
const deleteFailureInjectModeBeforeCommit = "before_remove_commit"

func Delete(path string) *domainerrors.AppError {
	if shouldInjectDeleteFailure() {
		return injectedDeleteFailureError("injected delete failure before remove commit")
	}

	if err := os.Remove(path); err != nil {
		return domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to delete target entity document",
			map[string]any{"reason": classifyDeleteReason(err)},
		)
	}
	return nil
}

func shouldInjectDeleteFailure() bool {
	return strings.EqualFold(
		strings.TrimSpace(os.Getenv(deleteFailureInjectEnv)),
		deleteFailureInjectModeBeforeCommit,
	)
}

func injectedDeleteFailureError(message string) *domainerrors.AppError {
	return domainerrors.New(
		domainerrors.CodeWriteFailed,
		message,
		map[string]any{"reason": message},
	)
}

func classifyDeleteReason(err error) string {
	if os.IsPermission(err) {
		return "permission denied"
	}
	if os.IsNotExist(err) {
		return "target file is missing"
	}
	return "filesystem delete failed"
}
