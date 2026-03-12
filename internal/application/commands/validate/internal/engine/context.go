package engine

import (
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/expressions"
)

type resolvedEntityRef struct {
	ID      string
	Type    string
	Slug    string
	DirPath string
}

type runtimeExpressionContext struct {
	metaValues   map[string]any
	metaPresence map[string]bool
	refs         map[string]resolvedEntityRef
}

func buildRuntimeExpressionContext(
	frontmatter map[string]any,
	resolvedRefs map[string]resolvedEntityRef,
) runtimeExpressionContext {
	metaValues := make(map[string]any, len(frontmatter))
	metaPresence := make(map[string]bool, len(frontmatter))
	for key, value := range frontmatter {
		metaPresence[key] = true
		if dateValue, ok := value.(time.Time); ok {
			metaValues[key] = dateValue.Format("2006-01-02")
			continue
		}
		metaValues[key] = value
	}
	return runtimeExpressionContext{
		metaValues:   metaValues,
		metaPresence: metaPresence,
		refs:         resolvedRefs,
	}
}

func (context runtimeExpressionContext) ResolveReference(reference expressions.Reference) (any, bool) {
	switch reference.Kind {
	case expressions.ReferenceMeta:
		presence, exists := context.metaPresence[reference.Field]
		if !exists || !presence {
			return nil, false
		}

		value, exists := context.metaValues[reference.Field]
		if !exists {
			return nil, false
		}
		return value, true
	case expressions.ReferenceRefs:
		resolved, exists := context.refs[reference.Field]
		if !exists {
			return nil, false
		}

		if reference.Part == "" {
			return true, true
		}

		switch reference.Part {
		case "id":
			return resolved.ID, true
		case "type":
			return resolved.Type, true
		case "slug":
			return resolved.Slug, true
		case "dir_path":
			return resolved.DirPath, true
		default:
			return nil, false
		}
	default:
		return nil, false
	}
}
