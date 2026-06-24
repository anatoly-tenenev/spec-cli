package diagnostics

import "github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/internal/values"

const (
	ValidationIssueLevelError         = "error"
	ValidationIssueClassSchemaError   = "SchemaError"
	ValidationIssueClassInstanceError = "InstanceError"
)

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
		validIssues = append(validIssues, values.DeepCopy(issue).(map[string]any))
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
