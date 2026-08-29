// Package ruletypes answers whether a frontmatter value matches the type or
// enum the schema declares for it. The type names are the schema's own, not
// Go's, so the mapping from one to the other lives here instead of being
// spelled out at each check.
package ruletypes

import "github.com/anatoly-tenenev/spec-cli/internal/application/values"

func MatchesRuleType(value any, expected string) bool {
	switch expected {
	case "string", "entityRef":
		_, ok := value.(string)
		return ok
	case "integer":
		return isIntegerValue(value)
	case "number":
		return isIntegerValue(value) || isFloatValue(value)
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "null":
		return value == nil
	case "array":
		_, ok := value.([]any)
		return ok
	default:
		return false
	}
}

func ContainsEnumValue(enum []any, actual any) bool {
	for _, candidate := range enum {
		if values.LiteralEqual(candidate, actual) {
			return true
		}
	}
	return false
}

func isIntegerValue(value any) bool {
	switch value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	default:
		return false
	}
}

func isFloatValue(value any) bool {
	switch value.(type) {
	case float32, float64:
		return true
	default:
		return false
	}
}
