// Package diagnostics builds the read failures raised while loading a
// workspace, each carrying the clause of the standard the document breaks.
// Naming the clause is what turns "failed to read" into something the caller
// can act on, and holding the references here keeps one wording per rule.
package diagnostics

import (
	readmodeldiagnostics "github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/internal/diagnostics"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

const (
	FrontmatterStandardRef = "11"
	TypeStandardRef        = "5.3"
	IDStandardRef          = "11.1"
	SlugStandardRef        = "11.2"
	RefsStandardRef        = "6"
)

func NewReadError(message string, issueMessage string, standardRef string, details map[string]any) *domainerrors.AppError {
	issue := readmodeldiagnostics.ValidationIssue(
		readmodeldiagnostics.ValidationIssueLevelError,
		readmodeldiagnostics.ValidationIssueClassInstanceError,
		issueMessage,
		standardRef,
	)
	return domainerrors.New(
		domainerrors.CodeReadFailed,
		message,
		readmodeldiagnostics.WithValidationIssues(details, issue),
	)
}
