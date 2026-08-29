// Package options parses the get command's arguments and resolves its paths.
// Which selectors are valid is not decided here - that needs the schema, and
// belongs to the selector plan.
//
// parse.go parses arguments; paths.go resolves workspace and schema paths.
package options

import (
	"fmt"
	"github.com/anatoly-tenenev/spec-cli/internal/cliflags"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/get/internal/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func Parse(args []string) (model.Options, *domainerrors.AppError) {
	opts := model.Options{Selectors: []string{}}

	for idx := 0; idx < len(args); idx++ {
		token := args[idx]
		name, inlineValue, hasInlineValue := cliflags.SplitLong(token)
		if !strings.HasPrefix(name, "--") {
			return model.Options{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unknown get option: %s", token),
				nil,
			)
		}

		switch name {
		case "--id":
			value, nextIdx, err := cliflags.Value(args, idx, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				return model.Options{}, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					"--id value cannot be empty",
					nil,
				)
			}
			opts.ID = trimmed
			idx = nextIdx
		case "--select":
			value, nextIdx, err := cliflags.Value(args, idx, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				return model.Options{}, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					"--select value cannot be empty",
					nil,
				)
			}
			opts.Selectors = append(opts.Selectors, trimmed)
			idx = nextIdx
		default:
			return model.Options{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unknown get option: %s", name),
				nil,
			)
		}
	}

	if opts.ID == "" {
		return model.Options{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"--id is required",
			nil,
		)
	}

	return opts, nil
}
