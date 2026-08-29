// Package delete implements the delete command: remove one entity by id,
// provided nothing still points at it. The workspace lock is taken before the
// schema is compiled, so the snapshot the decision is made from is the same
// state the removal applies to.
//
// handler.go is the command entrypoint; help.go declares how the command
// describes itself to help.
package delete

import (
	"context"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/optionpaths"

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

	workspacePath, schemaPath, pathErr := optionpaths.Normalize(request.Global)
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
		return buildPostCompileError(compileErr, schemaPayload), nil
	}
	referencesCapability := schemacapreferences.Build(compileResult.Schema)

	snapshot, snapshotErr := workspace.BuildSnapshot(workspacePath, opts.ID)
	if snapshotErr != nil {
		return buildPostCompileError(snapshotErr, schemaPayload), nil
	}

	payload, executeErr := engine.Execute(opts, referencesCapability, snapshot)
	if executeErr != nil {
		return buildPostCompileError(executeErr, schemaPayload), nil
	}

	return responses.CommandOutput{JSON: payload}, nil
}

func buildPostCompileError(appErr *domainerrors.AppError, schemaPayload map[string]any) responses.CommandOutput {
	jsonPayload := map[string]any{
		"result_state": errormap.ResultStateForCode(appErr.Code),
		"error":        outputpayload.BuildErrorPayload(appErr),
	}
	if outputpayload.ShouldIncludeSchemaForError(appErr.Code) {
		jsonPayload["schema"] = schemaPayload
	}

	return responses.CommandOutput{
		JSON:     jsonPayload,
		ExitCode: appErr.ExitCode,
	}
}
