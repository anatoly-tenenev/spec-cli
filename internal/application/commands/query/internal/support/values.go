package support

import (
	"fmt"
)

const (
	ValidationIssueLevelError         = "error"
	ValidationIssueClassSchemaError   = "SchemaError"
	ValidationIssueClassInstanceError = "InstanceError"
)

func LiteralEqual(left any, right any) bool {
	if lf, lok := NumberToFloat64(left); lok {
		if rf, rok := NumberToFloat64(right); rok {
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

func ValidationIssue(level string, class string, message string, standardRef string) map[string]any {
	return map[string]any{
		"level":        level,
		"class":        class,
		"message":      message,
		"standard_ref": standardRef,
	}
}

func WithValidationIssues(details map[string]any, issues ...map[string]any) map[string]any {
	validIssues := make([]map[string]any, 0, len(issues))
	for _, issue := range issues {
		if len(issue) == 0 {
			continue
		}
		validIssues = append(validIssues, DeepCopy(issue).(map[string]any))
	}
	if len(validIssues) == 0 {
		return details
	}

	mergedDetails := map[string]any{}
	for key, value := range details {
		mergedDetails[key] = value
	}

	mergedDetails["validation"] = map[string]any{
		"issues": validIssues,
	}
	return mergedDetails
}
