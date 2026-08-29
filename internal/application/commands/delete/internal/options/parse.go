// Package options parses the delete command's arguments and resolves its
// paths: the target id, the revision the caller expects to be deleting, and
// whether to stop short of removing anything.
//
// parse.go parses arguments; paths.go resolves workspace and schema paths.
package options

import (
	"fmt"
	"github.com/anatoly-tenenev/spec-cli/internal/cliflags"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/delete/internal/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func Parse(args []string) (model.Options, *domainerrors.AppError) {
	opts := model.Options{}

	for idx := 0; idx < len(args); idx++ {
		token := args[idx]

		name, inlineValue, hasInlineValue := cliflags.SplitLong(token)
		if !strings.HasPrefix(name, "--") {
			return model.Options{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unknown delete option: %s", token),
				nil,
			)
		}

		switch name {
		case "--id":
			value, nextIdx, err := cliflags.Value(args, idx, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				return model.Options{}, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					"--id value cannot be empty",
					nil,
				)
			}
			opts.ID = trimmed
			idx = nextIdx
		case "--expect-revision":
			value, nextIdx, err := cliflags.Value(args, idx, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				return model.Options{}, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					"--expect-revision value cannot be empty",
					nil,
				)
			}
			opts.ExpectRevision = trimmed
			idx = nextIdx
		case "--dry-run":
			parsed, err := cliflags.Bool(name, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			opts.DryRun = parsed
		default:
			return model.Options{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unknown delete option: %s", name),
				nil,
			)
		}
	}

	if opts.ID == "" {
		return model.Options{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"--id is required",
			nil,
		)
	}

	return opts, nil
}
