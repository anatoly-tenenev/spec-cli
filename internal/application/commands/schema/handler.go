package schema

import (
	"context"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/schema/internal/options"
	schemacompile "github.com/anatoly-tenenev/spec-cli/internal/application/schema/compile"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/responses"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
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

	compiler := h.newCompiler()
	compileResult := compiler.Compile(schemaPath, request.Global.SchemaPath)

	resultState := responses.ResultStateValid
	exitCode := 0
	if !compileResult.Valid {
		resultState = responses.ResultStateInvalid
		exitCode = 1
	}

	payload := map[string]any{
		"result_state":     resultState,
		"validation_scope": "schema",
		"schema": map[string]any{
			"valid":   compileResult.Valid,
			"summary": compileResult.Summary,
			"issues":  compileResult.Issues,
		},
	}

	return responses.CommandOutput{
		JSON:     payload,
		ExitCode: exitCode,
	}, nil
}
