package options

import (
	"fmt"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func Parse(args []string) (model.Options, *domainerrors.AppError) {
	opts := model.Options{Operations: []model.WriteOperation{}}
	seenPaths := map[string]struct{}{}

	for idx := 0; idx < len(args); idx++ {
		token := args[idx]
		name, inlineValue, hasInlineValue := splitLongFlag(token)
		if !strings.HasPrefix(name, "--") {
			return model.Options{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unknown add option: %s", token),
				nil,
			)
		}

		switch name {
		case "--type":
			value, nextIdx, err := valueWithFallback(args, idx, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			value = strings.TrimSpace(value)
			if value == "" {
				return model.Options{}, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					"--type value cannot be empty",
					nil,
				)
			}
			opts.EntityType = value
			idx = nextIdx
		case "--slug":
			value, nextIdx, err := valueWithFallback(args, idx, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			value = strings.TrimSpace(value)
			if value == "" {
				return model.Options{}, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					"--slug value cannot be empty",
					nil,
				)
			}
			opts.Slug = value
			idx = nextIdx
		case "--set":
			value, nextIdx, err := valueWithFallbackAllowDash(args, idx, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			op, parseErr := parsePathValue(value)
			if parseErr != nil {
				return model.Options{}, parseErr
			}
			if _, exists := seenPaths[op.Path]; exists {
				return model.Options{}, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					fmt.Sprintf("duplicate write path: %s", op.Path),
					map[string]any{"path": op.Path},
				)
			}
			seenPaths[op.Path] = struct{}{}
			opts.Operations = append(opts.Operations, model.WriteOperation{
				Kind:     model.WriteOperationSet,
				Path:     op.Path,
				RawValue: op.Value,
			})
			idx = nextIdx
		case "--set-file":
			value, nextIdx, err := valueWithFallbackAllowDash(args, idx, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			op, parseErr := parsePathValue(value)
			if parseErr != nil {
				return model.Options{}, parseErr
			}
			if strings.TrimSpace(op.Value) == "" {
				return model.Options{}, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					"--set-file requires non-empty file path",
					nil,
				)
			}
			if _, exists := seenPaths[op.Path]; exists {
				return model.Options{}, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					fmt.Sprintf("duplicate write path: %s", op.Path),
					map[string]any{"path": op.Path},
				)
			}
			seenPaths[op.Path] = struct{}{}
			opts.Operations = append(opts.Operations, model.WriteOperation{
				Kind:     model.WriteOperationSetFile,
				Path:     op.Path,
				RawValue: op.Value,
			})
			idx = nextIdx
		case "--content-file":
			value, nextIdx, err := valueWithFallback(args, idx, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				return model.Options{}, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					"--content-file value cannot be empty",
					nil,
				)
			}
			opts.ContentFile = trimmed
			idx = nextIdx
		case "--content-stdin":
			parsed, err := parseBoolFlag(name, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			opts.ContentStdin = parsed
		case "--dry-run":
			parsed, err := parseBoolFlag(name, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			opts.DryRun = parsed
		default:
			return model.Options{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unknown add option: %s", name),
				nil,
			)
		}
	}

	if opts.EntityType == "" {
		return model.Options{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"--type is required",
			nil,
		)
	}
	if opts.Slug == "" {
		return model.Options{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"--slug is required",
			nil,
		)
	}

	if opts.ContentFile != "" && opts.ContentStdin {
		return model.Options{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"--content-file and --content-stdin are mutually exclusive",
			nil,
		)
	}

	if (opts.ContentFile != "" || opts.ContentStdin) && hasSectionWrite(opts.Operations) {
		return model.Options{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"whole-body input cannot be combined with content.sections.* write-paths",
			nil,
		)
	}

	return opts, nil
}

type pathValue struct {
	Path  string
	Value string
}

func parsePathValue(raw string) (pathValue, *domainerrors.AppError) {
	eqIdx := strings.Index(raw, "=")
	if eqIdx <= 0 {
		return pathValue{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"write operation must match <path=value>",
			map[string]any{"value": raw},
		)
	}

	path := strings.TrimSpace(raw[:eqIdx])
	value := raw[eqIdx+1:]
	if path == "" {
		return pathValue{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"write path cannot be empty",
			nil,
		)
	}

	return pathValue{Path: path, Value: value}, nil
}

func hasSectionWrite(operations []model.WriteOperation) bool {
	for _, op := range operations {
		if strings.HasPrefix(op.Path, "content.sections.") {
			return true
		}
	}
	return false
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

func valueWithFallbackAllowDash(args []string, currentIdx int, hasInlineValue bool, inlineValue string) (string, int, *domainerrors.AppError) {
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
	if nextIdx >= len(args) {
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

	switch strings.ToLower(strings.TrimSpace(inlineValue)) {
	case "true", "1", "yes", "y", "on":
		return true, nil
	case "false", "0", "no", "n", "off":
		return false, nil
	default:
		return false, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			fmt.Sprintf("%s accepts boolean values only", name),
			map[string]any{"value": inlineValue},
		)
	}
}
