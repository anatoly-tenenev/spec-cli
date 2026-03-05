package engine

import (
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

func addIssue(issues *[]domainvalidation.Issue, entity *model.CheckedEntity, issue domainvalidation.Issue) {
	if issue.Entity == nil {
		if snapshot := snapshotEntity(*entity); snapshot != nil {
			issue.Entity = snapshot
		}
	}

	*issues = append(*issues, issue)
	if issue.Level == domainvalidation.LevelError {
		entity.HasError = true
	}
}

func snapshotEntity(entity model.CheckedEntity) *domainvalidation.Entity {
	if entity.Type == "" && entity.ID == "" && entity.Slug == "" {
		return nil
	}

	return &domainvalidation.Entity{
		Type: entity.Type,
		ID:   entity.ID,
		Slug: entity.Slug,
	}
}

func CountIssuesByLevel(issues []domainvalidation.Issue) (errors int, warnings int) {
	for _, issue := range issues {
		switch issue.Level {
		case domainvalidation.LevelWarning:
			warnings++
		default:
			errors++
		}
	}
	return errors, warnings
}
