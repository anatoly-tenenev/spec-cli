package add

import (
	"context"
	"fmt"
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/engine"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/options"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/schema"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/workspace"
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

	typeSpec, exists := loadedSchema.EntityTypes[normalizedOpts.EntityType]
	if !exists {
		return responses.CommandOutput{}, domainerrors.New(
			domainerrors.CodeEntityTypeUnknown,
			fmt.Sprintf("unknown entity type: %s", normalizedOpts.EntityType),
			map[string]any{"entity_type": normalizedOpts.EntityType},
		)
	}

	snapshot, snapshotErr := workspace.BuildSnapshot(workspacePath, loadedSchema)
	if snapshotErr != nil {
		return responses.CommandOutput{}, snapshotErr
	}

	payload, executeErr := engine.Execute(normalizedOpts, typeSpec, snapshot, h.now)
	if executeErr != nil {
		return responses.CommandOutput{}, executeErr
	}

	return responses.CommandOutput{JSON: payload}, nil
}
