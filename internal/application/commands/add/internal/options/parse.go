// Package options parses the add command's arguments into write operations and
// resolves its paths. Two writes to the same path are refused rather than
// resolved last-one-wins: the caller stated two values for one field, and
// picking either would be a guess.
//
// Values are kept as raw text here; what they mean depends on the field's
// declared type, which only the schema knows.
//
// parse.go parses arguments; paths.go resolves workspace, schema and file
// paths.
package options

import (
	"fmt"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/writeargs"
	"github.com/anatoly-tenenev/spec-cli/internal/cliflags"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func Parse(args []string) (model.Options, *domainerrors.AppError) {
	opts := model.Options{Operations: []model.WriteOperation{}}
	seenPaths := map[string]struct{}{}

	for idx := 0; idx < len(args); idx++ {
		token := args[idx]
		name, inlineValue, hasInlineValue := cliflags.SplitLong(token)
		if !strings.HasPrefix(name, "--") {
			return model.Options{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unknown add option: %s", token),
				nil,
			)
		}

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
			opts.EntityType = value
			idx = nextIdx
		case "--slug":
			value, nextIdx, err := cliflags.Value(args, idx, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			value = strings.TrimSpace(value)
			if value == "" {
				return model.Options{}, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					"--slug value cannot be empty",
					nil,
				)
			}
			opts.Slug = value
			idx = nextIdx
		case "--set", "--set-file":
			value, nextIdx, err := cliflags.ValueAllowDash(args, idx, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}

			// The two options differ only in what the value is - a value to
			// write, or the file to read it from - and are registered alike.
			kind := model.WriteOperationSet
			parse := writeargs.ParsePathValue
			if name == "--set-file" {
				kind = model.WriteOperationSetFile
				parse = writeargs.ParseSetFile
			}

			op, parseErr := parse(value)
			if parseErr != nil {
				return model.Options{}, parseErr
			}
			if _, exists := seenPaths[op.Path]; exists {
				return model.Options{}, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					fmt.Sprintf("duplicate write path: %s", op.Path),
					map[string]any{"path": op.Path},
				)
			}
			seenPaths[op.Path] = struct{}{}
			opts.Operations = append(opts.Operations, model.WriteOperation{
				Kind:     kind,
				Path:     op.Path,
				RawValue: op.Value,
			})
			idx = nextIdx
		case "--content-file":
			value, nextIdx, err := cliflags.Value(args, idx, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				return model.Options{}, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					"--content-file value cannot be empty",
					nil,
				)
			}
			opts.ContentFile = trimmed
			idx = nextIdx
		case "--content-stdin":
			parsed, err := cliflags.BoolWords(name, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			opts.ContentStdin = parsed
		case "--dry-run":
			parsed, err := cliflags.BoolWords(name, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			opts.DryRun = parsed
		default:
			return model.Options{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unknown add option: %s", name),
				nil,
			)
		}
	}

	if opts.EntityType == "" {
		return model.Options{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"--type is required",
			nil,
		)
	}
	if opts.Slug == "" {
		return model.Options{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"--slug is required",
			nil,
		)
	}

	if opts.ContentFile != "" && opts.ContentStdin {
		return model.Options{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"--content-file and --content-stdin are mutually exclusive",
			nil,
		)
	}

	if (opts.ContentFile != "" || opts.ContentStdin) && writeargs.HasSectionWrite(opts.Operations) {
		return model.Options{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"whole-body input cannot be combined with content.sections.* write-paths",
			nil,
		)
	}

	return opts, nil
}
