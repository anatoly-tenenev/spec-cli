package delete

import (
	"context"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/delete/internal/engine"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/delete/internal/options"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/delete/internal/workspace"
	schemacapreferences "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/references"
	schemacompile "github.com/anatoly-tenenev/spec-cli/internal/application/schema/compile"
	"github.com/anatoly-tenenev/spec-cli/internal/application/workspacelock"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/responses"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	"github.com/anatoly-tenenev/spec-cli/internal/output/errormap"
	outputpayload "github.com/anatoly-tenenev/spec-cli/internal/output/payload"
)

type Handler struct {
	newCompiler func() *schemacompile.Compiler
}

func NewHandler() *Handler {
	return &Handler{newCompiler: schemacompile.NewCompiler}
}

func (h *Handler) Handle(_ context.Context, request requests.Command) (responses.CommandOutput, *domainerrors.AppError) {
	opts, parseErr := options.Parse(request.Args)
	if parseErr != nil {
		return responses.CommandOutput{}, parseErr
	}

	workspacePath, schemaPath, pathErr := options.NormalizePaths(request.Global)
	if pathErr != nil {
		return responses.CommandOutput{}, pathErr
	}

	lockGuard, lockErr := workspacelock.AcquireExclusive(workspacePath)
	if lockErr != nil {
		return responses.CommandOutput{}, lockErr
	}
	defer lockGuard.Release()

	compiler := h.newCompiler()
	compileResult, compileErr := compiler.Compile(schemaPath, request.Global.SchemaPath)
	schemaPayload := outputpayload.BuildSchemaPayload(compileResult)
	if compileErr != nil {
		return buildErrorWithSchema(compileErr, schemaPayload), nil
	}
	referencesCapability := schemacapreferences.Build(compileResult.Schema)

	snapshot, snapshotErr := workspace.BuildSnapshot(workspacePath, opts.ID)
	if snapshotErr != nil {
		return buildErrorWithSchema(snapshotErr, schemaPayload), nil
	}

	payload, executeErr := engine.Execute(opts, referencesCapability, snapshot)
	if executeErr != nil {
		return buildErrorWithSchema(executeErr, schemaPayload), nil
	}
	payload["schema"] = schemaPayload

	return responses.CommandOutput{JSON: payload}, nil
}

func buildErrorWithSchema(appErr *domainerrors.AppError, schemaPayload map[string]any) responses.CommandOutput {
	return responses.CommandOutput{
		JSON: map[string]any{
			"result_state": errormap.ResultStateForCode(appErr.Code),
			"schema":       schemaPayload,
			"error":        outputpayload.BuildErrorPayload(appErr),
		},
		ExitCode: appErr.ExitCode,
	}
}
