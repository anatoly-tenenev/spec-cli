// Package options parses the update command's arguments into patch operations
// and resolves its paths. Two operations on one path are refused rather than
// applied in sequence: a --set and an --unset of the same field state opposite
// intentions, and picking either one silently would be a guess.
//
// Whole-body replacement is tracked separately from per-path writes, because
// it and a section write cannot both describe the same body.
//
// parse.go parses arguments; paths.go resolves workspace, schema and file
// paths.
package options

import (
	"fmt"
	"github.com/anatoly-tenenev/spec-cli/internal/cliflags"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func Parse(args []string) (model.Options, *domainerrors.AppError) {
	opts := model.Options{
		Operations:    []model.WriteOperation{},
		BodyOperation: model.BodyOperationNone,
	}

	seenPaths := map[string]model.WriteOperationKind{}
	contentFile := ""
	contentStdin := false
	clearContent := false

	for idx := 0; idx < len(args); idx++ {
		token := args[idx]
		name, inlineValue, hasInlineValue := cliflags.SplitLong(token)
		if !strings.HasPrefix(name, "--") {
			return model.Options{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unknown update option: %s", token),
				nil,
			)
		}

		switch name {
		case "--id":
			value, nextIdx, err := cliflags.Value(args, idx, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			value = strings.TrimSpace(value)
			if value == "" {
				return model.Options{}, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					"--id value cannot be empty",
					nil,
				)
			}
			opts.ID = value
			idx = nextIdx
		case "--set":
			value, nextIdx, err := cliflags.ValueAllowDash(args, idx, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			op, parseErr := parsePathValue(value)
			if parseErr != nil {
				return model.Options{}, parseErr
			}
			if duplicateErr := registerWritePath(seenPaths, op.Path, model.WriteOperationSet); duplicateErr != nil {
				return model.Options{}, duplicateErr
			}
			opts.Operations = append(opts.Operations, model.WriteOperation{
				Kind:     model.WriteOperationSet,
				Path:     op.Path,
				RawValue: op.Value,
			})
			idx = nextIdx
		case "--set-file":
			value, nextIdx, err := cliflags.ValueAllowDash(args, idx, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			op, parseErr := parsePathValue(value)
			if parseErr != nil {
				return model.Options{}, parseErr
			}
			if strings.TrimSpace(op.Value) == "" {
				return model.Options{}, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					"--set-file requires non-empty file path",
					nil,
				)
			}
			if duplicateErr := registerWritePath(seenPaths, op.Path, model.WriteOperationSetFile); duplicateErr != nil {
				return model.Options{}, duplicateErr
			}
			opts.Operations = append(opts.Operations, model.WriteOperation{
				Kind:     model.WriteOperationSetFile,
				Path:     op.Path,
				RawValue: op.Value,
			})
			idx = nextIdx
		case "--unset":
			value, nextIdx, err := cliflags.Value(args, idx, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			path := strings.TrimSpace(value)
			if path == "" {
				return model.Options{}, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					"--unset path cannot be empty",
					nil,
				)
			}
			if duplicateErr := registerWritePath(seenPaths, path, model.WriteOperationUnset); duplicateErr != nil {
				return model.Options{}, duplicateErr
			}
			opts.Operations = append(opts.Operations, model.WriteOperation{
				Kind: model.WriteOperationUnset,
				Path: path,
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
			contentFile = trimmed
			idx = nextIdx
		case "--content-stdin":
			parsed, err := cliflags.BoolWords(name, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			contentStdin = parsed
		case "--clear-content":
			parsed, err := cliflags.BoolWords(name, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			clearContent = parsed
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
			parsed, err := cliflags.BoolWords(name, hasInlineValue, inlineValue)
			if err != nil {
				return model.Options{}, err
			}
			opts.DryRun = parsed
		default:
			return model.Options{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unknown update option: %s", name),
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

	bodyOpCount := 0
	if contentFile != "" {
		bodyOpCount++
		opts.BodyOperation = model.BodyOperationReplaceFile
		opts.BodyFile = contentFile
	}
	if contentStdin {
		bodyOpCount++
		opts.BodyOperation = model.BodyOperationReplaceSTDIN
	}
	if clearContent {
		bodyOpCount++
		opts.BodyOperation = model.BodyOperationClear
	}
	if bodyOpCount > 1 {
		return model.Options{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"--content-file, --content-stdin and --clear-content are mutually exclusive",
			nil,
		)
	}

	if len(opts.Operations) == 0 && opts.BodyOperation == model.BodyOperationNone {
		return model.Options{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"at least one patch operation is required",
			nil,
		)
	}

	if opts.BodyOperation != model.BodyOperationNone && hasSectionWrite(opts.Operations) {
		return model.Options{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"whole-body input cannot be combined with content.sections.* patch operations",
			nil,
		)
	}

	return opts, nil
}

func registerWritePath(
	seen map[string]model.WriteOperationKind,
	path string,
	kind model.WriteOperationKind,
) *domainerrors.AppError {
	if existing, ok := seen[path]; ok {
		if (existing == model.WriteOperationUnset && kind != model.WriteOperationUnset) ||
			(existing != model.WriteOperationUnset && kind == model.WriteOperationUnset) {
			return domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("path '%s' cannot be used in both set and unset operations", path),
				map[string]any{"path": path},
			)
		}
		return domainerrors.New(
			domainerrors.CodeInvalidArgs,
			fmt.Sprintf("duplicate write path: %s", path),
			map[string]any{"path": path},
		)
	}

	seen[path] = kind
	return nil
}

type pathValue struct {
	Path  string
	Value string
}

func parsePathValue(raw string) (pathValue, *domainerrors.AppError) {
	eqIdx := strings.Index(raw, "=")
	if eqIdx <= 0 {
		return pathValue{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"write operation must match <path=value>",
			map[string]any{"value": raw},
		)
	}

	path := strings.TrimSpace(raw[:eqIdx])
	value := raw[eqIdx+1:]
	if path == "" {
		return pathValue{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"write path cannot be empty",
			nil,
		)
	}

	return pathValue{Path: path, Value: value}, nil
}

func hasSectionWrite(operations []model.WriteOperation) bool {
	for _, op := range operations {
		if strings.HasPrefix(op.Path, "content.sections.") {
			return true
		}
	}
	return false
}
