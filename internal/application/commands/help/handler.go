// Package help implements the help command: it renders the CLI's own
// description, enriched with whatever the workspace schema makes available.
// help is text-only - an explicit --format json is refused rather than
// answered with a shape no one would parse.
//
// It is the one command that tolerates a broken schema: when the schema cannot
// be loaded, help still describes the CLI and states why the schema-derived
// parts are missing, because that is exactly the situation a caller runs help
// in.
//
// handler.go is the command entrypoint; help.go declares how the command
// describes itself to help.
package help

import (
	"context"
	"fmt"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/help/internal/options"
	"github.com/anatoly-tenenev/spec-cli/internal/application/help/helpglobal"
	"github.com/anatoly-tenenev/spec-cli/internal/application/help/helpmodel"
	"github.com/anatoly-tenenev/spec-cli/internal/application/help/helpschema"
	"github.com/anatoly-tenenev/spec-cli/internal/application/help/helptext"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/responses"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

type Handler struct {
	catalog *helpmodel.Catalog
}

func NewHandler(catalog *helpmodel.Catalog) *Handler {
	return &Handler{catalog: catalog}
}

func (h *Handler) Handle(_ context.Context, request requests.Command) (responses.CommandOutput, *domainerrors.AppError) {
	if request.Global.FormatExplicit && request.Global.Format == requests.FormatJSON {
		return responses.CommandOutput{}, domainerrors.New(
			domainerrors.CodeCapabilityUnsupported,
			"help supports text output only",
			nil,
		)
	}

	opts, parseErr := options.Parse(request.Args)
	if parseErr != nil {
		return responses.CommandOutput{}, parseErr
	}
	if opts.TargetCommand != "" {
		if _, exists := h.catalog.Find(opts.TargetCommand); !exists {
			return responses.CommandOutput{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unknown command: %s", opts.TargetCommand),
				map[string]any{"command": opts.TargetCommand},
			)
		}
	}

	paths, pathErr := options.NormalizePaths(request.Global)
	if pathErr != nil {
		return responses.CommandOutput{}, pathErr
	}

	schemaReport := helpschema.LoadReport(paths.SchemaPath, paths.SchemaResolvedPath)

	schemaView := helptext.SchemaView{
		WorkspacePath:  paths.WorkspaceDisplay,
		ResolvedPath:   schemaReport.ResolvedPath,
		Status:         string(schemaReport.Status),
		ShowProjection: opts.ShowSchemaProjection && opts.TargetCommand != "",
		Loaded:         schemaReport.Loaded,
		ReasonCode:     string(schemaReport.ReasonCode),
		Impact:         schemaReport.Impact,
		RecoveryClass:  string(schemaReport.RecoveryClass),
		RetryCommand:   schemaReport.RetryCommand,
	}

	if opts.TargetCommand == "" {
		text := helptext.RenderGeneral(
			"spec-cli is a schema-aware CLI for working with specification entities as JSON-like documents.",
			helpglobal.Options(),
			h.catalog.Ordered(),
			schemaView,
		)
		return responses.CommandOutput{Text: text}, nil
	}

	spec, _ := h.catalog.Find(opts.TargetCommand)
	text := helptext.RenderCommand(spec, schemaView)
	return responses.CommandOutput{Text: text}, nil
}
