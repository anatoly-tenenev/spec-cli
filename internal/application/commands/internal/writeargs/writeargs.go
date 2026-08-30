// Package writeargs holds the argument surface add and update share: the
// <path=value> form of --set and --set-file, the rule that whole-body input
// and a section write cannot both describe the body, and the resolution of
// the file paths those options carry.
//
// The two commands accept the same options against the same schema, so an
// argument one of them rejects must be rejected by the other, with the same
// message. Where they genuinely differ - update also has --unset, and tracks
// body replacement as its own operation - the difference stays in the command.
package writeargs

import (
	"path/filepath"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/writemodel"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

// PathValue is one --set or --set-file argument split into the write path and
// the text after the first '='. The value is kept raw: what it means depends
// on the field's declared type, which only the schema knows.
type PathValue struct {
	Path  string
	Value string
}

// ParsePathValue splits a <path=value> argument. Only the first '=' separates,
// so a value may contain further ones.
func ParsePathValue(raw string) (PathValue, *domainerrors.AppError) {
	eqIdx := strings.Index(raw, "=")
	if eqIdx <= 0 {
		return PathValue{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"write operation must match <path=value>",
			map[string]any{"value": raw},
		)
	}

	path := strings.TrimSpace(raw[:eqIdx])
	value := raw[eqIdx+1:]
	if path == "" {
		return PathValue{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"write path cannot be empty",
			nil,
		)
	}

	return PathValue{Path: path, Value: value}, nil
}

// ParseSetFile splits a --set-file argument. Its value names a file, so an
// empty one is refused here rather than surfacing later as a failed read.
func ParseSetFile(raw string) (PathValue, *domainerrors.AppError) {
	parsed, parseErr := ParsePathValue(raw)
	if parseErr != nil {
		return PathValue{}, parseErr
	}
	if strings.TrimSpace(parsed.Value) == "" {
		return PathValue{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"--set-file requires non-empty file path",
			nil,
		)
	}
	return parsed, nil
}

// HasSectionWrite reports whether any operation addresses a content section.
// Both commands refuse to combine one with whole-body input, because the two
// state different bodies for the same document.
func HasSectionWrite(operations []writemodel.WriteOperation) bool {
	for _, op := range operations {
		if strings.HasPrefix(op.Path, "content.sections.") {
			return true
		}
	}
	return false
}

// NormalizeContentFile resolves the --content-file path against the global
// options. add and update name the option the same way to the caller, so the
// refusal reads the same for both.
func NormalizeContentFile(global requests.GlobalOptions, value string) (string, *domainerrors.AppError) {
	if global.RequireAbsolutePaths && !filepath.IsAbs(value) {
		return "", domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"--content-file must be absolute when --require-absolute-paths is enabled",
			nil,
		)
	}

	absolutePath, pathErr := filepath.Abs(value)
	if pathErr != nil {
		return "", domainerrors.New(
			domainerrors.CodeInvalidArgs,
			"failed to resolve --content-file path",
			map[string]any{"reason": pathErr.Error()},
		)
	}

	return absolutePath, nil
}

// NormalizeOperationPaths resolves the source path of every --set-file
// operation, leaving other operations untouched. It always returns a fresh
// slice, so the caller's parsed options are not mutated.
func NormalizeOperationPaths(
	global requests.GlobalOptions,
	operations []writemodel.WriteOperation,
) ([]writemodel.WriteOperation, *domainerrors.AppError) {
	normalized := make([]writemodel.WriteOperation, 0, len(operations))
	for _, op := range operations {
		nextOp := op
		if op.Kind == writemodel.WriteOperationSetFile {
			if global.RequireAbsolutePaths && !filepath.IsAbs(op.RawValue) {
				return nil, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					"--set-file path must be absolute when --require-absolute-paths is enabled",
					map[string]any{"path": op.Path},
				)
			}
			absolutePath, pathErr := filepath.Abs(op.RawValue)
			if pathErr != nil {
				return nil, domainerrors.New(
					domainerrors.CodeInvalidArgs,
					"failed to resolve --set-file path",
					map[string]any{"path": op.Path, "reason": pathErr.Error()},
				)
			}
			nextOp.RawValue = absolutePath
		}
		normalized = append(normalized, nextOp)
	}

	return normalized, nil
}
