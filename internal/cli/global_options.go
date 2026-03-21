package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

const autoConfigFilename = "spec-cli.json"

type loadedGlobalConfig struct {
	SchemaValue     string
	SchemaSet       bool
	WorkspaceValue  string
	WorkspaceSet    bool
	ConfigPath      string
	ConfigDirectory string
}

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
	explicitConfig := false

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
				explicitConfig = explicitConfig || name == "--config"
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
				explicitConfig = explicitConfig || name == "--config"
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

	configErr := applyConfigOptions(&opts, explicitWorkspace, explicitSchema, explicitConfig)
	if configErr != nil {
		return opts, "", nil, configErr
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

func applyConfigOptions(
	opts *requests.GlobalOptions,
	explicitWorkspace bool,
	explicitSchema bool,
	explicitConfig bool,
) *domainerrors.AppError {
	loaded, hasConfig, configErr := loadActiveConfig(opts.ConfigPath, explicitConfig)
	if configErr != nil {
		return configErr
	}
	if !hasConfig {
		return nil
	}

	opts.ConfigPath = loaded.ConfigPath

	if loaded.WorkspaceSet && !explicitWorkspace {
		opts.Workspace = resolveConfigPath(loaded.ConfigDirectory, loaded.WorkspaceValue)
	}
	if loaded.SchemaSet && !explicitSchema {
		opts.SchemaPath = resolveConfigPath(loaded.ConfigDirectory, loaded.SchemaValue)
	}

	return nil
}

func loadActiveConfig(rawPath string, explicitConfig bool) (loadedGlobalConfig, bool, *domainerrors.AppError) {
	configPath, err := activeConfigPath(rawPath, explicitConfig)
	if err != nil {
		return loadedGlobalConfig{}, false, invalidConfigError("", err.Error(), nil)
	}

	content, readErr := os.ReadFile(configPath)
	if readErr != nil {
		if !explicitConfig && errors.Is(readErr, os.ErrNotExist) {
			return loadedGlobalConfig{}, false, nil
		}
		return loadedGlobalConfig{}, false, invalidConfigError(configPath, "failed to read config file", readErr)
	}

	parsed, parseErr := parseConfig(content, configPath)
	if parseErr != nil {
		return loadedGlobalConfig{}, false, parseErr
	}

	return parsed, true, nil
}

func activeConfigPath(rawPath string, explicitConfig bool) (string, error) {
	if explicitConfig {
		return filepath.Abs(rawPath)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to resolve current working directory: %w", err)
	}
	return filepath.Join(cwd, autoConfigFilename), nil
}

func parseConfig(content []byte, configPath string) (loadedGlobalConfig, *domainerrors.AppError) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(content, &raw); err != nil {
		return loadedGlobalConfig{}, invalidConfigError(configPath, "failed to parse JSON config", err)
	}

	parsed := loadedGlobalConfig{
		ConfigPath:      filepath.Clean(configPath),
		ConfigDirectory: filepath.Dir(configPath),
	}

	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		rawValue := raw[key]
		switch key {
		case "schema":
			value, parseErr := parseConfigPathValue(rawValue, key, configPath)
			if parseErr != nil {
				return loadedGlobalConfig{}, parseErr
			}
			parsed.SchemaValue = value
			parsed.SchemaSet = true
		case "workspace":
			value, parseErr := parseConfigPathValue(rawValue, key, configPath)
			if parseErr != nil {
				return loadedGlobalConfig{}, parseErr
			}
			parsed.WorkspaceValue = value
			parsed.WorkspaceSet = true
		default:
			return loadedGlobalConfig{}, domainerrors.New(
				domainerrors.CodeInvalidConfig,
				fmt.Sprintf("unknown config key: %s", key),
				map[string]any{
					"path": configPath,
					"key":  key,
				},
			)
		}
	}

	return parsed, nil
}

func parseConfigPathValue(rawValue json.RawMessage, key string, configPath string) (string, *domainerrors.AppError) {
	var value string
	if err := json.Unmarshal(rawValue, &value); err != nil {
		return "", invalidConfigError(configPath, fmt.Sprintf("config key %q must be a string", key), err)
	}
	if strings.TrimSpace(value) == "" {
		return "", domainerrors.New(
			domainerrors.CodeInvalidConfig,
			fmt.Sprintf("config key %q must be a non-empty string", key),
			map[string]any{
				"path": configPath,
				"key":  key,
			},
		)
	}
	return value, nil
}

func resolveConfigPath(configDir string, rawValue string) string {
	if filepath.IsAbs(rawValue) {
		return filepath.Clean(rawValue)
	}
	return filepath.Clean(filepath.Join(configDir, rawValue))
}

func invalidConfigError(configPath string, message string, cause error) *domainerrors.AppError {
	details := map[string]any{}
	if configPath != "" {
		details["path"] = filepath.Clean(configPath)
	}
	if cause != nil {
		details["reason"] = cause.Error()
	}
	if len(details) == 0 {
		details = nil
	}

	return domainerrors.New(
		domainerrors.CodeInvalidConfig,
		message,
		details,
	)
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
