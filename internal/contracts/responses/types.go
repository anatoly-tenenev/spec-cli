// Package responses defines the output side of the CLI wire contract: the
// envelope every command returns and the result states a caller may observe.
// Changing anything here changes what integrations parse.
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
	Text     string
	ExitCode int
}
