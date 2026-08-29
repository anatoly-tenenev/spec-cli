// Package payload builds the response blocks shared by every command: the
// error object and the top-level schema block. The schema block is attached
// only for schema-related failures, so a runtime error never carries schema
// diagnostics that did not cause it.
package payload

import (
	schemacompile "github.com/anatoly-tenenev/spec-cli/internal/application/schema/compile"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/responses"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	"github.com/anatoly-tenenev/spec-cli/internal/output/errormap"
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

func ShouldIncludeSchemaForError(code domainerrors.Code) bool {
	switch code {
	case domainerrors.CodeSchemaNotFound,
		domainerrors.CodeSchemaReadError,
		domainerrors.CodeSchemaParseError,
		domainerrors.CodeSchemaInvalid,
		domainerrors.CodeSchemaProjectionError:
		return true
	default:
		return false
	}
}

// BuildErrorOutput assembles the whole response for a command that failed after
// its schema compiled: the error object, the schema block when the failure was
// schema-related, and the exit code the error carries.
//
// Every command that compiles a schema needs exactly this, so the shape of a
// failed response does not depend on which command produced it.
func BuildErrorOutput(appErr *domainerrors.AppError, schemaPayload map[string]any) responses.CommandOutput {
	jsonPayload := map[string]any{
		"result_state": errormap.ResultStateForCode(appErr.Code),
		"error":        BuildErrorPayload(appErr),
	}
	if ShouldIncludeSchemaForError(appErr.Code) {
		jsonPayload["schema"] = schemaPayload
	}

	return responses.CommandOutput{
		JSON:     jsonPayload,
		ExitCode: appErr.ExitCode,
	}
}
