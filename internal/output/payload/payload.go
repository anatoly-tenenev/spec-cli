package payload

import (
	schemacompile "github.com/anatoly-tenenev/spec-cli/internal/application/schema/compile"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func BuildSchemaPayload(result schemacompile.Result) map[string]any {
	return map[string]any{
		"valid":   result.Valid,
		"summary": result.Summary,
		"issues":  result.Issues,
	}
}

func BuildErrorPayload(appErr *domainerrors.AppError) map[string]any {
	if appErr == nil {
		return map[string]any{}
	}

	errorPayload := map[string]any{
		"code":      appErr.Code,
		"message":   appErr.Message,
		"exit_code": appErr.ExitCode,
	}
	if len(appErr.Details) > 0 {
		errorPayload["details"] = appErr.Details
	}

	return errorPayload
}
