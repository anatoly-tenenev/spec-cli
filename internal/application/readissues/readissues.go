// Package readissues builds what a read failure says about the document that
// caused it: the clause of the standard it breaks, as one entry under
// error.details.validation.issues.
//
// get and query answer for the same workspace and can hit the same broken
// document - a missing id, frontmatter that no longer parses - so the block
// naming the rule has to be the same block. Building it in one place is what
// keeps a caller from having to know which command reported the failure in
// order to read it.
//
// Reads must keep working on a non-conforming workspace, so a failure here
// describes the document rather than judging it: validate is what judges.
package readissues

import (
	"github.com/anatoly-tenenev/spec-cli/internal/application/values"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

const (
	LevelError         = "error"
	ClassSchemaError   = "SchemaError"
	ClassInstanceError = "InstanceError"
)

// Issue is one entry of the validation block.
func Issue(level string, class string, message string, standardRef string) map[string]any {
	return map[string]any{
		"level":        level,
		"class":        class,
		"message":      message,
		"standard_ref": standardRef,
	}
}

// WithIssues merges a validation block into error details. Empty issues are
// dropped, and details are left untouched when nothing survives, so a caller
// never sees an empty validation block it would have to special-case. Each
// issue is copied, because the details map ends up in a response.
func WithIssues(details map[string]any, issues ...map[string]any) map[string]any {
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
	mergedDetails["validation"] = map[string]any{"issues": validIssues}
	return mergedDetails
}

// NewReadError is the read failure carrying exactly one issue, which is what
// every caller so far needs: the document could not be read, and this is the
// rule it breaks.
func NewReadError(
	message string,
	issueMessage string,
	standardRef string,
	details map[string]any,
) *domainerrors.AppError {
	issue := Issue(LevelError, ClassInstanceError, issueMessage, standardRef)
	return domainerrors.New(
		domainerrors.CodeReadFailed,
		message,
		WithIssues(details, issue),
	)
}
