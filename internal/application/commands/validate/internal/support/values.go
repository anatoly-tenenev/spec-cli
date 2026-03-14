package support

import (
	"fmt"
	"regexp"
)

var expressionPlaceholderPattern = regexp.MustCompile(`<[^<>]+>`)

func IsSupportedRuleType(value string) bool {
	switch value {
	case "string", "integer", "number", "boolean", "null", "entity_ref", "array":
		return true
	default:
		return false
	}
}

func IsExpressionValue(value any) bool {
	s, ok := value.(string)
	if !ok {
		return false
	}
	return expressionPlaceholderPattern.MatchString(s)
}

func MatchesRuleType(value any, expected string) bool {
	switch expected {
	case "string", "entity_ref":
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
		if LiteralEqual(candidate, actual) {
			return true
		}
	}
	return false
}

func LiteralEqual(left any, right any) bool {
	if lf, lok := numberToFloat64(left); lok {
		if rf, rok := numberToFloat64(right); rok {
			return lf == rf
		}
	}

	switch l := left.(type) {
	case string:
		r, ok := right.(string)
		return ok && l == r
	case bool:
		r, ok := right.(bool)
		return ok && l == r
	case nil:
		return right == nil
	default:
		return fmt.Sprintf("%v", left) == fmt.Sprintf("%v", right)
	}
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

func numberToFloat64(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	default:
		return 0, false
	}
}
