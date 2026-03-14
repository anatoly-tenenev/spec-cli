package support

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
	mergedDetails["validation"] = map[string]any{"issues": validIssues}
	return mergedDetails
}
