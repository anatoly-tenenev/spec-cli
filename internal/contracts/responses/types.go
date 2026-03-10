package responses

type ResultState string

const (
	ResultStateValid          ResultState = "valid"
	ResultStateInvalid        ResultState = "invalid"
	ResultStatePartiallyValid ResultState = "partially_valid"
	ResultStateNotFound       ResultState = "not_found"
	ResultStateUnsupported    ResultState = "unsupported"
	ResultStateIndeterminate  ResultState = "indeterminate"
)

type CommandOutput struct {
	JSON     map[string]any
	ExitCode int
}
