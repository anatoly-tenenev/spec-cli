package writes

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/support"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/workspace"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

type Applied struct {
	Frontmatter map[string]any
	Body        string
	UserChanged bool
	Changes     []map[string]any
}

type preparedOperation struct {
	Kind  model.WriteOperationKind
	Path  string
	Spec  model.WritePathSpec
	Value any
}

func Apply(
	opts model.Options,
	typeSpec model.EntityTypeSpec,
	frontmatter map[string]any,
	body string,
) (Applied, *domainerrors.AppError) {
	prepared, bodyValue, preflightErr := preflight(opts, typeSpec)
	if preflightErr != nil {
		return Applied{}, preflightErr
	}

	nextFrontmatter := cloneMap(frontmatter)
	nextBody := strings.ReplaceAll(body, "\r\n", "\n")
	changes := make([]map[string]any, 0, len(prepared)+1)
	userChanged := false

	for _, op := range prepared {
		switch op.Spec.Kind {
		case model.WritePathMeta, model.WritePathRef:
			before, exists := nextFrontmatter[op.Spec.FieldName]

			switch op.Kind {
			case model.WriteOperationSet, model.WriteOperationSetFile:
				after := support.NormalizeValue(op.Value)
				if exists && support.LiteralEqual(support.NormalizeValue(before), after) {
					continue
				}
				nextFrontmatter[op.Spec.FieldName] = after
				userChanged = true
				changes = append(changes, map[string]any{
					"field":  op.Path,
					"op":     "set",
					"before": normalizeScalarOrNil(before, exists),
					"after":  after,
				})
			case model.WriteOperationUnset:
				if !exists {
					continue
				}
				delete(nextFrontmatter, op.Spec.FieldName)
				userChanged = true
				changes = append(changes, map[string]any{
					"field":  op.Path,
					"op":     "unset",
					"before": normalizeScalarOrNil(before, true),
					"after":  nil,
				})
			}
		case model.WritePathSection:
			label := op.Spec.FieldName
			layout := workspace.BuildSectionLayout(nextBody)
			beforeRange, beforePresent := layout.FirstRange(label)
			beforeBody := ""
			if beforePresent {
				beforeBody = strings.Join(layout.Lines[beforeRange.BodyStart:beforeRange.EndLine], "\n")
			}

			switch op.Kind {
			case model.WriteOperationSet, model.WriteOperationSetFile:
				afterBody := op.Value.(string)
				if beforePresent {
					if beforeBody == afterBody {
						continue
					}
					updatedLines := replaceLines(
						layout.Lines,
						beforeRange.BodyStart,
						beforeRange.EndLine,
						splitRawLines(afterBody),
					)
					nextBody = strings.Join(updatedLines, "\n")
				} else {
					headingTitle := label
					if sectionSpec, ok := typeSpec.Sections[label]; ok && strings.TrimSpace(sectionSpec.Title) != "" {
						headingTitle = sectionSpec.Title
					}
					inserted := insertMissingSection(layout, label, headingTitle, afterBody, typeSpec.SectionOrder)
					nextBody = strings.Join(inserted, "\n")
				}

				userChanged = true
				changes = append(changes, map[string]any{
					"field":          op.Path,
					"op":             "set",
					"before_present": beforePresent,
					"after_present":  true,
					"before_hash":    hashOrNil(beforeBody, beforePresent),
					"after_hash":     hashString(afterBody),
				})
			case model.WriteOperationUnset:
				if !beforePresent {
					continue
				}
				nextBody = strings.Join(
					removeSectionRange(layout.Lines, beforeRange.HeadingLine, beforeRange.EndLine),
					"\n",
				)
				userChanged = true
				changes = append(changes, map[string]any{
					"field":          op.Path,
					"op":             "unset",
					"before_present": true,
					"after_present":  false,
					"before_hash":    hashString(beforeBody),
					"after_hash":     nil,
				})
			}
		default:
			return Applied{}, domainerrors.New(
				domainerrors.CodeInternalError,
				"unsupported write-path kind",
				map[string]any{"kind": op.Spec.Kind},
			)
		}
	}

	if opts.BodyOperation != model.BodyOperationNone {
		beforeBody := nextBody
		switch opts.BodyOperation {
		case model.BodyOperationReplaceFile, model.BodyOperationReplaceSTDIN:
			nextBody = bodyValue
		case model.BodyOperationClear:
			nextBody = ""
		}

		if beforeBody != nextBody {
			userChanged = true
			opValue := "replace"
			if opts.BodyOperation == model.BodyOperationClear {
				opValue = "clear"
			}
			changes = append(changes, map[string]any{
				"field":       "content.raw",
				"op":          opValue,
				"before_hash": hashString(beforeBody),
				"after_hash":  hashString(nextBody),
			})
		}
	}

	return Applied{
		Frontmatter: nextFrontmatter,
		Body:        nextBody,
		UserChanged: userChanged,
		Changes:     changes,
	}, nil
}

