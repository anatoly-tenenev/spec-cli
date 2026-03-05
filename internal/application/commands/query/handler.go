package query

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

func (h *Handler) Handle(_ context.Context, request requests.Command) (responses.CommandOutput, *domainerrors.AppError) {
	for _, arg := range request.Args {
		if arg == "--help" || arg == "-h" {
			return responses.CommandOutput{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				"--help is not implemented yet",
				nil,
			)
		}
	}

	page := map[string]any{
		"mode":           "offset",
		"limit":          100,
		"offset":         0,
		"returned":       0,
		"has_more":       false,
		"next_offset":    nil,
		"effective_sort": []string{"type:asc", "id:asc"},
	}

	jsonResponse := map[string]any{
		"result_state": responses.ResultStateValid,
		"items":        []any{},
		"matched":      0,
		"page":         page,
	}

	ndjsonResponse := []map[string]any{
		{
			"record_type":  "summary",
			"result_state": responses.ResultStateValid,
			"matched":      0,
			"page":         page,
		},
	}

	return responses.CommandOutput{
		JSON:   jsonResponse,
		NDJSON: ndjsonResponse,
	}, nil
}
