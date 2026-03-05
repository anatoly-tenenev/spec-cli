package engine

import (
	"fmt"
	"os"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/workspace"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

func RunValidation(candidates []model.WorkspaceCandidate, schema model.ValidationSchema, opts model.Options) (model.ValidationRun, *domainerrors.AppError) {
	run := model.ValidationRun{
		CandidateEntities: len(candidates),
		Issues:            make([]domainvalidation.Issue, 0),
	}

	checked := make([]model.CheckedEntity, 0, len(candidates))
	ids := map[string][]int{}
	slugsByType := map[string]map[string][]int{}
	suffixByType := map[string]map[int][]int{}

	stopAfterCurrent := false

	for _, candidate := range candidates {
		if stopAfterCurrent {
			break
		}

		entity := model.CheckedEntity{}
		run.CheckedEntities++

		raw, err := os.ReadFile(candidate.Path)
		if err != nil {
			return model.ValidationRun{}, domainerrors.New(
				domainerrors.CodeWriteFailed,
				"failed to read workspace document",
				map[string]any{"reason": err.Error()},
			)
		}

		frontmatter, body, parseErr := workspace.ParseFrontmatter(raw)
		if parseErr != nil {
			addIssue(&run.Issues, &entity, domainvalidation.Issue{
				Code:        "frontmatter.invalid",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     parseErr.Error(),
				StandardRef: "10.2",
			})
			checked = append(checked, entity)
			if opts.FailFast && entity.HasError {
				stopAfterCurrent = true
			}
			continue
		}

		sections, duplicateLabels := workspace.ExtractSectionLabels(body)

		typeName, hasType := workspace.ReadStringField(frontmatter, "type")
		if !hasType {
			addIssue(&run.Issues, &entity, domainvalidation.Issue{
				Code:        "builtin.type_missing",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     "built-in field 'type' is required",
				StandardRef: "5.3",
				Field:       "frontmatter.type",
			})
		} else {
			entity.Type = typeName
		}

		typeSpec, typeKnown := schema.Entity[typeName]
		if hasType && !typeKnown {
			addIssue(&run.Issues, &entity, domainvalidation.Issue{
				Code:        "builtin.type_unknown",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     fmt.Sprintf("entity type '%s' is not declared in schema", typeName),
				StandardRef: "5.3",
				Field:       "frontmatter.type",
			})
		}

		id, hasID := workspace.ReadStringField(frontmatter, "id")
		if !hasID {
			addIssue(&run.Issues, &entity, domainvalidation.Issue{
				Code:        "builtin.id_missing",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     "built-in field 'id' is required",
				StandardRef: "11.1",
				Field:       "frontmatter.id",
			})
		} else {
			entity.ID = id
			ids[id] = append(ids[id], len(checked))
		}

		if hasID && typeKnown {
			suffix, hasSuffix := parseIDSuffix(id, typeSpec.IDPrefix)
			if !hasSuffix {
				addIssue(&run.Issues, &entity, domainvalidation.Issue{
					Code:        "builtin.id_format_invalid",
					Level:       domainvalidation.LevelError,
					Class:       "InstanceError",
					Message:     fmt.Sprintf("id must match '%s-<number>'", typeSpec.IDPrefix),
					StandardRef: "7.3",
					Field:       "frontmatter.id",
				})
			} else {
				entity.HasSuffix = true
				entity.IDSuffix = suffix
				if _, exists := suffixByType[typeName]; !exists {
					suffixByType[typeName] = map[int][]int{}
				}
				suffixByType[typeName][suffix] = append(suffixByType[typeName][suffix], len(checked))
			}
		}

		slug, hasSlug := workspace.ReadStringField(frontmatter, "slug")
		if !hasSlug {
			addIssue(&run.Issues, &entity, domainvalidation.Issue{
				Code:        "builtin.slug_missing",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     "built-in field 'slug' is required",
				StandardRef: "11.2",
				Field:       "frontmatter.slug",
			})
		} else {
			entity.Slug = slug
			if !slugPattern.MatchString(slug) {
				addIssue(&run.Issues, &entity, domainvalidation.Issue{
					Code:        "builtin.slug_format_invalid",
					Level:       domainvalidation.LevelError,
					Class:       "InstanceError",
					Message:     "slug must match ^[a-z0-9]+(?:-[a-z0-9]+)*$",
					StandardRef: "11.2",
					Field:       "frontmatter.slug",
				})
			}

			if typeKnown {
				if _, exists := slugsByType[typeName]; !exists {
					slugsByType[typeName] = map[string][]int{}
				}
				slugsByType[typeName][slug] = append(slugsByType[typeName][slug], len(checked))
			}
		}

		validateDateField(&run.Issues, &entity, frontmatter, "created_date", "11.3")
		validateDateField(&run.Issues, &entity, frontmatter, "updated_date", "11.3")

		if typeKnown {
			validateAllowedFrontmatterKeys(&run.Issues, &entity, frontmatter, typeSpec)
			validateRequiredFields(&run.Issues, &entity, frontmatter, typeSpec)
			validateRequiredSections(&run.Issues, &entity, sections, duplicateLabels, typeSpec)
		} else {
			for _, label := range duplicateLabels {
				addIssue(&run.Issues, &entity, domainvalidation.Issue{
					Code:        "content.section_label_duplicate",
					Level:       domainvalidation.LevelError,
					Class:       "InstanceError",
					Message:     fmt.Sprintf("section label '%s' is duplicated", label),
					StandardRef: "13.2",
				})
			}
		}

		checked = append(checked, entity)
		if opts.FailFast && entity.HasError {
			stopAfterCurrent = true
		}
	}

	appendGlobalUniquenessIssues(&run.Issues, checked, ids, slugsByType, suffixByType)

	valid := 0
	for _, entity := range checked {
		if !entity.HasError {
			valid++
		}
	}
	run.EntitiesValid = valid

	return run, nil
}
