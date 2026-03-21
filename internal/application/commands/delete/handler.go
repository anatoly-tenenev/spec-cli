package delete

import (
	"context"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/delete/internal/engine"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/delete/internal/options"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/delete/internal/schema"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/delete/internal/workspace"
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

	loadedSchema, schemaErr := schema.Load(schemaPath, request.Global.SchemaPath)
	if schemaErr != nil {
		return responses.CommandOutput{}, schemaErr
	}

	snapshot, snapshotErr := workspace.BuildSnapshot(workspacePath, opts.ID)
	if snapshotErr != nil {
		return responses.CommandOutput{}, snapshotErr
	}

	payload, executeErr := engine.Execute(opts, loadedSchema, snapshot)
	if executeErr != nil {
		return responses.CommandOutput{}, executeErr
	}

	return responses.CommandOutput{JSON: payload}, nil
}
