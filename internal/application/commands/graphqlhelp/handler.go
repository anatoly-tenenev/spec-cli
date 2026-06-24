package graphqlhelp

import (
	"context"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/graphqlhelp/internal/options"
	"github.com/anatoly-tenenev/spec-cli/internal/application/graphql/catalog"
	"github.com/anatoly-tenenev/spec-cli/internal/application/graphql/projection"
	"github.com/anatoly-tenenev/spec-cli/internal/application/graphql/sdl"
	readcap "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/read"
	schemacompile "github.com/anatoly-tenenev/spec-cli/internal/application/schema/compile"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/responses"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	"github.com/anatoly-tenenev/spec-cli/internal/output/errormap"
	outputpayload "github.com/anatoly-tenenev/spec-cli/internal/output/payload"
)

type Handler struct {
	newCompiler func() *schemacompile.Compiler
}

func NewHandler() *Handler {
	return &Handler{newCompiler: schemacompile.NewCompiler}
}

func (h *Handler) Handle(_ context.Context, request requests.Command) (responses.CommandOutput, *domainerrors.AppError) {
	if request.Global.FormatExplicit && request.Global.Format == requests.FormatJSON {
		return responses.CommandOutput{}, domainerrors.New(domainerrors.CodeCapabilityUnsupported, "graphql-help supports text output only", nil)
	}
	opts, parseErr := options.Parse(request.Args)
	if parseErr != nil {
		return responses.CommandOutput{}, parseErr
	}
	_, schemaPath, pathErr := options.NormalizePaths(request.Global)
	if pathErr != nil {
		return responses.CommandOutput{}, pathErr
	}
	compileResult, compileErr := h.newCompiler().Compile(schemaPath, request.Global.SchemaPath)
	if compileErr != nil {
		return buildError(compileErr, outputpayload.BuildSchemaPayload(compileResult)), nil
	}
	readCapability := readcap.Build(compileResult.Schema)
	proj, projectionErr := projection.Build(compileResult.Schema, readCapability)
	if projectionErr != nil {
		return buildError(projectionErr, nil), nil
	}
	for _, entity := range opts.Entities {
		if _, exists := proj.Entities[entity]; !exists {
			return responses.CommandOutput{}, domainerrors.New(domainerrors.CodeInvalidArgs, "unknown GraphQL entity", map[string]any{"entity_type": entity})
		}
	}
	if opts.SchemaOnly {
		return responses.CommandOutput{Text: sdl.Render(proj, opts.Entities)}, nil
	}
	return responses.CommandOutput{Text: catalog.Render(proj)}, nil
}

func buildError(appErr *domainerrors.AppError, schemaPayload map[string]any) responses.CommandOutput {
	payload := map[string]any{
		"result_state": errormap.ResultStateForCode(appErr.Code),
		"error":        outputpayload.BuildErrorPayload(appErr),
	}
	if schemaPayload != nil && outputpayload.ShouldIncludeSchemaForError(appErr.Code) {
		payload["schema"] = schemaPayload
	}
	return responses.CommandOutput{JSON: payload, ExitCode: appErr.ExitCode}
}
