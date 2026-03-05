package commandbus

import (
	"context"
	"fmt"

	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/responses"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

type Handler interface {
	Handle(ctx context.Context, request requests.Command) (responses.CommandOutput, *domainerrors.AppError)
}

type Bus struct {
	handlers map[string]Handler
}

func New() *Bus {
	return &Bus{handlers: make(map[string]Handler)}
}

func (b *Bus) Register(command string, handler Handler) {
	b.handlers[command] = handler
}

func (b *Bus) Dispatch(ctx context.Context, request requests.Command) (responses.CommandOutput, *domainerrors.AppError) {
	handler, ok := b.handlers[request.Name]
	if !ok {
		return responses.CommandOutput{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			fmt.Sprintf("unknown command: %s", request.Name),
			map[string]any{"command": request.Name},
		)
	}

	return handler.Handle(ctx, request)
}
