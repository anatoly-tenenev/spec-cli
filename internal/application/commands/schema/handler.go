// Package schema implements the schema command, whose only subcommand today is
// check: compile the schema and return its diagnostics. It deliberately does
// not scan the workspace, so a caller can ask whether the schema itself is
// sound separately from whether documents conform to it.
//
// handler.go is the command entrypoint; help.go declares how the command
// describes itself to help.
package schema

import (
	"context"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/schemaload"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/schema/internal/options"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/responses"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	"github.com/anatoly-tenenev/spec-cli/internal/output/errormap"
	outputpayload "github.com/anatoly-tenenev/spec-cli/internal/output/payload"
)

type Handler struct {
	schema schemaload.Loader
}

func NewHandler() *Handler {
	return &Handler{schema: schemaload.NewLoader()}
}

func (h *Handler) Handle(_ context.Context, request requests.Command) (responses.CommandOutput, *domainerrors.AppError) {
	opts, parseErr := options.Parse(request.Args)
	if parseErr != nil {
		return responses.CommandOutput{}, parseErr
	}

	schemaPath, pathErr := options.NormalizeSchemaPath(request.Global)
	if pathErr != nil {
		return responses.CommandOutput{}, pathErr
	}

	if opts.Subcommand != "check" {
		return responses.CommandOutput{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"unsupported schema subcommand",
			map[string]any{"subcommand": opts.Subcommand},
		)
	}

	_, schemaPayload, compileErr := h.schema.Compile(schemaPath, request.Global.SchemaPath)

	if compileErr != nil {
		return responses.CommandOutput{
			JSON: map[string]any{
				"result_state":     errormap.ResultStateForCode(compileErr.Code),
				"validation_scope": "schema",
				"schema":           schemaPayload,
				"error":            outputpayload.BuildErrorPayload(compileErr),
			},
			ExitCode: compileErr.ExitCode,
		}, nil
	}

	payload := map[string]any{
		"result_state":     responses.ResultStateValid,
		"validation_scope": "schema",
		"schema":           schemaPayload,
	}

	return responses.CommandOutput{
		JSON:     payload,
		ExitCode: 0,
	}, nil
}
