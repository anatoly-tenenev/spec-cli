package options

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

type Options struct {
	Subcommand string
}

func Parse(args []string) (Options, *domainerrors.AppError) {
	if len(args) == 0 {
		return Options{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"schema subcommand is required",
			map[string]any{"allowed_subcommands": []string{"check"}},
		)
	}

	subcommand := strings.TrimSpace(args[0])
	if strings.HasPrefix(subcommand, "-") {
		return Options{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			fmt.Sprintf("unknown schema option: %s", args[0]),
			nil,
		)
	}

	if subcommand != "check" {
		return Options{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			fmt.Sprintf("unknown schema subcommand: %s", subcommand),
			map[string]any{"subcommand": subcommand},
		)
	}

	if len(args) > 1 {
		extra := args[1]
		if strings.HasPrefix(extra, "-") {
			return Options{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unknown schema option: %s", extra),
				nil,
			)
		}
		return Options{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			fmt.Sprintf("unexpected schema argument: %s", extra),
			nil,
		)
	}

	return Options{Subcommand: subcommand}, nil
}

func NormalizeSchemaPath(global requests.GlobalOptions) (string, *domainerrors.AppError) {
	schemaPath, err := filepath.Abs(global.SchemaPath)
	if err != nil {
		return "", domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"failed to resolve schema path",
			map[string]any{"reason": err.Error()},
		)
	}
	return schemaPath, nil
}
