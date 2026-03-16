package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func defaultGlobalOptions() requests.GlobalOptions {
	return requests.GlobalOptions{
		Workspace:  ".",
		SchemaPath: "spec.schema.yaml",
		Format:     requests.FormatJSON,
	}
}

func parseGlobalOptions(args []string) (requests.GlobalOptions, string, []string, *domainerrors.AppError) {
	opts := defaultGlobalOptions()
	if len(args) == 0 {
		return opts, "", nil, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"command is required",
			map[string]any{"commands": []string{"validate", "query", "get", "add", "update", "delete", "version"}},
		)
	}

	for i := 0; i < len(args); i++ {
		token := args[i]

		if token == "--" {
			return opts, "", nil, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				"command is required before '--'",
				nil,
			)
		}

		if !strings.HasPrefix(token, "-") {
			if !isSupportedCommand(token) {
				return opts, "", nil, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					fmt.Sprintf("unknown command: %s", token),
					map[string]any{"command": token},
				)
			}
			return opts, token, args[i+1:], nil
		}

		name, value, hasValue := splitLongFlag(token)
		if !strings.HasPrefix(name, "--") {
			return opts, "", nil, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unsupported option format: %s", token),
				nil,
			)
		}

		switch name {
		case "--workspace":
			v, next, err := valueWithFallback(args, i, hasValue, value)
			if err != nil {
				return opts, "", nil, err
			}
			opts.Workspace = v
			i = next
		case "--schema":
			v, next, err := valueWithFallback(args, i, hasValue, value)
			if err != nil {
				return opts, "", nil, err
			}
			opts.SchemaPath = v
			i = next
		case "--format":
			v, next, err := valueWithFallback(args, i, hasValue, value)
			if err != nil {
				return opts, "", nil, err
			}
			switch requests.OutputFormat(v) {
			case requests.FormatJSON:
				opts.Format = requests.OutputFormat(v)
			default:
				return opts, "", nil, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					"--format must be one of: json",
					map[string]any{"value": v},
				)
			}
			i = next
		case "--config":
			v, next, err := valueWithFallback(args, i, hasValue, value)
			if err != nil {
				return opts, "", nil, err
			}
			opts.ConfigPath = v
			i = next
		case "--require-absolute-paths":
			parsed, err := parseBoolFlag(name, hasValue, value)
			if err != nil {
				return opts, "", nil, err
			}
			opts.RequireAbsolutePaths = parsed
		case "--verbose":
			parsed, err := parseBoolFlag(name, hasValue, value)
			if err != nil {
				return opts, "", nil, err
			}
			opts.Verbose = parsed
		default:
			return opts, "", nil, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unknown global option: %s", name),
				nil,
			)
		}
	}

	return opts, "", nil, domainerrors.New(
		domainerrors.CodeInvalidArgs,
		"command is required",
		map[string]any{"commands": []string{"validate", "query", "get", "add", "update", "delete", "version"}},
	)
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
