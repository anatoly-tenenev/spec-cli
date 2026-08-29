// Package query implements the query command: compile the schema, plan the
// query against it, load the workspace, execute. Everything past the schema
// compile is the shared read model, so query, get and graphql-query cannot
// disagree about what a selector or a filter means; what this package owns is
// the command's own surface - its flags and its response shape.
//
// handler.go is the command entrypoint; help.go declares how the command
// describes itself to help.
package query

import (
	"context"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/optionpaths"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/options"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/engine"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/workspace"
	schemacapread "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/read"
	schemacompile "github.com/anatoly-tenenev/spec-cli/internal/application/schema/compile"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/responses"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	outputpayload "github.com/anatoly-tenenev/spec-cli/internal/output/payload"
	"github.com/anatoly-tenenev/spec-cli/internal/output/querypayload"
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

	compiler := h.newCompiler()
	compileResult, compileErr := compiler.Compile(schemaPath, request.Global.SchemaPath)
	schemaPayload := outputpayload.BuildSchemaPayload(compileResult)

	if compileErr != nil {
		return outputpayload.BuildErrorOutput(compileErr, schemaPayload), nil
	}

	readCapability := schemacapread.Build(compileResult.Schema)

	plan, planErr := engine.BuildPlan(opts, readCapability)
	if planErr != nil {
		return outputpayload.BuildErrorOutput(planErr, schemaPayload), nil
	}

	entities, workspaceErr := workspace.LoadEntities(workspacePath, readCapability, opts.TypeFilters)
	if workspaceErr != nil {
		return outputpayload.BuildErrorOutput(workspaceErr, schemaPayload), nil
	}

	queryResult, executeErr := engine.Execute(plan, entities)
	if executeErr != nil {
		return outputpayload.BuildErrorOutput(executeErr, schemaPayload), nil
	}

	return responses.CommandOutput{
		JSON: querypayload.BuildSuccess(queryResult.ResultState, queryRootFields(queryResult.RootFields)),
	}, nil
}

func queryRootFields(roots []model.QueryRootField) []querypayload.RootField {
	payloadRoots := make([]querypayload.RootField, 0, len(roots))
	for _, root := range roots {
		payloadRoots = append(payloadRoots, querypayload.RootField{
			EntityType: root.EntityType,
			Items:      root.Items,
			TotalCount: root.TotalCount,
			PageInfo: querypayload.PageInfo{
				Mode:          root.PageInfo.Mode,
				Limit:         root.PageInfo.Limit,
				Offset:        root.PageInfo.Offset,
				Returned:      root.PageInfo.Returned,
				HasMore:       root.PageInfo.HasMore,
				NextOffset:    root.PageInfo.NextOffset,
				EffectiveSort: root.PageInfo.EffectiveSort,
			},
		})
	}
	return payloadRoots
}
