package engine

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/support"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/workspace"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func validateAllowedFrontmatterKeys(issues *[]domainvalidation.Issue, entity *model.CheckedEntity, frontmatter map[string]any, typeSpec model.SchemaEntityType) {
	allowed := map[string]struct{}{
		"type":        {},
		"id":          {},
		"slug":        {},
		"createdDate": {},
		"updatedDate": {},
	}
	for _, rule := range typeSpec.RequiredFields {
		allowed[rule.Name] = struct{}{}
	}

	keys := support.SortedMapKeys(frontmatter)
	for _, key := range keys {
		if _, ok := allowed[key]; ok {
			continue
		}
		addIssue(issues, entity, domainvalidation.Issue{
			Code:        "frontmatter.field_not_allowed",
			Level:       domainvalidation.LevelError,
			Class:       "InstanceError",
			Message:     fmt.Sprintf("field '%s' is not allowed by schema", key),
			StandardRef: "12.3",
			Field:       fmt.Sprintf("frontmatter.%s", key),
		})
	}
}

func appendGlobalUniquenessIssues(
	issues *[]domainvalidation.Issue,
	checked []model.CheckedEntity,
	ids map[string][]int,
	slugsByType map[string]map[string][]int,
	suffixByType map[string]map[int][]int,
) {
	duplicatedIDs := support.DuplicatedStringKeys(ids)
	for _, id := range duplicatedIDs {
		for _, idx := range ids[id] {
			checked[idx].HasError = true
		}
		*issues = append(*issues, domainvalidation.Issue{
			Code:        "global.id_duplicate",
			Level:       domainvalidation.LevelError,
			Class:       "InstanceError",
			Message:     fmt.Sprintf("id '%s' is duplicated", id),
			StandardRef: "11.1",
			Field:       "frontmatter.id",
		})
	}

	typeNames := support.SortedMapKeys(slugsByType)
	for _, typeName := range typeNames {
		duplicatedSlugs := support.DuplicatedStringKeys(slugsByType[typeName])
		for _, slug := range duplicatedSlugs {
			for _, idx := range slugsByType[typeName][slug] {
				checked[idx].HasError = true
			}
			*issues = append(*issues, domainvalidation.Issue{
				Code:        "global.slug_duplicate_by_type",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     fmt.Sprintf("slug '%s' is duplicated for type '%s'", slug, typeName),
				StandardRef: "11.2",
				Field:       "frontmatter.slug",
			})
		}
	}

	typeNames = support.SortedMapKeys(suffixByType)
	for _, typeName := range typeNames {
		suffixes := support.DuplicatedIntKeys(suffixByType[typeName])
		for _, suffix := range suffixes {
			for _, idx := range suffixByType[typeName][suffix] {
				checked[idx].HasError = true
			}
			*issues = append(*issues, domainvalidation.Issue{
				Code:        "global.id_suffix_duplicate_by_type",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     fmt.Sprintf("id numeric suffix '%d' is duplicated for type '%s'", suffix, typeName),
				StandardRef: "7.3",
				Field:       "frontmatter.id",
			})
		}
	}
}

func validateDateField(issues *[]domainvalidation.Issue, entity *model.CheckedEntity, frontmatter map[string]any, field string, standardRef string) {
	dateValue, exists := workspace.ReadStringField(frontmatter, field)
	if !exists {
		addIssue(issues, entity, domainvalidation.Issue{
			Code:        "builtin.date_missing",
			Level:       domainvalidation.LevelError,
			Class:       "InstanceError",
			Message:     fmt.Sprintf("built-in field '%s' is required", field),
			StandardRef: standardRef,
			Field:       fmt.Sprintf("frontmatter.%s", field),
		})
		return
	}

	if _, err := time.Parse("2006-01-02", dateValue); err != nil {
		addIssue(issues, entity, domainvalidation.Issue{
			Code:        "builtin.date_format_invalid",
			Level:       domainvalidation.LevelError,
			Class:       "InstanceError",
			Message:     fmt.Sprintf("field '%s' must be in YYYY-MM-DD format", field),
			StandardRef: standardRef,
			Field:       fmt.Sprintf("frontmatter.%s", field),
		})
	}
}

func parseIDSuffix(id string, prefix string) (int, bool) {
	expectedPrefix := prefix + "-"
	if !strings.HasPrefix(id, expectedPrefix) {
		return 0, false
	}

	rawSuffix := strings.TrimPrefix(id, expectedPrefix)
	if rawSuffix == "" {
		return 0, false
	}
	for _, ch := range rawSuffix {
		if ch < '0' || ch > '9' {
			return 0, false
		}
	}

	suffix, err := strconv.Atoi(rawSuffix)
	if err != nil || suffix < 0 {
		return 0, false
	}
	return suffix, true
}