func preflight(
	opts model.Options,
	typeSpec model.EntityTypeSpec,
) ([]preparedOperation, string, *domainerrors.AppError) {
	operations := make([]preparedOperation, 0, len(opts.Operations))

	for _, op := range opts.Operations {
		writeSpec, exists := typeSpec.AllowWritePaths[op.Path]
		if !exists {
			if isForbiddenWritePath(op.Path) {
				return nil, "", domainerrors.New(
					domainerrors.CodeWriteContractViolation,
					fmt.Sprintf("write path '%s' is forbidden by write contract", op.Path),
					map[string]any{"path": op.Path},
				)
			}
		}
		if op.Kind == model.WriteOperationUnset && !containsPath(typeSpec.UnsetPaths, op.Path) {
			return nil, "", domainerrors.New(
				domainerrors.CodeWriteContractViolation,
				fmt.Sprintf("write path '%s' is not allowed", op.Path),
				map[string]any{"path": op.Path},
			)
		}
		if op.Kind != model.WriteOperationUnset && !containsPath(typeSpec.SetPaths, op.Path) {
			return nil, "", domainerrors.New(
				domainerrors.CodeWriteContractViolation,
				fmt.Sprintf("write path '%s' is not allowed", op.Path),
				map[string]any{"path": op.Path},
			)
		}

		if op.Kind == model.WriteOperationSetFile {
			if _, ok := typeSpec.AllowSetFilePaths[op.Path]; !ok {
				return nil, "", domainerrors.New(
					domainerrors.CodeWriteContractViolation,
					fmt.Sprintf("--set-file is not allowed for path '%s'", op.Path),
					map[string]any{"path": op.Path},
				)
			}
		}

		prepared := preparedOperation{Kind: op.Kind, Path: op.Path, Spec: writeSpec}
		if op.Kind != model.WriteOperationUnset {
			value, valueErr := resolveOperationValue(op, writeSpec, typeSpec)
			if valueErr != nil {
				return nil, "", valueErr
			}
			prepared.Value = value
		}
		operations = append(operations, prepared)
	}

	bodyValue := ""
	if opts.BodyOperation != model.BodyOperationNone && !typeSpec.HasContent {
		return nil, "", domainerrors.New(
			domainerrors.CodeWriteContractViolation,
			"whole-body input is not allowed for entity type without content",
			nil,
		)
	}

	if opts.BodyOperation == model.BodyOperationReplaceFile {
		raw, err := os.ReadFile(opts.BodyFile)
		if err != nil {
			return nil, "", domainerrors.New(
				domainerrors.CodeWriteFailed,
				"failed to read --content-file",
				map[string]any{"reason": err.Error()},
			)
		}
		bodyValue = string(raw)
	}

	if opts.BodyOperation == model.BodyOperationReplaceSTDIN {
		raw, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, "", domainerrors.New(
				domainerrors.CodeWriteFailed,
				"failed to read --content-stdin",
				map[string]any{"reason": err.Error()},
			)
		}
		bodyValue = string(raw)
	}

	return operations, bodyValue, nil
}

