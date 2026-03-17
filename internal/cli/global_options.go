package cli

import (
	"fmt"
	"path/filepath"
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
			map[string]any{"commands": supportedCommandNames()},
		)
	}

	commandName := ""
	commandArgs := make([]string, 0, len(args))
	commandSelected := false
	seenGlobals := map[string]struct{}{}
	explicitWorkspace := false
	explicitSchema := false

	for i := 0; i < len(args); i++ {
		token := args[i]

		if token == "--" {
			if commandSelected {
				commandArgs = append(commandArgs, args[i:]...)
				break
			}
			return opts, "", nil, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				"command is required before '--'",
				nil,
			)
		}

		if !strings.HasPrefix(token, "-") {
			if !commandSelected {
				if !isSupportedCommand(token) {
					return opts, "", nil, domainerrors.New(
						domainerrors.CodeInvalidArgs,
						fmt.Sprintf("unknown command: %s", token),
						map[string]any{"command": token},
					)
				}
				commandName = token
				commandSelected = true
				continue
			}
			commandArgs = append(commandArgs, token)
			continue
		}

		name, value, hasValue := splitLongFlag(token)
		if !commandSelected {
			if !strings.HasPrefix(name, "--") {
				return opts, "", nil, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					fmt.Sprintf("unsupported option format: %s", token),
					nil,
				)
			}

			handled, nextIdx, usedWorkspace, usedSchema, err := consumeGlobalOption(args, i, name, value, hasValue, &opts, seenGlobals)
			if err != nil {
				return opts, "", nil, err
			}
			if handled {
				explicitWorkspace = explicitWorkspace || usedWorkspace
				explicitSchema = explicitSchema || usedSchema
				i = nextIdx
				continue
			}

			return opts, "", nil, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unknown global option: %s", name),
				nil,
			)
		}

		if strings.HasPrefix(name, "--") {
			handled, nextIdx, usedWorkspace, usedSchema, err := consumeGlobalOption(args, i, name, value, hasValue, &opts, seenGlobals)
			if err != nil {
				return opts, "", nil, err
			}
			if handled {
				explicitWorkspace = explicitWorkspace || usedWorkspace
				explicitSchema = explicitSchema || usedSchema
				i = nextIdx
				continue
			}
		}

		commandArgs = append(commandArgs, token)
	}

	if !commandSelected {
		return opts, "", nil, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"command is required",
			map[string]any{"commands": supportedCommandNames()},
		)
	}

	if opts.RequireAbsolutePaths {
		if explicitWorkspace && !filepath.IsAbs(opts.Workspace) {
			return opts, "", nil, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				"--workspace must be absolute when --require-absolute-paths is enabled",
				nil,
			)
		}
		if explicitSchema && !filepath.IsAbs(opts.SchemaPath) {
			return opts, "", nil, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				"--schema must be absolute when --require-absolute-paths is enabled",
				nil,
			)
		}
	}

	return opts, commandName, commandArgs, nil
}

func consumeGlobalOption(
	args []string,
	currentIdx int,
	name string,
	value string,
	hasValue bool,
	opts *requests.GlobalOptions,
	seen map[string]struct{},
) (bool, int, bool, bool, *domainerrors.AppError) {
	switch name {
	case "--workspace":
		if err := markGlobalOptionSeen(name, seen); err != nil {
			return true, currentIdx, false, false, err
		}
		v, next, err := valueWithFallback(args, currentIdx, hasValue, value)
		if err != nil {
			return true, currentIdx, false, false, err
		}
		opts.Workspace = v
		return true, next, true, false, nil
	case "--schema":
		if err := markGlobalOptionSeen(name, seen); err != nil {
			return true, currentIdx, false, false, err
		}
		v, next, err := valueWithFallback(args, currentIdx, hasValue, value)
		if err != nil {
			return true, currentIdx, false, false, err
		}
		opts.SchemaPath = v
		return true, next, false, true, nil
	case "--format":
		if err := markGlobalOptionSeen(name, seen); err != nil {
			return true, currentIdx, false, false, err
		}
		v, next, err := valueWithFallback(args, currentIdx, hasValue, value)
		if err != nil {
			return true, currentIdx, false, false, err
		}
		switch requests.OutputFormat(v) {
		case requests.FormatJSON, requests.FormatText:
			opts.Format = requests.OutputFormat(v)
			opts.FormatExplicit = true
		default:
			return true, currentIdx, false, false, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				"--format must be one of: json, text",
				map[string]any{"value": v},
			)
		}
		return true, next, false, false, nil
	case "--config":
		if err := markGlobalOptionSeen(name, seen); err != nil {
			return true, currentIdx, false, false, err
		}
		v, next, err := valueWithFallback(args, currentIdx, hasValue, value)
		if err != nil {
			return true, currentIdx, false, false, err
		}
		opts.ConfigPath = v
		return true, next, false, false, nil
	case "--require-absolute-paths":
		if err := markGlobalOptionSeen(name, seen); err != nil {
			return true, currentIdx, false, false, err
		}
		parsed, err := parseBoolFlag(name, hasValue, value)
		if err != nil {
			return true, currentIdx, false, false, err
		}
		opts.RequireAbsolutePaths = parsed
		return true, currentIdx, false, false, nil
	case "--verbose":
		if err := markGlobalOptionSeen(name, seen); err != nil {
			return true, currentIdx, false, false, err
		}
		parsed, err := parseBoolFlag(name, hasValue, value)
		if err != nil {
			return true, currentIdx, false, false, err
		}
		opts.Verbose = parsed
		return true, currentIdx, false, false, nil
	default:
		return false, currentIdx, false, false, nil
	}
}

func markGlobalOptionSeen(name string, seen map[string]struct{}) *domainerrors.AppError {
	if _, exists := seen[name]; exists {
		return domainerrors.New(
			domainerrors.CodeInvalidArgs,
			fmt.Sprintf("duplicate global option: %s", name),
			nil,
		)
	}
	seen[name] = struct{}{}
	return nil
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
