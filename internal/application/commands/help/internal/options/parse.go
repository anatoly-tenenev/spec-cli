package options

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

type Parsed struct {
	TargetCommand string
}

type Paths struct {
	SchemaPath         string
	SchemaResolvedPath string
}

const fixedPathRootEnv = "SPEC_CLI_FIXED_PATH_ROOT"

func Parse(args []string) (Parsed, *domainerrors.AppError) {
	if len(args) == 0 {
		return Parsed{}, nil
	}
	if len(args) > 1 {
		return Parsed{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"help accepts at most one positional argument: <command>",
			nil,
		)
	}

	token := strings.TrimSpace(args[0])
	if token == "" {
		return Parsed{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"help command argument cannot be empty",
			nil,
		)
	}
	if strings.HasPrefix(token, "-") {
		return Parsed{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			fmt.Sprintf("unknown help option: %s", token),
			nil,
		)
	}

	return Parsed{TargetCommand: token}, nil
}

func NormalizePaths(global requests.GlobalOptions) (Paths, *domainerrors.AppError) {
	workspacePath, workspaceErr := filepath.Abs(global.Workspace)
	if workspaceErr != nil {
		return Paths{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"failed to resolve workspace path",
			nil,
		)
	}

	schemaPath, schemaErr := filepath.Abs(global.SchemaPath)
	if schemaErr != nil {
		return Paths{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"failed to resolve schema path",
			nil,
		)
	}

	return Paths{
		SchemaPath:         schemaPath,
		SchemaResolvedPath: schemaResolvedPath(workspacePath, schemaPath),
	}, nil
}

func schemaResolvedPath(workspaceAbs string, schemaAbs string) string {
	fixedRoot := strings.TrimSpace(os.Getenv(fixedPathRootEnv))
	if fixedRoot == "" {
		return filepath.ToSlash(filepath.Clean(schemaAbs))
	}

	fixedRootPath := filepath.Clean(fixedRoot)
	if !filepath.IsAbs(fixedRootPath) {
		absRoot, err := filepath.Abs(fixedRootPath)
		if err != nil {
			return filepath.ToSlash(filepath.Clean(schemaAbs))
		}
		fixedRootPath = absRoot
	}

	rel, err := filepath.Rel(workspaceAbs, schemaAbs)
	if err != nil {
		return filepath.ToSlash(filepath.Clean(schemaAbs))
	}

	// Keep deterministic anchor name so snapshots stay stable across random temp dirs.
	fixedWorkspace := filepath.Join(fixedRootPath, "workspace")
	resolvedPath := filepath.Clean(filepath.Join(fixedWorkspace, rel))
	return filepath.ToSlash(resolvedPath)
}
