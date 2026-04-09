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
	TargetCommand        string
	ShowSchemaProjection bool
}

type Paths struct {
	WorkspacePath      string
	WorkspaceDisplay   string
	SchemaPath         string
	SchemaResolvedPath string
}

const fixedPathRootEnv = "SPEC_CLI_FIXED_PATH_ROOT"
const optionShowSchemaProjection = "--show-schema-projection"

func Parse(args []string) (Parsed, *domainerrors.AppError) {
	parsed := Parsed{}
	seenOptions := map[string]struct{}{}

	for _, rawToken := range args {
		token := strings.TrimSpace(rawToken)
		if token == "" {
			return Parsed{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				"help command argument cannot be empty",
				nil,
			)
		}

		if strings.HasPrefix(token, "-") {
			optionName, optionValue, hasValue := splitLongFlag(token)
			switch optionName {
			case optionShowSchemaProjection:
				if _, exists := seenOptions[optionName]; exists {
					return Parsed{}, domainerrors.New(
						domainerrors.CodeInvalidArgs,
						fmt.Sprintf("duplicate help option: %s", optionShowSchemaProjection),
						nil,
					)
				}
				if hasValue {
					return Parsed{}, domainerrors.New(
						domainerrors.CodeInvalidArgs,
						fmt.Sprintf("%s does not accept a value", optionShowSchemaProjection),
						map[string]any{"value": optionValue},
					)
				}
				seenOptions[optionName] = struct{}{}
				parsed.ShowSchemaProjection = true
				continue
			default:
				return Parsed{}, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					fmt.Sprintf("unknown help option: %s", token),
					nil,
				)
			}
		}

		if parsed.TargetCommand != "" {
			return Parsed{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				"help accepts at most one positional argument: <command>",
				nil,
			)
		}
		parsed.TargetCommand = token
	}

	return parsed, nil
}

func splitLongFlag(token string) (string, string, bool) {
	optionName, optionValue, hasValue := strings.Cut(token, "=")
	return optionName, optionValue, hasValue
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

	fixedRootPath, hasFixedRoot := fixedRootPath()

	return Paths{
		WorkspacePath:      workspacePath,
		WorkspaceDisplay:   workspaceDisplayPath(global.Workspace, workspacePath, fixedRootPath, hasFixedRoot),
		SchemaPath:         schemaPath,
		SchemaResolvedPath: schemaResolvedPath(workspacePath, schemaPath),
	}, nil
}

func workspaceDisplayPath(rawWorkspace string, workspaceAbs string, fixedRootPath string, hasFixedRoot bool) string {
	if hasFixedRoot {
		return filepath.ToSlash(filepath.Join(fixedRootPath, "workspace"))
	}

	normalized := filepath.ToSlash(filepath.Clean(strings.TrimSpace(rawWorkspace)))
	if normalized == "" {
		return filepath.ToSlash(filepath.Clean(workspaceAbs))
	}
	return normalized
}

func schemaResolvedPath(workspaceAbs string, schemaAbs string) string {
	fixedRootPath, hasFixedRoot := fixedRootPath()
	if !hasFixedRoot {
		return filepath.ToSlash(filepath.Clean(schemaAbs))
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

func fixedRootPath() (string, bool) {
	fixedRoot := strings.TrimSpace(os.Getenv(fixedPathRootEnv))
	if fixedRoot == "" {
		return "", false
	}

	rootPath := filepath.Clean(fixedRoot)
	if filepath.IsAbs(rootPath) {
		return rootPath, true
	}

	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return "", false
	}
	return absRoot, true
}
