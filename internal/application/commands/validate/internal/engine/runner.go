package engine

import (
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/workspace"
	schemacapvalidate "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/validate"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

type parsedCandidate struct {
	Candidate       model.WorkspaceCandidate
	RelativePath    string
	Frontmatter     map[string]any
	Sections        map[string]string
	DuplicateLabels []string
	TypeName        string
	HasType         bool
	TypeKnown       bool
	TypeSpec        schemacapvalidate.EntityValidationModel
	ID              string
	HasID           bool
	Slug            string
	HasSlug         bool
	ParseErr        error
}

func RunValidation(
	candidates []model.WorkspaceCandidate,
	schema schemacapvalidate.Capability,
	opts model.Options,
	workspaceRoot string,
) (model.ValidationRun, *domainerrors.AppError) {
	run := model.ValidationRun{
		CandidateEntities:   len(candidates),
		ValidatorConformant: true,
		Issues:              make([]domainvalidation.Issue, 0),
	}

	parsed, parseErr := parseCandidates(candidates, schema, workspaceRoot)
	if parseErr != nil {
		return model.ValidationRun{}, parseErr
	}

	refIndexSource := parsed
	if len(opts.TypeFilters) > 0 {
		referenceCandidates, referenceErr := workspace.BuildCandidateSet(workspaceRoot, nil)
		if referenceErr != nil {
			return model.ValidationRun{}, referenceErr
		}

		referenceParsed, referenceParseErr := parseCandidates(referenceCandidates, schema, workspaceRoot)
		if referenceParseErr != nil {
			return model.ValidationRun{}, referenceParseErr
		}
		refIndexSource = referenceParsed
	}

	idTargetIndex := buildResolvedTargetIndex(refIndexSource)

	checked := make([]model.CheckedEntity, 0, len(candidates))
	ids := map[string][]int{}
	slugsByType := map[string]map[string][]int{}
	suffixByType := map[string]map[int][]int{}
	stopAfterCurrent := false

	for _, candidate := range parsed {
		if stopAfterCurrent {
			break
		}

		entity := model.CheckedEntity{}
		run.CheckedEntities++
		if candidate.HasType {
			entity.Type = candidate.TypeName
		}
		if candidate.HasID {
			entity.ID = candidate.ID
		}
		if candidate.HasSlug {
			entity.Slug = candidate.Slug
		}

		checkedIndex := len(checked)

		if candidate.ParseErr != nil {
			addIssue(&run.Issues, &entity, domainvalidation.Issue{
				Code:        "frontmatter.invalid",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     candidate.ParseErr.Error(),
				StandardRef: "10.2",
			})
			checked = append(checked, entity)
			if opts.FailFast && entity.HasError {
				stopAfterCurrent = true
			}
			continue
		}

		if !candidate.HasType {
			addIssue(&run.Issues, &entity, domainvalidation.Issue{
				Code:        "builtin.type_missing",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     "built-in field 'type' is required",
				StandardRef: "5.3",
				Field:       "frontmatter.type",
			})
		} else if !candidate.TypeKnown {
			addIssue(&run.Issues, &entity, domainvalidation.Issue{
				Code:        "builtin.type_unknown",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     fmt.Sprintf("entity type '%s' is not declared in schema", candidate.TypeName),
				StandardRef: "5.3",
				Field:       "frontmatter.type",
			})
		}

		if !candidate.HasID {
			addIssue(&run.Issues, &entity, domainvalidation.Issue{
				Code:        "builtin.id_missing",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     "built-in field 'id' is required",
				StandardRef: "11.1",
				Field:       "frontmatter.id",
			})
		} else {
			ids[candidate.ID] = append(ids[candidate.ID], checkedIndex)
			if candidate.TypeKnown {
				suffix, hasSuffix := parseIDSuffix(candidate.ID, candidate.TypeSpec.IDPrefix)
				if !hasSuffix {
					addIssue(&run.Issues, &entity, domainvalidation.Issue{
						Code:        "builtin.id_format_invalid",
						Level:       domainvalidation.LevelError,
						Class:       "InstanceError",
						Message:     fmt.Sprintf("id must match '%s-<number>'", candidate.TypeSpec.IDPrefix),
						StandardRef: "7.3",
						Field:       "frontmatter.id",
					})
				} else {
					entity.HasSuffix = true
					entity.IDSuffix = suffix
					if _, exists := suffixByType[candidate.TypeName]; !exists {
						suffixByType[candidate.TypeName] = map[int][]int{}
					}
					suffixByType[candidate.TypeName][suffix] = append(suffixByType[candidate.TypeName][suffix], checkedIndex)
				}
			}
		}

		if !candidate.HasSlug {
			addIssue(&run.Issues, &entity, domainvalidation.Issue{
				Code:        "builtin.slug_missing",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     "built-in field 'slug' is required",
				StandardRef: "11.2",
				Field:       "frontmatter.slug",
			})
		} else {
			if !slugPattern.MatchString(candidate.Slug) {
				addIssue(&run.Issues, &entity, domainvalidation.Issue{
					Code:        "builtin.slug_format_invalid",
					Level:       domainvalidation.LevelError,
					Class:       "InstanceError",
					Message:     "slug must match ^[a-z0-9]+(?:-[a-z0-9]+)*$",
					StandardRef: "11.2",
					Field:       "frontmatter.slug",
				})
			}

			if candidate.TypeKnown {
				if _, exists := slugsByType[candidate.TypeName]; !exists {
					slugsByType[candidate.TypeName] = map[string][]int{}
				}
				slugsByType[candidate.TypeName][candidate.Slug] = append(slugsByType[candidate.TypeName][candidate.Slug], checkedIndex)
			}
		}

		validateDateField(&run.Issues, &entity, candidate.Frontmatter, "createdDate", "11.3")
		validateDateField(&run.Issues, &entity, candidate.Frontmatter, "updatedDate", "11.3")

		if candidate.TypeKnown {
			validateAllowedFrontmatterKeys(&run.Issues, &entity, candidate.Frontmatter, candidate.TypeSpec)

			resolvedRefs := resolveEntityReferences(
				&run.Issues,
				&entity,
				candidate.Frontmatter,
				candidate.TypeSpec,
				idTargetIndex,
			)
			context := buildRuntimeEvaluationContext(candidate.Frontmatter, resolvedRefs, candidate.TypeSpec)

			validateRequiredFields(
				&run.Issues,
				&entity,
				candidate.Frontmatter,
				candidate.TypeSpec,
				idTargetIndex,
				context,
			)
			validateRequiredSections(&run.Issues, &entity, candidate.Sections, candidate.DuplicateLabels, candidate.TypeSpec, context)
			validatePathPattern(&run.Issues, &entity, candidate.RelativePath, candidate.TypeSpec, context)
		} else {
			for _, label := range candidate.DuplicateLabels {
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

	for _, issue := range run.Issues {
		if issue.Class == "ProfileError" {
			run.ValidatorConformant = false
			break
		}
	}

	valid := 0
	for _, entity := range checked {
		if !entity.HasError {
			valid++
		}
	}
	run.EntitiesValid = valid

	return run, nil
}

func parseCandidates(
	candidates []model.WorkspaceCandidate,
	schema schemacapvalidate.Capability,
	workspaceRoot string,
) ([]parsedCandidate, *domainerrors.AppError) {
	parsed := make([]parsedCandidate, 0, len(candidates))

	for _, candidate := range candidates {
		raw, err := os.ReadFile(candidate.Path)
		if err != nil {
			return nil, domainerrors.New(
				domainerrors.CodeWriteFailed,
				"failed to read workspace document",
				nil,
			)
		}

		relativePath, relErr := filepath.Rel(workspaceRoot, candidate.Path)
		if relErr != nil {
			return nil, domainerrors.New(
				domainerrors.CodeWriteFailed,
				"failed to resolve workspace-relative path",
				map[string]any{"reason": relErr.Error()},
			)
		}
		relativePath = filepath.ToSlash(relativePath)

		entry := parsedCandidate{
			Candidate:    candidate,
			RelativePath: relativePath,
		}

		frontmatter, body, parseErr := workspace.ParseFrontmatter(raw)
		if parseErr != nil {
			entry.ParseErr = parseErr
			parsed = append(parsed, entry)
			continue
		}

		entry.Frontmatter = frontmatter
		entry.Sections, entry.DuplicateLabels = workspace.ExtractSectionLabels(body)

		entry.TypeName, entry.HasType = workspace.ReadStringField(frontmatter, "type")
		if entry.HasType {
			typeSpec, exists := schema.EntityTypes[entry.TypeName]
			entry.TypeKnown = exists
			if exists {
				entry.TypeSpec = typeSpec
			}
		}

		entry.ID, entry.HasID = workspace.ReadStringField(frontmatter, "id")
		entry.Slug, entry.HasSlug = workspace.ReadStringField(frontmatter, "slug")

		parsed = append(parsed, entry)
	}

	return parsed, nil
}

func buildResolvedTargetIndex(parsed []parsedCandidate) map[string][]resolvedEntityRef {
	index := map[string][]resolvedEntityRef{}

	for _, entity := range parsed {
		if entity.ParseErr != nil || !entity.HasType || !entity.TypeKnown || !entity.HasID || !entity.HasSlug {
			continue
		}

		dirPath := pathpkg.Dir(normalizeRelativePath(entity.RelativePath))
		if dirPath == "." {
			dirPath = ""
		}

		target := resolvedEntityRef{
			ID:      entity.ID,
			Type:    entity.TypeName,
			Slug:    entity.Slug,
			DirPath: dirPath,
		}
		index[entity.ID] = append(index[entity.ID], target)
	}

	for _, targets := range index {
		sort.Slice(targets, func(i int, j int) bool {
			if targets[i].Type != targets[j].Type {
				return targets[i].Type < targets[j].Type
			}
			if targets[i].Slug != targets[j].Slug {
				return targets[i].Slug < targets[j].Slug
			}
			return targets[i].DirPath < targets[j].DirPath
		})
	}

	return index
}
