// Package update implements the update command: change an existing entity in
// place. It differs from add in what it must preserve - everything the caller
// did not touch, including hand edits - and in offering optimistic
// concurrency through --expect-revision, so a caller can refuse to overwrite a
// document that moved since it was read.
//
// The workspace lock is taken before the target is read, so the revision the
// decision is made against is the one the write applies to.
//
// handler.go is the command entrypoint; help.go declares how the command
// describes itself to help.
package update

import (
	"context"
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/schemaload"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/engine"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/options"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/workspace"
	schemacapwrite "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/write"
	"github.com/anatoly-tenenev/spec-cli/internal/application/workspacelock"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/responses"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	outputpayload "github.com/anatoly-tenenev/spec-cli/internal/output/payload"
)

type Handler struct {
	now    func() time.Time
	schema schemaload.Loader
}

func NewHandler(now func() time.Time) *Handler {
	if now == nil {
		now = time.Now
	}
	return &Handler{now: now, schema: schemaload.NewLoader()}
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

	compileResult, schemaPayload, compileErr := h.schema.Compile(schemaPath, request.Global.SchemaPath)

	if compileErr != nil {
		return outputpayload.BuildErrorOutput(compileErr, schemaPayload), nil
	}
	writeCapability := schemacapwrite.Build(compileResult.Schema)

	snapshot, snapshotErr := workspace.BuildSnapshot(workspacePath, normalizedOpts.ID)
	if snapshotErr != nil {
		return outputpayload.BuildErrorOutput(snapshotErr, schemaPayload), nil
	}

	payload, executeErr := engine.Execute(normalizedOpts, writeCapability, snapshot, h.now)
	if executeErr != nil {
		return outputpayload.BuildErrorOutput(executeErr, schemaPayload), nil
	}

	return responses.CommandOutput{JSON: payload}, nil
}
