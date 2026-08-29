// Package writeops turns one raw --set/--set-file operation into the value that
// will be written, according to the write path the schema declares for it. The
// same operation text means different things on different paths: a meta value
// is parsed as YAML and type-checked, a ref value is an entity id, a section
// value is taken verbatim.
//
// add and update accept the same operations against the same schema, so a value
// one of them writes must be a value the other would write too.
package writeops

import (
	"fmt"
	"os"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/writemodel"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/yamlvalues"
	"github.com/anatoly-tenenev/spec-cli/internal/application/values"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func ResolveValue(
	op writemodel.WriteOperation,
	writeSpec writemodel.WritePathSpec,
	typeSpec writemodel.EntityTypeSpec,
) (any, *domainerrors.AppError) {
	if op.Kind == writemodel.WriteOperationSetFile {
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
	case writemodel.WritePathMeta:
		field := typeSpec.MetaFields[writeSpec.FieldName]
		parsed, parseErr := yamlvalues.ParseYAMLValue(op.RawValue)
		if parseErr != nil {
			return nil, domainerrors.New(
				domainerrors.CodeWriteContractViolation,
				fmt.Sprintf("failed to parse value for path '%s'", op.Path),
				map[string]any{"path": op.Path, "reason": parseErr.Error()},
			)
		}
		if !IsTypeCompatible(field, parsed) {
			return nil, domainerrors.New(
				domainerrors.CodeWriteContractViolation,
				fmt.Sprintf("value for path '%s' does not match schema type '%s'", op.Path, field.Type),
				map[string]any{
					"path":          op.Path,
					"expected_type": field.Type,
					"actual_type":   DescribeValueType(parsed),
				},
			)
		}
		return values.NormalizeValue(parsed), nil
	case writemodel.WritePathRef:
		field := typeSpec.MetaFields[writeSpec.FieldName]
		if field.IsEntityRefArray {
			parsed, parseErr := yamlvalues.ParseYAMLValue(op.RawValue)
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
					map[string]any{"path": op.Path, "expected_type": "array", "actual_type": DescribeValueType(parsed)},
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
	case writemodel.WritePathSection:
		return op.RawValue, nil
	default:
		return nil, domainerrors.New(
			domainerrors.CodeInternalError,
			"unsupported write-path kind",
			map[string]any{"kind": writeSpec.Kind},
		)
	}
}

func IsTypeCompatible(field writemodel.MetaField, rawValue any) bool {
	value := values.NormalizeValue(rawValue)

	switch field.Type {
	case "string":
		_, ok := value.(string)
		return ok
	case "integer":
		number, ok := values.NumberToFloat64(value)
		return ok && number == float64(int(number))
	case "number":
		_, ok := values.NumberToFloat64(value)
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
		return false
	}
}

func DescribeValueType(rawValue any) string {
	value := values.NormalizeValue(rawValue)

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
		if _, ok := values.NumberToFloat64(typed); ok {
			return "number"
		}
		return fmt.Sprintf("%T", value)
	}
}

func IsForbiddenWritePath(path string) bool {
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
