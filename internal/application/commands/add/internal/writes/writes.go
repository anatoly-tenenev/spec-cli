// Package writes turns the requested --set operations into the frontmatter,
// reference ids and section bodies of the new entity. Every path is looked up
// in the schema's write contract first: a path the schema does not declare
// writable, or one the standard reserves, is refused rather than written
// through.
//
// Raw text is converted according to the field's declared type, so a value
// reaches validation as the type the schema says it is.
package writes

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/writeops"
	"github.com/anatoly-tenenev/spec-cli/internal/application/iofailure"
	"github.com/anatoly-tenenev/spec-cli/internal/application/values"
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
			if writeops.IsForbiddenWritePath(op.Path) {
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

		value, valueErr := writeops.ResolveValue(op, writeSpec, typeSpec)
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
			applied.MetaPayload[field.Name] = values.NormalizeValue(value)
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
				iofailure.Details(err),
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
