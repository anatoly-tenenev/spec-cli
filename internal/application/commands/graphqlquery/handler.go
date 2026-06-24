package graphqlquery

import (
	"context"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/graphqlquery/internal/options"
	"github.com/anatoly-tenenev/spec-cli/internal/application/graphql/binding"
	"github.com/anatoly-tenenev/spec-cli/internal/application/graphql/document"
	"github.com/anatoly-tenenev/spec-cli/internal/application/graphql/projection"
	readworkspace "github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/workspace"
	readcap "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/read"
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
	paths, pathErr := options.NormalizePaths(request.Global, opts)
	if pathErr != nil {
		return responses.CommandOutput{}, pathErr
	}
	loaded, loadErr := document.Load(document.Request{
		Query:         opts.Query,
		File:          paths.QueryFile,
		VariablesJSON: opts.VariablesJSON,
		VariablesFile: paths.VariablesFile,
	})
	if loadErr != nil {
		return buildError(loadErr, nil), nil
	}
	compileResult, compileErr := h.newCompiler().Compile(paths.SchemaPath, request.Global.SchemaPath)
	schemaPayload := outputpayload.BuildSchemaPayload(compileResult)
	if compileErr != nil {
		return buildError(compileErr, schemaPayload), nil
	}
	readCapability := readcap.Build(compileResult.Schema)
	proj, projectionErr := projection.Build(compileResult.Schema, readCapability)
	if projectionErr != nil {
		return buildError(projectionErr, nil), nil
	}
	rootPlans, bindErr := binding.Build(proj, loaded.Query, loaded.Variables, opts.OperationName)
	if bindErr != nil {
		return buildError(bindErr, nil), nil
	}
	entities, workspaceErr := readworkspace.LoadEntities(paths.WorkspacePath, readCapability, nil)
	if workspaceErr != nil {
		return buildError(workspaceErr, nil), nil
	}
	data, executeErr := binding.Execute(rootPlans, entities)
	if executeErr != nil {
		return buildError(executeErr, nil), nil
	}
	return responses.CommandOutput{
		JSON: map[string]any{
			"result_state": "valid",
			"data":         data,
		},
	}, nil
}

func buildError(appErr *domainerrors.AppError, schemaPayload map[string]any) responses.CommandOutput {
	payload := map[string]any{
		"result_state": errormap.ResultStateForCode(appErr.Code),
		"error":        outputpayload.BuildErrorPayload(appErr),
	}
	if schemaPayload != nil && outputpayload.ShouldIncludeSchemaForError(appErr.Code) {
		payload["schema"] = schemaPayload
	}
	return responses.CommandOutput{JSON: payload, ExitCode: appErr.ExitCode}
}
