package get

import (
	"context"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/get/internal/engine"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/get/internal/options"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/get/internal/workspace"
	schemacapread "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/read"
	schemacompile "github.com/anatoly-tenenev/spec-cli/internal/application/schema/compile"
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

	compiler := h.newCompiler()
	compileResult, compileErr := compiler.Compile(schemaPath, request.Global.SchemaPath)
	schemaPayload := outputpayload.BuildSchemaPayload(compileResult)

	if compileErr != nil {
		return buildErrorWithSchema(compileErr, schemaPayload), nil
	}

	readCapability := schemacapread.Build(compileResult.Schema)

	selectorPlan, selectorErr := engine.BuildSelectorPlan(opts.Selectors, readCapability)
	if selectorErr != nil {
		return buildErrorWithSchema(selectorErr, schemaPayload), nil
	}

	located, locateErr := workspace.LocateByID(workspacePath, opts.ID)
	if locateErr != nil {
		return buildErrorWithSchema(locateErr, schemaPayload), nil
	}

	target, targetErr := workspace.ReadTarget(located.TargetPath, located.TargetRaw, opts.ID)
	if targetErr != nil {
		return buildErrorWithSchema(targetErr, schemaPayload), nil
	}

	entityView, buildErr := engine.BuildEntityView(target, readCapability, located.IdentityIndex, selectorPlan)
	if buildErr != nil {
		return buildErrorWithSchema(buildErr, schemaPayload), nil
	}

	entityPayload := engine.ProjectEntity(entityView, selectorPlan.Tree, selectorPlan.NullIfMissingPaths)

	return responses.CommandOutput{
		JSON: map[string]any{
			"result_state": responses.ResultStateValid,
			"schema":       schemaPayload,
			"target": map[string]any{
				"match_by": "id",
				"id":       opts.ID,
			},
			"entity": entityPayload,
		},
	}, nil
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
