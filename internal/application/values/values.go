// Package values holds the comparison and copying rules for the dynamic values
// decoded out of YAML. Numbers arrive as several Go types and dates as
// time.Time, so equality and output normalization have to be decided in one
// place; commands, the read model and expression evaluation all share these
// rules to keep the same document from comparing differently per layer.
package values

import (
	"fmt"
	"reflect"
	"time"
)

// IsTruthy answers whether a dynamic value counts as true. The rule is
// JMESPath's, not ours: false, an empty list, an empty object, an empty string
// and null are falsy, everything else is truthy.
//
// A schema expression and a query filter both ask it of the same document, so
// an empty array cannot be "has tags" for one and "no tags" for the other.
// Neither layer owns the answer - the JMESPath specification does - which is
// why it lives here rather than in either of them.
//
// The body mirrors the unexported isFalse of go-jmespath. Once the fork exports
// it, this becomes one line delegating to the library and the copy is gone.
func IsTruthy(value any) bool {
	return !isFalse(value)
}

func isFalse(value any) bool {
	switch typed := value.(type) {
	case bool:
		return !typed
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	case string:
		return len(typed) == 0
	case nil:
		return true
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Struct:
		// A struct is never falsy, even when every field holds a zero value.
		return false
	case reflect.Slice, reflect.Map:
		return rv.Len() == 0
	case reflect.Ptr:
		if rv.IsNil() {
			return true
		}
		return isFalse(rv.Elem().Interface())
	}

	return false
}

func DeepCopy(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		copied := make(map[string]any, len(typed))
		for key, item := range typed {
			copied[key] = DeepCopy(item)
		}
		return copied
	case []any:
		copied := make([]any, len(typed))
		for idx := range typed {
			copied[idx] = DeepCopy(typed[idx])
		}
		return copied
	default:
		return typed
	}
}

func NumberToFloat64(value any) (float64, bool) {
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

func LiteralEqual(left any, right any) bool {
	if leftFloat, ok := NumberToFloat64(left); ok {
		if rightFloat, rok := NumberToFloat64(right); rok {
			return leftFloat == rightFloat
		}
	}

	switch leftTyped := left.(type) {
	case string:
		rightTyped, ok := right.(string)
		return ok && leftTyped == rightTyped
	case bool:
		rightTyped, ok := right.(bool)
		return ok && leftTyped == rightTyped
	case nil:
		return right == nil
	case time.Time:
		if rightTime, ok := right.(time.Time); ok {
			return leftTyped.Equal(rightTime)
		}
		if rightString, ok := right.(string); ok {
			return leftTyped.Format("2006-01-02") == rightString
		}
		return false
	default:
		return fmt.Sprintf("%v", left) == fmt.Sprintf("%v", right)
	}
}

// NormalizeMap normalizes every entry of a decoded frontmatter map, and always
// returns a map even when handed none. Read commands parse the same documents
// with the same YAML decoder, so a date or a nested value one of them reports
// as a string cannot be a time.Time for another.
func NormalizeMap(input map[string]any) map[string]any {
	normalized := make(map[string]any, len(input))
	for key, value := range input {
		normalized[key] = NormalizeValue(value)
	}
	return normalized
}

func NormalizeValue(value any) any {
	switch typed := value.(type) {
	case time.Time:
		return typed.Format("2006-01-02")
	case map[string]any:
		normalized := make(map[string]any, len(typed))
		for key, item := range typed {
			normalized[key] = NormalizeValue(item)
		}
		return normalized
	case []any:
		normalized := make([]any, len(typed))
		for idx := range typed {
			normalized[idx] = NormalizeValue(typed[idx])
		}
		return normalized
	default:
		return typed
	}
}
