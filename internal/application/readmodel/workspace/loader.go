// Package workspace loads a whole workspace into the entity views the read
// engine runs on: scan, parse every document, index ids, resolve references,
// then project the views. It is the read side's only door to the filesystem,
// so what a query can see is decided in one place.
//
// Whole-workspace steps come before the type filter is applied, because a
// reference may point at an entity of a type the query did not ask for.
//
// loader.go runs the pipeline; documents.go parses one file, views.go narrows
// a document to what the schema declares, references.go resolves its entityRef
// fields. standardrefs.go names the clauses of the standard a failure cites.
package workspace

import (
	"fmt"

	"github.com/anatoly-tenenev/spec-cli/internal/application/entitydoc"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readissues"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/model"
	schemacapread "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/read"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func LoadEntities(
	workspacePath string,
	capability schemacapread.Capability,
	typeFilters []string,
) ([]model.EntityView, *domainerrors.AppError) {
	markdownFiles, scanErr := entitydoc.ScanMarkdownFiles(workspacePath)
	if scanErr != nil {
		return nil, scanErr
	}

	allEntities := make([]parsedDocument, 0, len(markdownFiles))
	for _, path := range markdownFiles {
		parsed, parseErr := parseDocument(path)
		if parseErr != nil {
			return nil, parseErr
		}
		allEntities = append(allEntities, *parsed)
	}

	for _, entity := range allEntities {
		if _, known := capability.EntityTypes[entity.Type]; known {
			continue
		}
		return nil, readissues.NewReadError(
			"failed to determine entity type",
			fmt.Sprintf("entity type '%s' is not declared in schema.entity", entity.Type),
			typeStandardRef,
			nil,
		)
	}

	idIndex := buildIDIndex(allEntities)
	allowedTypes := make(map[string]struct{}, len(typeFilters))
	for _, typeName := range typeFilters {
		allowedTypes[typeName] = struct{}{}
	}

	entityViews := make([]model.EntityView, 0, len(allEntities))
	for _, entity := range allEntities {
		if len(allowedTypes) > 0 {
			if _, keep := allowedTypes[entity.Type]; !keep {
				continue
			}
		}

		entityType := capability.EntityTypes[entity.Type]
		metaPublic := buildMetadata(entity.Frontmatter, entityType.MetaFields)
		metaWhere := buildWhereMetadata(entity.Frontmatter, entityType.MetaFields)
		refsPublic, refsWhere, refsErr := resolveReferences(entity.Frontmatter, entityType.RefFields, idIndex)
		if refsErr != nil {
			return nil, refsErr
		}

		publicView := map[string]any{
			"type":        entity.Type,
			"id":          entity.ID,
			"slug":        entity.Slug,
			"revision":    entity.Revision,
			"createdDate": entity.CreatedDate,
			"updatedDate": entity.UpdatedDate,
			"meta":        metaPublic,
			"refs":        refsPublic,
			"content": map[string]any{
				"raw":      entity.RawContent,
				"sections": sectionsToAnyMap(entity.Sections),
			},
		}

		whereView := map[string]any{
			"type":        entity.Type,
			"id":          entity.ID,
			"slug":        entity.Slug,
			"revision":    entity.Revision,
			"createdDate": entity.CreatedDate,
			"updatedDate": entity.UpdatedDate,
			"meta":        metaWhere,
			"refs":        refsWhere,
			"content": map[string]any{
				"raw":      entity.RawContent,
				"sections": buildWhereSections(entity.Sections, entityType.Sections),
			},
		}

		entityViews = append(entityViews, model.EntityView{
			Type:                   entity.Type,
			ID:                     entity.ID,
			View:                   publicView,
			WhereContext:           whereView,
			DuplicateSectionLabels: entity.DuplicateSectionLabels,
		})
	}

	return entityViews, nil
}
