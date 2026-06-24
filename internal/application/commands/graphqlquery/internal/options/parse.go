package options

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

type Options struct {
	Query         string
	File          string
	VariablesJSON string
	VariablesFile string
	OperationName string
}

type Paths struct {
	WorkspacePath string
	SchemaPath    string
	QueryFile     string
	VariablesFile string
}

func Parse(args []string) (Options, *domainerrors.AppError) {
	opts := Options{}
	seen := map[string]struct{}{}
	for idx := 0; idx < len(args); idx++ {
		token := args[idx]
		name, inline, hasInline := splitLongFlag(token)
		if !strings.HasPrefix(name, "--") {
			return Options{}, domainerrors.New(domainerrors.CodeInvalidArgs, fmt.Sprintf("unknown graphql-query option: %s", token), nil)
		}
		switch name {
		case "--query", "--file", "--variables-json", "--variables-file", "--operation-name":
			if _, exists := seen[name]; exists {
				return Options{}, domainerrors.New(domainerrors.CodeInvalidArgs, fmt.Sprintf("duplicate graphql-query option: %s", name), nil)
			}
			seen[name] = struct{}{}
			value, next, err := valueWithFallback(args, idx, hasInline, inline)
			if err != nil {
				return Options{}, err
			}
			if strings.TrimSpace(value) == "" {
				return Options{}, domainerrors.New(domainerrors.CodeInvalidArgs, name+" value cannot be empty", nil)
			}
			switch name {
			case "--query":
				opts.Query = value
			case "--file":
				opts.File = value
			case "--variables-json":
				opts.VariablesJSON = value
			case "--variables-file":
				opts.VariablesFile = value
			case "--operation-name":
				opts.OperationName = strings.TrimSpace(value)
			}
			idx = next
		default:
			return Options{}, domainerrors.New(domainerrors.CodeInvalidArgs, fmt.Sprintf("unknown graphql-query option: %s", name), nil)
		}
	}
	if opts.Query != "" && opts.File != "" {
		return Options{}, domainerrors.New(domainerrors.CodeInvalidArgs, "--query and --file are mutually exclusive", nil)
	}
	if opts.Query == "" && opts.File == "" {
		return Options{}, domainerrors.New(domainerrors.CodeInvalidArgs, "one of --query or --file is required", nil)
	}
	if opts.VariablesJSON != "" && opts.VariablesFile != "" {
		return Options{}, domainerrors.New(domainerrors.CodeInvalidArgs, "--variables-json and --variables-file are mutually exclusive", nil)
	}
	if opts.File == "-" && opts.VariablesFile == "-" {
		return Options{}, domainerrors.New(domainerrors.CodeInvalidArgs, "--file - and --variables-file - cannot both read stdin", nil)
	}
	return opts, nil
}

func NormalizePaths(global requests.GlobalOptions, opts Options) (Paths, *domainerrors.AppError) {
	if global.RequireAbsolutePaths {
		if !filepath.IsAbs(global.Workspace) {
			return Paths{}, domainerrors.New(domainerrors.CodeInvalidArgs, "--workspace must be absolute when --require-absolute-paths is enabled", nil)
		}
		if !filepath.IsAbs(global.SchemaPath) {
			return Paths{}, domainerrors.New(domainerrors.CodeInvalidArgs, "--schema must be absolute when --require-absolute-paths is enabled", nil)
		}
		if opts.File != "" && opts.File != "-" && !filepath.IsAbs(opts.File) {
			return Paths{}, domainerrors.New(domainerrors.CodeInvalidArgs, "--file must be absolute when --require-absolute-paths is enabled", nil)
		}
		if opts.VariablesFile != "" && opts.VariablesFile != "-" && !filepath.IsAbs(opts.VariablesFile) {
			return Paths{}, domainerrors.New(domainerrors.CodeInvalidArgs, "--variables-file must be absolute when --require-absolute-paths is enabled", nil)
		}
	}
	workspacePath, err := filepath.Abs(global.Workspace)
	if err != nil {
		return Paths{}, domainerrors.New(domainerrors.CodeInvalidArgs, "failed to resolve workspace path", map[string]any{"reason": err.Error()})
	}
	schemaPath, err := filepath.Abs(global.SchemaPath)
	if err != nil {
		return Paths{}, domainerrors.New(domainerrors.CodeInvalidArgs, "failed to resolve schema path", map[string]any{"reason": err.Error()})
	}
	paths := Paths{WorkspacePath: workspacePath, SchemaPath: schemaPath}
	if opts.File != "" {
		paths.QueryFile = opts.File
		if opts.File != "-" {
			paths.QueryFile, err = filepath.Abs(opts.File)
			if err != nil {
				return Paths{}, domainerrors.New(domainerrors.CodeInvalidArgs, "failed to resolve --file path", map[string]any{"reason": err.Error()})
			}
		}
	}
	if opts.VariablesFile != "" {
		paths.VariablesFile = opts.VariablesFile
		if opts.VariablesFile != "-" {
			paths.VariablesFile, err = filepath.Abs(opts.VariablesFile)
			if err != nil {
				return Paths{}, domainerrors.New(domainerrors.CodeInvalidArgs, "failed to resolve --variables-file path", map[string]any{"reason": err.Error()})
			}
		}
	}
	return paths, nil
}

func splitLongFlag(token string) (string, string, bool) {
	parts := strings.SplitN(token, "=", 2)
	if len(parts) == 2 {
		return parts[0], parts[1], true
	}
	return token, "", false
}

func valueWithFallback(args []string, idx int, hasInline bool, inline string) (string, int, *domainerrors.AppError) {
	if hasInline {
		return inline, idx, nil
	}
	next := idx + 1
	if next >= len(args) || strings.HasPrefix(args[next], "--") {
		return "", idx, domainerrors.New(domainerrors.CodeInvalidArgs, fmt.Sprintf("missing value for %s", args[idx]), nil)
	}
	return args[next], next, nil
}
