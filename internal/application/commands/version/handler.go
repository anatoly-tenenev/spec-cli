package version

import (
	"context"
	"fmt"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/version/internal/options"
	"github.com/anatoly-tenenev/spec-cli/internal/buildinfo"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/responses"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

type Handler struct {
	resolveVersion func() (string, error)
}

func NewHandler() *Handler {
	return newHandler(buildinfo.ResolveVersion)
}

func newHandler(resolveVersion func() (string, error)) *Handler {
	if resolveVersion == nil {
		resolveVersion = buildinfo.ResolveVersion
	}
	return &Handler{resolveVersion: resolveVersion}
}

func (h *Handler) Handle(_ context.Context, request requests.Command) (responses.CommandOutput, *domainerrors.AppError) {
	_, parseErr := options.Parse(request.Args)
	if parseErr != nil {
		return responses.CommandOutput{}, parseErr
	}

	versionValue, resolveErr := h.resolveVersion()
	if resolveErr != nil {
		return responses.CommandOutput{}, domainerrors.New(
			domainerrors.CodeInternalError,
			fmt.Sprintf("failed to resolve build version: %v", resolveErr),
			nil,
		)
	}

	return responses.CommandOutput{
		JSON: map[string]any{
			"result_state": responses.ResultStateValid,
			"version":      versionValue,
		},
	}, nil
}
