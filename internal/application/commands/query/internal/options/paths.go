package options

import (
	"path/filepath"

	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func NormalizePaths(global requests.GlobalOptions) (string, string, *domainerrors.AppError) {
	if global.RequireAbsolutePaths {
		if !filepath.IsAbs(global.Workspace) {
			return "", "", domainerrors.New(
				domainerrors.CodeInvalidArgs,
				"--workspace must be absolute when --require-absolute-paths is enabled",
				nil,
			)
		}
		if !filepath.IsAbs(global.SchemaPath) {
			return "", "", domainerrors.New(
				domainerrors.CodeInvalidArgs,
				"--schema must be absolute when --require-absolute-paths is enabled",
				nil,
			)
		}
	}

	workspacePath, err := filepath.Abs(global.Workspace)
	if err != nil {
		return "", "", domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"failed to resolve workspace path",
			map[string]any{"reason": err.Error()},
		)
	}

	schemaPath, err := filepath.Abs(global.SchemaPath)
	if err != nil {
		return "", "", domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"failed to resolve schema path",
			map[string]any{"reason": err.Error()},
		)
	}

	return workspacePath, schemaPath, nil
}
