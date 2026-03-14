package support

import (
	"fmt"
	"time"
)

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

func NormalizeScalar(value any) any {
	switch typed := value.(type) {
	case time.Time:
		return typed.Format("2006-01-02")
	default:
		return value
	}
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

func ValidationIssue(code string, message string, standardRef string, field string) map[string]any {
	issue := map[string]any{
		"code":         code,
		"level":        "error",
		"class":        "InstanceError",
		"message":      message,
		"standard_ref": standardRef,
	}
	if field != "" {
		issue["field"] = field
	}
	return issue
}

func WithValidationIssues(details map[string]any, issues ...map[string]any) map[string]any {
	if len(issues) == 0 {
		return details
	}

	filtered := make([]map[string]any, 0, len(issues))
	for _, issue := range issues {
		if len(issue) == 0 {
			continue
		}
		filtered = append(filtered, DeepCopy(issue).(map[string]any))
	}
	if len(filtered) == 0 {
		return details
	}

	merged := map[string]any{}
	for key, value := range details {
		merged[key] = value
	}
	merged["validation"] = map[string]any{"issues": filtered}
	return merged
}
