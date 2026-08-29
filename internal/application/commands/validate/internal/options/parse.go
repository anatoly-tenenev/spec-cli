// Package options parses the validate command's arguments and resolves its
// paths: which types to check, whether to stop at the first error, and whether
// warnings count as errors.
//
// parse.go parses arguments; paths.go resolves workspace and schema paths.
package options

import (
	"fmt"
	"github.com/anatoly-tenenev/spec-cli/internal/cliflags"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func Parse(args []string) (model.Options, *domainerrors.AppError) {
	opts := model.Options{TypeFilters: map[string]struct{}{}}

	for idx := 0; idx < len(args); idx++ {
		token := args[idx]
		name, inlineValue, hasInlineValue := cliflags.SplitLong(token)

		switch name {
		case "--type":
			value, nextIdx, err := cliflags.Value(args, idx, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			value = strings.TrimSpace(value)
			if value == "" {
				return model.Options{}, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					"--type value cannot be empty",
					nil,
				)
			}
			opts.TypeFilters[value] = struct{}{}
			idx = nextIdx
		case "--fail-fast":
			parsed, err := cliflags.Bool(name, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			opts.FailFast = parsed
		case "--warnings-as-errors":
			parsed, err := cliflags.Bool(name, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			opts.WarningsAsErrors = parsed
		default:
			return model.Options{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unknown validate option: %s", token),
				nil,
			)
		}
	}

	return opts, nil
}
