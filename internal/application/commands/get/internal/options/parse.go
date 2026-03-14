package options

import (
	"fmt"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/get/internal/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func Parse(args []string) (model.Options, *domainerrors.AppError) {
	opts := model.Options{Selectors: []string{}}

	for idx := 0; idx < len(args); idx++ {
		token := args[idx]
		name, inlineValue, hasInlineValue := splitLongFlag(token)
		if !strings.HasPrefix(name, "--") {
			return model.Options{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unknown get option: %s", token),
				nil,
			)
		}

		switch name {
		case "--id":
			value, nextIdx, err := valueWithFallback(args, idx, hasInlineValue, inlineValue)
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
			value, nextIdx, err := valueWithFallback(args, idx, hasInlineValue, inlineValue)
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

func splitLongFlag(token string) (string, string, bool) {
	parts := strings.SplitN(token, "=", 2)
	if len(parts) == 1 {
		return parts[0], "", false
	}
	return parts[0], parts[1], true
}

func valueWithFallback(args []string, currentIdx int, hasInlineValue bool, inlineValue string) (string, int, *domainerrors.AppError) {
	if hasInlineValue {
		if inlineValue == "" {
			return "", currentIdx, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				"option value cannot be empty",
				nil,
			)
		}
		return inlineValue, currentIdx, nil
	}

	nextIdx := currentIdx + 1
	if nextIdx >= len(args) || strings.HasPrefix(args[nextIdx], "-") {
		return "", currentIdx, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"option value is required",
			nil,
		)
	}

	return args[nextIdx], nextIdx, nil
}
