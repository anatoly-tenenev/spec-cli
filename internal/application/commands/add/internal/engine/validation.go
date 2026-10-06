// validation.go judges the finished candidate. The checks themselves are the
// ones update runs too and live in commands/internal/entitycheck; what is here
// is add's view of the workspace - a new document competes with every existing
// one for its id and slug - and the error a rejected add is reported as.

package engine

import (
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/entitycheck"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

func validateCandidate(
	typeSpec model.EntityTypeSpec,
	candidate *model.Candidate,
	snapshot model.Snapshot,
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
		},
		pathIssues,
		refIssues,
		evaluationContext,
	)
}

func validationFailedError(issuesList []domainvalidation.Issue) *domainerrors.AppError {
	return domainerrors.New(
		domainerrors.CodeValidationFailed,
		"created entity failed validation",
		map[string]any{
			"validation": map[string]any{
				"issues": issuesList,
			},
		},
	)
}
