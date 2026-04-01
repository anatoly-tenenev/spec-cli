package query

import (
	"context"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/engine"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/options"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/schema"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/workspace"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/responses"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
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

	loadedSchema, schemaErr := schema.Load(schemaPath)
	if schemaErr != nil {
		return responses.CommandOutput{}, schemaErr
	}

	schemaIndex, schemaIndexErr := schema.BuildIndex(loadedSchema)
	if schemaIndexErr != nil {
		return responses.CommandOutput{}, schemaIndexErr
	}

	plan, planErr := engine.BuildPlan(opts, schemaIndex)
	if planErr != nil {
		return responses.CommandOutput{}, planErr
	}

	entities, workspaceErr := workspace.LoadEntities(workspacePath, schemaIndex, opts.TypeFilters)
	if workspaceErr != nil {
		return responses.CommandOutput{}, workspaceErr
	}

	queryResult, executeErr := engine.Execute(plan, entities)
	if executeErr != nil {
		return responses.CommandOutput{}, executeErr
	}

	return responses.CommandOutput{
		JSON: map[string]any{
			"result_state": queryResult.ResultState,
			"items":        queryResult.Items,
			"matched":      queryResult.Matched,
			"page": map[string]any{
				"mode":           queryResult.Page.Mode,
				"limit":          queryResult.Page.Limit,
				"offset":         queryResult.Page.Offset,
				"returned":       queryResult.Page.Returned,
				"has_more":       queryResult.Page.HasMore,
				"next_offset":    queryResult.Page.NextOffset,
				"effective_sort": queryResult.Page.EffectiveSort,
			},
		},
	}, nil
}
