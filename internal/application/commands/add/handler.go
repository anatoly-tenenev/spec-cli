package add

import (
	"context"
	"fmt"
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/engine"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/options"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/workspace"
	schemacapwrite "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/write"
	schemacompile "github.com/anatoly-tenenev/spec-cli/internal/application/schema/compile"
	"github.com/anatoly-tenenev/spec-cli/internal/application/workspacelock"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/responses"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	"github.com/anatoly-tenenev/spec-cli/internal/output/errormap"
	outputpayload "github.com/anatoly-tenenev/spec-cli/internal/output/payload"
)

type Handler struct {
	now         func() time.Time
	newCompiler func() *schemacompile.Compiler
}

func NewHandler(now func() time.Time) *Handler {
	if now == nil {
		now = time.Now
	}
	return &Handler{now: now, newCompiler: schemacompile.NewCompiler}
}

func (h *Handler) Handle(_ context.Context, request requests.Command) (responses.CommandOutput, *domainerrors.AppError) {
	opts, parseErr := options.Parse(request.Args)
	if parseErr != nil {
		return responses.CommandOutput{}, parseErr
	}

	workspacePath, schemaPath, normalizedOpts, pathErr := options.NormalizePaths(request.Global, opts)
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

	writeCapability := schemacapwrite.Build(compileResult.Schema)
	typeSpec, exists := writeCapability.EntityTypes[normalizedOpts.EntityType]
	if !exists {
		return buildErrorWithSchema(domainerrors.New(
			domainerrors.CodeEntityTypeUnknown,
			fmt.Sprintf("unknown entity type: %s", normalizedOpts.EntityType),
			map[string]any{"entity_type": normalizedOpts.EntityType},
		), schemaPayload), nil
	}

	snapshot, snapshotErr := workspace.BuildSnapshot(workspacePath, writeCapability.EntityTypes)
	if snapshotErr != nil {
		return buildErrorWithSchema(snapshotErr, schemaPayload), nil
	}

	payload, executeErr := engine.Execute(normalizedOpts, typeSpec, snapshot, h.now)
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
