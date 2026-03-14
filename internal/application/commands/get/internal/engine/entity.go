package engine

import (
	"fmt"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/get/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/get/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

const (
	getEntityTypeStandardRef     = "5.3"
	getEntityRefStandardRef      = "6"
	getEntitySectionsStandardRef = "8.3"
)

func BuildEntityView(
	target model.ParsedTarget,
	readModel model.ReadModel,
	identityIndex map[string][]model.EntityIdentity,
	plan model.SelectorPlan,
) (map[string]any, *domainerrors.AppError) {
	entityType, exists := readModel.EntityTypes[target.Type]
	if !exists {
		return nil, newReadError(
			"failed to determine entity type",
			fmt.Sprintf("entity type '%s' is not declared in schema.entity", target.Type),
			getEntityTypeStandardRef,
			nil,
		)
	}

	meta := buildMeta(target.Frontmatter, entityType.MetaFields)
	refs := map[string]any{}
	if plan.RequiresRefs {
		resolvedRefs := resolveRefs(meta, entityType.RefFields, identityIndex)
		for key, value := range resolvedRefs {
			refs[key] = value
		}
		for field := range plan.RequiredRefFields {
			if _, ok := refs[field]; ok {
				continue
			}
			return nil, newReadError(
				"failed to compute requested refs field",
				fmt.Sprintf("requested refs field '%s' cannot be computed deterministically", field),
				getEntityRefStandardRef,
				map[string]any{"field": field},
			)
		}
	}

	if plan.RequiresSections {
		if sectionErr := validateRequestedSections(target.DuplicateSectionLabels, plan); sectionErr != nil {
			return nil, sectionErr
		}
	}

	view := map[string]any{
		"type":     target.Type,
		"id":       target.ID,
		"revision": target.Revision,
		"meta":     meta,
	}

	if target.Slug != "" {
		view["slug"] = target.Slug
	}
	if target.CreatedDate != "" {
		view["created_date"] = target.CreatedDate
	}
	if target.UpdatedDate != "" {
		view["updated_date"] = target.UpdatedDate
	}
	if plan.RequiresRefs {
		view["refs"] = refs
	}
	if plan.RequiresContent {
		content := map[string]any{}
		if plan.RequiresContentRaw {
			content["raw"] = target.RawBody
		}
		if plan.RequiresSections {
			content["sections"] = sectionsToAnyMap(target.Sections)
		}
		view["content"] = content
	}

	return view, nil
}

func buildMeta(frontmatter map[string]any, allowedFields map[string]struct{}) map[string]any {
	meta := map[string]any{}
	for _, field := range support.SortedMapKeys(allowedFields) {
		value, exists := frontmatter[field]
		if !exists {
			continue
		}
		meta[field] = support.DeepCopy(value)
	}
	return meta
}

func resolveRefs(
	meta map[string]any,
	refFields map[string]struct{},
	identityIndex map[string][]model.EntityIdentity,
) map[string]any {
	refs := map[string]any{}
	for _, refField := range support.SortedMapKeys(refFields) {
		rawTarget, exists := meta[refField]
		if !exists {
			continue
		}
		targetID, ok := rawTarget.(string)
		if !ok {
			continue
		}
		targetID = strings.TrimSpace(targetID)
		if targetID == "" {
			continue
		}

		targets := identityIndex[targetID]
		if len(targets) != 1 {
			continue
		}

		target := targets[0]
		refs[refField] = map[string]any{
			"type": target.Type,
			"id":   target.ID,
			"slug": target.Slug,
		}
	}
	return refs
}

func validateRequestedSections(duplicates map[string]int, plan model.SelectorPlan) *domainerrors.AppError {
	if len(duplicates) == 0 {
		return nil
	}

	if plan.RequiresAllSections {
		for _, label := range support.SortedMapKeys(duplicates) {
			return newReadError(
				"failed to compute requested content sections",
				fmt.Sprintf("section label '%s' is duplicated", label),
				getEntitySectionsStandardRef,
				map[string]any{"section": label},
			)
		}
	}

	for section := range plan.RequiredSectionNames {
		if duplicates[section] <= 1 {
			continue
		}
		return newReadError(
			"failed to compute requested content sections",
			fmt.Sprintf("section label '%s' is duplicated", section),
			getEntitySectionsStandardRef,
			map[string]any{"section": section},
		)
	}

	return nil
}

func sectionsToAnyMap(sections map[string]string) map[string]any {
	mapped := make(map[string]any, len(sections))
	for key, value := range sections {
		mapped[key] = value
	}
	return mapped
}

func newReadError(message string, issueMessage string, standardRef string, details map[string]any) *domainerrors.AppError {
	issue := support.ValidationIssue("error", "InstanceError", issueMessage, standardRef)
	return domainerrors.New(domainerrors.CodeReadFailed, message, support.WithValidationIssues(details, issue))
}