func resolveOperationValue(
	op model.WriteOperation,
	writeSpec model.WritePathSpec,
	typeSpec model.EntityTypeSpec,
) (any, *domainerrors.AppError) {
	if op.Kind == model.WriteOperationSetFile {
		raw, err := os.ReadFile(op.RawValue)
		if err != nil {
			return nil, domainerrors.New(
				domainerrors.CodeWriteFailed,
				"failed to read --set-file source",
				map[string]any{"path": op.Path, "reason": err.Error()},
			)
		}
		return string(raw), nil
	}

	switch writeSpec.Kind {
	case model.WritePathMeta:
		field := typeSpec.MetaFields[writeSpec.FieldName]
		parsed, parseErr := support.ParseYAMLValue(op.RawValue)
		if parseErr != nil {
			return nil, domainerrors.New(
				domainerrors.CodeWriteContractViolation,
				fmt.Sprintf("failed to parse value for path '%s'", op.Path),
				map[string]any{"path": op.Path, "reason": parseErr.Error()},
			)
		}
		if !isTypeCompatible(field, parsed) {
			return nil, domainerrors.New(
				domainerrors.CodeWriteContractViolation,
				fmt.Sprintf("value for path '%s' does not match schema type '%s'", op.Path, field.Type),
				map[string]any{
					"path":          op.Path,
					"expected_type": field.Type,
					"actual_type":   describeValueType(parsed),
				},
			)
		}
		return support.NormalizeValue(parsed), nil
	case model.WritePathRef:
		field := typeSpec.MetaFields[writeSpec.FieldName]
		if field.IsEntityRefArray {
			parsed, parseErr := support.ParseYAMLValue(op.RawValue)
			if parseErr != nil {
				return nil, domainerrors.New(
					domainerrors.CodeWriteContractViolation,
					fmt.Sprintf("failed to parse value for path '%s'", op.Path),
					map[string]any{"path": op.Path, "reason": parseErr.Error()},
				)
			}
			items, ok := parsed.([]any)
			if !ok {
				return nil, domainerrors.New(
					domainerrors.CodeWriteContractViolation,
					fmt.Sprintf("value for path '%s' must be array of entity ids", op.Path),
					map[string]any{"path": op.Path, "expected_type": "array", "actual_type": describeValueType(parsed)},
				)
			}
			result := make([]any, 0, len(items))
			for idx, item := range items {
				itemText, ok := item.(string)
				if !ok || strings.TrimSpace(itemText) == "" {
					return nil, domainerrors.New(
						domainerrors.CodeWriteContractViolation,
						fmt.Sprintf("value for path '%s' must contain non-empty string entity ids", op.Path),
						map[string]any{"path": op.Path, "index": idx},
					)
				}
				result = append(result, strings.TrimSpace(itemText))
			}
			return result, nil
		}
		value := strings.TrimSpace(op.RawValue)
		if value == "" {
			return nil, domainerrors.New(
				domainerrors.CodeWriteContractViolation,
				fmt.Sprintf("path '%s' requires non-empty target entity id", op.Path),
				map[string]any{"path": op.Path},
			)
		}
		return value, nil
	case model.WritePathSection:
		return op.RawValue, nil
	default:
		return nil, domainerrors.New(
			domainerrors.CodeInternalError,
			"unsupported write-path kind",
			map[string]any{"kind": writeSpec.Kind},
		)
	}
}

func isTypeCompatible(field model.MetaField, rawValue any) bool {
	value := support.NormalizeValue(rawValue)

	switch field.Type {
	case "string":
		_, ok := value.(string)
		return ok
	case "integer":
		number, ok := support.NumberToFloat64(value)
		return ok && number == float64(int(number))
	case "number":
		_, ok := support.NumberToFloat64(value)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	default:
		return false
	}
}

func describeValueType(rawValue any) string {
	value := support.NormalizeValue(rawValue)

	switch typed := value.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case bool:
		return "boolean"
	case []any:
		return "array"
	default:
		if _, ok := support.NumberToFloat64(typed); ok {
			return "number"
		}
		return fmt.Sprintf("%T", value)
	}
}

