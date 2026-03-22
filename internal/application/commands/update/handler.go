package update

import (
	"context"
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/engine"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/options"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/schema"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/workspace"
	"github.com/anatoly-tenenev/spec-cli/internal/application/workspacelock"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/responses"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

type Handler struct {
	now func() time.Time
}

func NewHandler(now func() time.Time) *Handler {
	if now == nil {
		now = time.Now
	}
	return &Handler{now: now}
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

	loadedSchema, schemaErr := schema.Load(schemaPath, request.Global.SchemaPath)
	if schemaErr != nil {
		return responses.CommandOutput{}, schemaErr
	}

	snapshot, snapshotErr := workspace.BuildSnapshot(workspacePath, normalizedOpts.ID)
	if snapshotErr != nil {
		return responses.CommandOutput{}, snapshotErr
	}

	payload, executeErr := engine.Execute(normalizedOpts, loadedSchema, snapshot, h.now)
	if executeErr != nil {
		return responses.CommandOutput{}, executeErr
	}

	return responses.CommandOutput{JSON: payload}, nil
}
