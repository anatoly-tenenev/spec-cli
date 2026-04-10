package writes

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

type Applied struct {
	FrontmatterValues map[string]any
	MetaPayload       map[string]any
	RefIDs            map[string]string
	RefIDArrays       map[string][]string
	SectionBodies     map[string]string
	WholeBody         string
	WholeBodyProvided bool
}

func Apply(opts model.Options, typeSpec model.EntityTypeSpec) (Applied, *domainerrors.AppError) {
	applied := Applied{
		FrontmatterValues: map[string]any{},
		MetaPayload:       map[string]any{},
		RefIDs:            map[string]string{},
		RefIDArrays:       map[string][]string{},
		SectionBodies:     map[string]string{},
	}

	for _, op := range opts.Operations {
		writeSpec, exists := typeSpec.AllowWritePaths[op.Path]
		if !exists {
			if isForbiddenWritePath(op.Path) {
				return Applied{}, domainerrors.New(
					domainerrors.CodeWriteContractViolation,
					fmt.Sprintf("write path '%s' is forbidden by write contract", op.Path),
					map[string]any{"path": op.Path},
				)
			}
			return Applied{}, domainerrors.New(
				domainerrors.CodeWriteContractViolation,
				fmt.Sprintf("write path '%s' is not allowed", op.Path),
				map[string]any{"path": op.Path},
			)
		}

		if op.Kind == model.WriteOperationSetFile {
			if _, ok := typeSpec.AllowSetFilePaths[op.Path]; !ok {
				return Applied{}, domainerrors.New(
					domainerrors.CodeWriteContractViolation,
					fmt.Sprintf("--set-file is not allowed for path '%s'", op.Path),
					map[string]any{"path": op.Path},
				)
			}
		}

		value, valueErr := resolveOperationValue(op, writeSpec, typeSpec)
		if valueErr != nil {
			return Applied{}, valueErr
		}

		switch writeSpec.Kind {
		case model.WritePathMeta:
			field := typeSpec.MetaFields[writeSpec.FieldName]
			applied.FrontmatterValues[field.Name] = value
			if field.IsEntityRef {
				idValue := strings.TrimSpace(value.(string))
				if idValue != "" {
					applied.RefIDs[field.Name] = idValue
				}
				continue
			}
			applied.MetaPayload[field.Name] = support.NormalizeValue(value)
		case model.WritePathRef:
			field := typeSpec.MetaFields[writeSpec.FieldName]
			if field.IsEntityRefArray {
				refIDs := extractRefIDArray(value.([]any))
				applied.FrontmatterValues[writeSpec.FieldName] = value
				applied.RefIDArrays[writeSpec.FieldName] = refIDs
			} else {
				idValue := strings.TrimSpace(value.(string))
				applied.FrontmatterValues[writeSpec.FieldName] = idValue
				if idValue != "" {
					applied.RefIDs[writeSpec.FieldName] = idValue
				}
			}
		case model.WritePathSection:
			applied.SectionBodies[writeSpec.FieldName] = value.(string)
		default:
			return Applied{}, domainerrors.New(
				domainerrors.CodeInternalError,
				"unsupported write-path kind",
				map[string]any{"kind": writeSpec.Kind},
			)
		}
	}

	if opts.ContentFile != "" {
		if !typeSpec.HasContent {
			return Applied{}, domainerrors.New(
				domainerrors.CodeWriteContractViolation,
				"whole-body input is not allowed for entity type without content",
				nil,
			)
		}
		raw, err := os.ReadFile(opts.ContentFile)
		if err != nil {
			return Applied{}, domainerrors.New(
				domainerrors.CodeWriteFailed,
				"failed to read --content-file",
				map[string]any{"reason": err.Error()},
			)
		}
		applied.WholeBody = string(raw)
		applied.WholeBodyProvided = true
	}

	if opts.ContentStdin {
		if !typeSpec.HasContent {
			return Applied{}, domainerrors.New(
				domainerrors.CodeWriteContractViolation,
				"whole-body input is not allowed for entity type without content",
				nil,
			)
		}
		raw, err := io.ReadAll(os.Stdin)
		if err != nil {
			return Applied{}, domainerrors.New(
				domainerrors.CodeWriteFailed,
				"failed to read --content-stdin",
				map[string]any{"reason": err.Error()},
			)
		}
		applied.WholeBody = string(raw)
		applied.WholeBodyProvided = true
	}

	return applied, nil
}

func BuildBody(typeSpec model.EntityTypeSpec, applied Applied) string {
	if applied.WholeBodyProvided {
		return applied.WholeBody
	}

	if len(applied.SectionBodies) == 0 {
		return ""
	}

	parts := make([]string, 0, len(applied.SectionBodies))
	for _, sectionName := range typeSpec.SectionOrder {
		body, exists := applied.SectionBodies[sectionName]
		if !exists {
			continue
		}

		title := sectionName
		if strings.TrimSpace(typeSpec.Sections[sectionName].Title) != "" {
			title = typeSpec.Sections[sectionName].Title
		}

		heading := fmt.Sprintf("## %s {#%s}", title, sectionName)
		if body == "" {
			parts = append(parts, heading)
			continue
		}
		parts = append(parts, heading+"\n"+body)
	}
	return strings.Join(parts, "\n\n")
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
		if field.IsEntityRef {
			value := strings.TrimSpace(op.RawValue)
			if value == "" {
				return nil, domainerrors.New(
					domainerrors.CodeWriteContractViolation,
					fmt.Sprintf("path '%s' requires non-empty target entity id", op.Path),
					map[string]any{"path": op.Path},
				)
			}
			return value, nil
		}
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
	case "entityRef":
		text, ok := value.(string)
		return ok && strings.TrimSpace(text) != ""
	default:
		return true
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

func extractRefIDArray(items []any) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		itemText, ok := item.(string)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(itemText)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}
