package options

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/delete/internal/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func Parse(args []string) (model.Options, *domainerrors.AppError) {
	opts := model.Options{}

	for idx := 0; idx < len(args); idx++ {
		token := args[idx]
		if token == "--help" || token == "-h" {
			opts.Help = true
			continue
		}

		name, inlineValue, hasInlineValue := splitLongFlag(token)
		if !strings.HasPrefix(name, "--") {
			return model.Options{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unknown delete option: %s", token),
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
		case "--expect-revision":
			value, nextIdx, err := valueWithFallback(args, idx, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				return model.Options{}, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					"--expect-revision value cannot be empty",
					nil,
				)
			}
			opts.ExpectRevision = trimmed
			idx = nextIdx
		case "--dry-run":
			parsed, err := parseBoolFlag(name, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			opts.DryRun = parsed
		default:
			return model.Options{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unknown delete option: %s", name),
				nil,
			)
		}
	}

	if opts.Help {
		return opts, nil
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

func parseBoolFlag(name string, hasInlineValue bool, inlineValue string) (bool, *domainerrors.AppError) {
	if !hasInlineValue {
		return true, nil
	}

	parsed, err := strconv.ParseBool(inlineValue)
	if err != nil {
		return false, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			fmt.Sprintf("%s accepts boolean values only", name),
			map[string]any{"value": inlineValue},
		)
	}

	return parsed, nil
}
