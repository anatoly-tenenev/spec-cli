package issues

import (
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/model"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

func New(code string, message string, standardRef string, field string, candidate *model.Candidate) domainvalidation.Issue {
	item := domainvalidation.Issue{
		Code:        code,
		Level:       domainvalidation.LevelError,
		Class:       "InstanceError",
		Message:     message,
		StandardRef: standardRef,
		Field:       field,
	}
	if candidate != nil {
		item.Entity = &domainvalidation.Entity{
			Type: candidate.Type,
			ID:   candidate.ID,
			Slug: candidate.Slug,
		}
	}
	return item
}
