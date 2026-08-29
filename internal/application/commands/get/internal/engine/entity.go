package engine

import (
	"fmt"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/get/internal/issuedetails"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/get/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/collections"
	"github.com/anatoly-tenenev/spec-cli/internal/application/entityrefs"
	schemacapread "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/read"
	"github.com/anatoly-tenenev/spec-cli/internal/application/values"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

const (
	getEntityTypeStandardRef     = "5.3"
	getEntityRefStandardRef      = "6"
	getEntitySectionsStandardRef = "13.2"
)

func BuildEntityView(
	target model.ParsedTarget,
	readCapability schemacapread.Capability,
	identityIndex map[string][]model.EntityIdentity,
	plan model.SelectorPlan,
) (map[string]any, *domainerrors.AppError) {
	entityType, exists := readCapability.EntityTypes[target.Type]
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
		requestedFields := buildRequestedRefFields(entityType.RefFields, plan)
		resolvedRefs, refErr := resolveRefs(
			target.Frontmatter,
			identityIndex,
			requestedFields,
		)
		if refErr != nil {
			return nil, refErr
		}
		refs = resolvedRefs
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
		view["createdDate"] = target.CreatedDate
	}
	if target.UpdatedDate != "" {
		view["updatedDate"] = target.UpdatedDate
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

func buildMeta(frontmatter map[string]any, allowedFields map[string]schemacapread.MetaField) map[string]any {
	meta := map[string]any{}
	for _, field := range collections.SortedMapKeys(allowedFields) {
		value, exists := frontmatter[field]
		if !exists {
			continue
		}
		meta[field] = values.DeepCopy(value)
	}
	return meta
}

func resolveRefs(
	frontmatter map[string]any,
	identityIndex map[string][]model.EntityIdentity,
	requestedFields map[string]schemacapread.RefField,
) (map[string]any, *domainerrors.AppError) {
	refs := map[string]any{}
	for _, refField := range collections.SortedMapKeys(requestedFields) {
		refSpec := requestedFields[refField]
		rawTarget, exists := frontmatter[refField]
		if !exists {
			refs[refField] = nil
			continue
		}

		if refSpec.Cardinality == schemacapread.RefCardinalityArray {
			refValue, refErr := resolveArrayRefValue(rawTarget, refSpec, identityIndex, refField)
			if refErr != nil {
				return nil, refErr
			}
			refs[refField] = refValue
			continue
		}

		refValue, refErr := resolveScalarRefValue(rawTarget, refSpec, identityIndex, refField)
		if refErr != nil {
			return nil, refErr
		}
		refs[refField] = refValue
	}
	return refs, nil
}

func resolveScalarRefValue(
	rawTarget any,
	refSpec schemacapread.RefField,
	identityIndex map[string][]model.EntityIdentity,
	refField string,
) (any, *domainerrors.AppError) {
	if rawTarget == nil {
		return nil, nil
	}

	targetID, ok := entityrefs.ReadID(rawTarget)
	if !ok {
		return nil, invalidRefReadError(refField)
	}
	resolved := entityrefs.Classify(targetID, refSpec, identityIndex)
	return entityrefs.ToPublicObject(resolved), nil
}

func resolveArrayRefValue(
	rawTarget any,
	refSpec schemacapread.RefField,
	identityIndex map[string][]model.EntityIdentity,
	refField string,
) (any, *domainerrors.AppError) {
	if rawTarget == nil {
		return nil, nil
	}

	items, ok := rawTarget.([]any)
	if !ok {
		return nil, invalidRefReadError(refField)
	}

	resolvedItems := make([]any, 0, len(items))
	for _, item := range items {
		if item == nil {
			resolvedItems = append(resolvedItems, nil)
			continue
		}
		targetID, ok := entityrefs.ReadID(item)
		if !ok {
			return nil, invalidRefReadError(refField)
		}
		resolved := entityrefs.Classify(targetID, refSpec, identityIndex)
		resolvedItems = append(resolvedItems, entityrefs.ToPublicObject(resolved))
	}
	return resolvedItems, nil
}

func buildRequestedRefFields(refFields map[string]schemacapread.RefField, plan model.SelectorPlan) map[string]schemacapread.RefField {
	requested := map[string]schemacapread.RefField{}
	if plan.RequiresAllRefFields {
		for field, spec := range refFields {
			requested[field] = spec
		}
		return requested
	}
	for field := range plan.RequiredRefFields {
		spec, exists := refFields[field]
		if !exists {
			continue
		}
		requested[field] = spec
	}
	return requested
}

func validateRequestedSections(duplicates map[string]int, plan model.SelectorPlan) *domainerrors.AppError {
	if len(duplicates) == 0 {
		return nil
	}

	if plan.RequiresAllSections {
		for _, label := range collections.SortedMapKeys(duplicates) {
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

func invalidRefReadError(refField string) *domainerrors.AppError {
	return newReadError(
		"failed to compute requested refs field",
		fmt.Sprintf("requested refs field '%s' has invalid value in frontmatter", refField),
		getEntityRefStandardRef,
		map[string]any{"field": refField},
	)
}

func newReadError(message string, issueMessage string, standardRef string, details map[string]any) *domainerrors.AppError {
	issue := issuedetails.ValidationIssue("error", "InstanceError", issueMessage, standardRef)
	return domainerrors.New(domainerrors.CodeReadFailed, message, issuedetails.WithValidationIssues(details, issue))
}
