package get

import (
	"context"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/get/internal/engine"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/get/internal/options"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/get/internal/schema"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/get/internal/workspace"
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

	readModel, schemaErr := schema.LoadReadModel(schemaPath, request.Global.SchemaPath)
	if schemaErr != nil {
		return responses.CommandOutput{}, schemaErr
	}

	selectorPlan, selectorErr := engine.BuildSelectorPlan(opts.Selectors, readModel)
	if selectorErr != nil {
		return responses.CommandOutput{}, selectorErr
	}

	located, locateErr := workspace.LocateByID(workspacePath, opts.ID)
	if locateErr != nil {
		return responses.CommandOutput{}, locateErr
	}

	target, targetErr := workspace.ReadTarget(located.TargetPath, located.TargetRaw, opts.ID)
	if targetErr != nil {
		return responses.CommandOutput{}, targetErr
	}

	entityView, buildErr := engine.BuildEntityView(target, readModel, located.IdentityIndex, selectorPlan)
	if buildErr != nil {
		return responses.CommandOutput{}, buildErr
	}

	entityPayload := engine.ProjectEntity(entityView, selectorPlan.Tree, selectorPlan.NullIfMissingPaths)

	return responses.CommandOutput{
		JSON: map[string]any{
			"result_state": responses.ResultStateValid,
			"target": map[string]any{
				"match_by": "id",
				"id":       opts.ID,
			},
			"entity": entityPayload,
		},
	}, nil
}
