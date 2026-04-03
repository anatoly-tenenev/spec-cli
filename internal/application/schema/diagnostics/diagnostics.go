package diagnostics

type Level string

const (
	LevelError   Level = "error"
	LevelWarning Level = "warning"
)

const (
	ClassSchemaError   = "SchemaError"
	ClassSchemaWarning = "SchemaWarning"
)

type Issue struct {
	Level   Level  `json:"level"`
	Class   string `json:"class"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

type Summary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
}

func NewError(code string, message string, path string) Issue {
	return Issue{
		Level:   LevelError,
		Class:   ClassSchemaError,
		Code:    code,
		Message: message,
		Path:    path,
	}
}

func NewWarning(code string, message string, path string) Issue {
	return Issue{
		Level:   LevelWarning,
		Class:   ClassSchemaWarning,
		Code:    code,
		Message: message,
		Path:    path,
	}
}

func Summarize(issues []Issue) Summary {
	summary := Summary{}
	for _, issue := range issues {
		switch issue.Level {
		case LevelError:
			summary.Errors++
		case LevelWarning:
			summary.Warnings++
		}
	}
	return summary
}

func HasErrors(issues []Issue) bool {
	for _, issue := range issues {
		if issue.Level == LevelError {
			return true
		}
	}
	return false
}
