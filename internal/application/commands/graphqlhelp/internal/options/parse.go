package options

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

type Options struct {
	SchemaOnly bool
	Entities   []string
}

func Parse(args []string) (Options, *domainerrors.AppError) {
	opts := Options{Entities: []string{}}
	for idx := 0; idx < len(args); idx++ {
		token := args[idx]
		name, inline, hasInline := splitLongFlag(token)
		if !strings.HasPrefix(name, "--") {
			return Options{}, domainerrors.New(domainerrors.CodeInvalidArgs, fmt.Sprintf("unknown graphql-help option: %s", token), nil)
		}
		switch name {
		case "--schema-only":
			if hasInline {
				return Options{}, domainerrors.New(domainerrors.CodeInvalidArgs, "--schema-only does not take a value", nil)
			}
			opts.SchemaOnly = true
		case "--entity":
			value, next, err := valueWithFallback(args, idx, hasInline, inline)
			if err != nil {
				return Options{}, err
			}
			value = strings.TrimSpace(value)
			if value == "" {
				return Options{}, domainerrors.New(domainerrors.CodeInvalidArgs, "--entity value cannot be empty", nil)
			}
			opts.Entities = append(opts.Entities, value)
			idx = next
		default:
			return Options{}, domainerrors.New(domainerrors.CodeInvalidArgs, fmt.Sprintf("unknown graphql-help option: %s", name), nil)
		}
	}
	if len(opts.Entities) > 0 && !opts.SchemaOnly {
		return Options{}, domainerrors.New(domainerrors.CodeInvalidArgs, "--entity is allowed only with --schema-only", nil)
	}
	return opts, nil
}

func NormalizePaths(global requests.GlobalOptions) (string, string, *domainerrors.AppError) {
	if global.RequireAbsolutePaths {
		if !filepath.IsAbs(global.Workspace) {
			return "", "", domainerrors.New(domainerrors.CodeInvalidArgs, "--workspace must be absolute when --require-absolute-paths is enabled", nil)
		}
		if !filepath.IsAbs(global.SchemaPath) {
			return "", "", domainerrors.New(domainerrors.CodeInvalidArgs, "--schema must be absolute when --require-absolute-paths is enabled", nil)
		}
	}
	workspacePath, err := filepath.Abs(global.Workspace)
	if err != nil {
		return "", "", domainerrors.New(domainerrors.CodeInvalidArgs, "failed to resolve workspace path", map[string]any{"reason": err.Error()})
	}
	schemaPath, err := filepath.Abs(global.SchemaPath)
	if err != nil {
		return "", "", domainerrors.New(domainerrors.CodeInvalidArgs, "failed to resolve schema path", map[string]any{"reason": err.Error()})
	}
	return workspacePath, schemaPath, nil
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