func isForbiddenWritePath(path string) bool {
	if path == "type" || path == "id" || path == "slug" || path == "createdDate" || path == "updatedDate" {
		return true
	}
	if path == "content" || path == "content.raw" || path == "content.sections" {
		return true
	}
	if strings.HasPrefix(path, "refs.") {
		parts := strings.Split(path, ".")
		if len(parts) >= 3 {
			switch parts[2] {
			case "id", "type", "slug":
				return true
			}
		}
	}
	return false
}

func containsPath(paths []string, target string) bool {
	for _, path := range paths {
		if path == target {
			return true
		}
	}
	return false
}

func cloneMap(input map[string]any) map[string]any {
	next := make(map[string]any, len(input))
	for key, value := range input {
		next[key] = support.DeepCopy(value)
	}
	return next
}

func normalizeScalarOrNil(value any, exists bool) any {
	if !exists {
		return nil
	}
	return support.NormalizeValue(value)
}

func splitRawLines(value string) []string {
	if value == "" {
		return []string{}
	}
	return strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
}

func replaceLines(lines []string, from int, to int, insert []string) []string {
	result := make([]string, 0, len(lines)-max(0, to-from)+len(insert))
	result = append(result, lines[:from]...)
	result = append(result, insert...)
	result = append(result, lines[to:]...)
	return result
}

func removeSectionRange(lines []string, from int, to int) []string {
	next := replaceLines(lines, from, to, []string{})
	return trimBoundaryBlanks(next, from)
}

func trimBoundaryBlanks(lines []string, around int) []string {
	result := slices.Clone(lines)
	if len(result) == 0 {
		return result
	}

	if around > len(result) {
		around = len(result)
	}
	left := around - 1
	right := around
	for left >= 0 && right < len(result) && result[left] == "" && result[right] == "" {
		result = append(result[:right], result[right+1:]...)
	}
	return result
}

func insertMissingSection(
	layout workspace.SectionLayout,
	label string,
	title string,
	body string,
	canonicalOrder []string,
) []string {
	lines := slices.Clone(layout.Lines)
	insertAt := len(lines)
	targetOrderIdx := slices.Index(canonicalOrder, label)

	if targetOrderIdx >= 0 {
		for idx := targetOrderIdx - 1; idx >= 0; idx-- {
			if prev, ok := layout.FirstRange(canonicalOrder[idx]); ok {
				insertAt = prev.EndLine
				break
			}
		}
		if insertAt == len(lines) {
			for idx := targetOrderIdx + 1; idx < len(canonicalOrder); idx++ {
				if next, ok := layout.FirstRange(canonicalOrder[idx]); ok {
					insertAt = next.HeadingLine
					break
				}
			}
		}
	}

	heading := fmt.Sprintf("## %s {#%s}", title, label)
	sectionLines := []string{heading}
	sectionLines = append(sectionLines, splitRawLines(body)...)
	return insertBlockWithSpacing(lines, insertAt, sectionLines)
}

func insertBlockWithSpacing(lines []string, position int, block []string) []string {
	if position < 0 {
		position = 0
	}
	if position > len(lines) {
		position = len(lines)
	}

	prefix := slices.Clone(lines[:position])
	suffix := slices.Clone(lines[position:])

	for len(prefix) > 0 && prefix[len(prefix)-1] == "" {
		prefix = prefix[:len(prefix)-1]
	}
	for len(suffix) > 0 && suffix[0] == "" {
		suffix = suffix[1:]
	}

	result := make([]string, 0, len(prefix)+len(block)+len(suffix)+2)
	result = append(result, prefix...)
	if len(prefix) > 0 {
		result = append(result, "")
	}
	result = append(result, block...)
	if len(suffix) > 0 {
		result = append(result, "")
	}
	result = append(result, suffix...)
	return result
}

func hashOrNil(value string, present bool) any {
	if !present {
		return nil
	}
	return hashString(value)
}

func hashString(value string) string {
	hash := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(hash[:])
}
