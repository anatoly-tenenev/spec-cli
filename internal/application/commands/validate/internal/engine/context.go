package engine

import (
	"time"

	schemacapvalidate "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/validate"
	"github.com/anatoly-tenenev/spec-cli/internal/domain/reservedkeys"
)

type resolvedEntityRef struct {
	ID      string
	Type    string
	Slug    string
	DirPath string
}

func buildRuntimeEvaluationContext(
	frontmatter map[string]any,
	resolvedRefs map[string]resolvedEntityRef,
	typeSpec schemacapvalidate.EntityValidationModel,
) map[string]any {
	meta := make(map[string]any, len(typeSpec.RequiredFields))
	refs := make(map[string]any)
	topLevel := map[string]any{}

	builtinKeys := []string{
		reservedkeys.BuiltinType,
		reservedkeys.BuiltinID,
		reservedkeys.BuiltinSlug,
		reservedkeys.BuiltinCreatedDate,
		reservedkeys.BuiltinUpdatedDate,
	}
	for _, key := range builtinKeys {
		value, exists := frontmatter[key]
		if !exists {
			topLevel[key] = nil
			continue
		}
		topLevel[key] = normalizeContextValue(value)
	}

	for _, fieldRule := range typeSpec.RequiredFields {
		if expressionContextBuiltinField(fieldRule.Name) {
			continue
		}

		if fieldRule.Type != reservedkeys.SchemaTypeEntityRef {
			if value, exists := frontmatter[fieldRule.Name]; exists {
				meta[fieldRule.Name] = normalizeContextValue(value)
			}
			continue
		}
		refs[fieldRule.Name] = nil
	}

	for fieldName, resolved := range resolvedRefs {
		if _, exists := refs[fieldName]; !exists {
			continue
		}
		refs[fieldName] = map[string]any{
			"id":      resolved.ID,
			"type":    resolved.Type,
			"slug":    resolved.Slug,
			"dirPath": resolved.DirPath,
		}
	}

	topLevel["meta"] = meta
	topLevel["refs"] = refs
	return topLevel
}

func expressionContextBuiltinField(fieldName string) bool {
	switch fieldName {
	case reservedkeys.BuiltinType,
		reservedkeys.BuiltinID,
		reservedkeys.BuiltinSlug,
		reservedkeys.BuiltinCreatedDate,
		reservedkeys.BuiltinUpdatedDate:
		return true
	default:
		return false
	}
}

func normalizeContextValue(value any) any {
	if dateValue, ok := value.(time.Time); ok {
		return dateValue.Format("2006-01-02")
	}
	return value
}
