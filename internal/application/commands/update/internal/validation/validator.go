// Package validation checks the whole updated entity, not just the fields the
// patch touched. The checks themselves are the ones add runs too and live in
// commands/internal/entitycheck; what this package adds is update's view of
// the workspace - the document being edited already holds its own id and slug,
// so it is not competition - and the error a rejected update is reported as.
//
// Checking everything is what keeps an update from completing a document that
// was already non-conforming into a state no one validated.
package validation

import (
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/entitycheck"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

func Validate(
	typeSpec model.EntityTypeSpec,
	candidate *model.Candidate,
	snapshot model.Snapshot,
	sourcePath string,
	pathIssues []domainvalidation.Issue,
	refIssues []domainvalidation.Issue,
	evaluationContext map[string]any,
) []domainvalidation.Issue {
	return entitycheck.Check(
		typeSpec,
		candidate,
		entitycheck.Workspace{
			EntitiesByID: snapshot.EntitiesByID,
			SlugsByType:  snapshot.SlugsByType,
			OwnPath:      sourcePath,
		},
		pathIssues,
		refIssues,
		evaluationContext,
	)
}

func AsAppError(issuesList []domainvalidation.Issue) *domainerrors.AppError {
	return domainerrors.New(
		domainerrors.CodeValidationFailed,
		"updated entity failed validation",
		map[string]any{
			"validation": map[string]any{
				"issues": issuesList,
			},
		},
	)
}
