package options

import (
	"path/filepath"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func NormalizePaths(global requests.GlobalOptions, opts model.Options) (string, string, model.Options, *domainerrors.AppError) {
	if global.RequireAbsolutePaths {
		if !filepath.IsAbs(global.Workspace) {
			return "", "", model.Options{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				"--workspace must be absolute when --require-absolute-paths is enabled",
				nil,
			)
		}
		if !filepath.IsAbs(global.SchemaPath) {
			return "", "", model.Options{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				"--schema must be absolute when --require-absolute-paths is enabled",
				nil,
			)
		}
	}

	workspacePath, err := filepath.Abs(global.Workspace)
	if err != nil {
		return "", "", model.Options{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"failed to resolve workspace path",
			map[string]any{"reason": err.Error()},
		)
	}

	schemaPath, err := filepath.Abs(global.SchemaPath)
	if err != nil {
		return "", "", model.Options{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"failed to resolve schema path",
			map[string]any{"reason": err.Error()},
		)
	}

	normalized := opts
	if normalized.BodyOperation == model.BodyOperationReplaceFile {
		if global.RequireAbsolutePaths && !filepath.IsAbs(normalized.BodyFile) {
			return "", "", model.Options{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				"--content-file must be absolute when --require-absolute-paths is enabled",
				nil,
			)
		}
		contentPath, pathErr := filepath.Abs(normalized.BodyFile)
		if pathErr != nil {
			return "", "", model.Options{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				"failed to resolve --content-file path",
				map[string]any{"reason": pathErr.Error()},
			)
		}
		normalized.BodyFile = contentPath
	}

	normalizedOps := make([]model.WriteOperation, 0, len(opts.Operations))
	for _, op := range opts.Operations {
		nextOp := op
		if op.Kind == model.WriteOperationSetFile {
			if global.RequireAbsolutePaths && !filepath.IsAbs(op.RawValue) {
				return "", "", model.Options{}, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					"--set-file path must be absolute when --require-absolute-paths is enabled",
					map[string]any{"path": op.Path},
				)
			}
			absolutePath, pathErr := filepath.Abs(op.RawValue)
			if pathErr != nil {
				return "", "", model.Options{}, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					"failed to resolve --set-file path",
					map[string]any{"path": op.Path, "reason": pathErr.Error()},
				)
			}
			nextOp.RawValue = absolutePath
		}
		normalizedOps = append(normalizedOps, nextOp)
	}
	normalized.Operations = normalizedOps

	return workspacePath, schemaPath, normalized, nil
}
