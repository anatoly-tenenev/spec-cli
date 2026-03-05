package update

import (
	"context"

	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/responses"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) Handle(_ context.Context, _ requests.Command) (responses.CommandOutput, *domainerrors.AppError) {
	return responses.CommandOutput{}, domainerrors.New(
		domainerrors.CodeNotImplemented,
		"update command is scaffolded but not implemented yet",
		nil,
	)
}
