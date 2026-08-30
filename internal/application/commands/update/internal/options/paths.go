package options

import (
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/optionpaths"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/writeargs"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func NormalizePaths(global requests.GlobalOptions, opts model.Options) (string, string, model.Options, *domainerrors.AppError) {
	workspacePath, schemaPath, pathErr := optionpaths.Normalize(global)
	if pathErr != nil {
		return "", "", model.Options{}, pathErr
	}

	normalized := opts
	if normalized.BodyOperation == model.BodyOperationReplaceFile {
		contentPath, contentErr := writeargs.NormalizeContentFile(global, normalized.BodyFile)
		if contentErr != nil {
			return "", "", model.Options{}, contentErr
		}
		normalized.BodyFile = contentPath
	}

	normalizedOps, opsErr := writeargs.NormalizeOperationPaths(global, opts.Operations)
	if opsErr != nil {
		return "", "", model.Options{}, opsErr
	}
	normalized.Operations = normalizedOps

	return workspacePath, schemaPath, normalized, nil
}
