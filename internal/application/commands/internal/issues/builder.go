// Package issues builds the instance-level validation issues reported by the
// write commands. It fixes the fields those issues always share - error level,
// InstanceError class, the entity taken from the candidate - so a caller only
// supplies what actually differs between checks.
package issues

import (
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/writemodel"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

func New(code string, message string, standardRef string, field string, candidate *writemodel.Candidate) domainvalidation.Issue {
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

// PathOrDefault names the schema location an issue points at. A compiled
// schema leaves the path blank where the construct is implicit, and a blank
// `field` would leave the caller with nothing to open, so each check supplies
// the location it would have been written at.
func PathOrDefault(path string, fallback string) string {
	if strings.TrimSpace(path) != "" {
		return path
	}
	return fallback
}
