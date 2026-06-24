package loading

import (
	"fmt"

	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/workspace/internal/diagnostics"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/workspace/internal/documents"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/workspace/internal/references"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/workspace/internal/views"
	schemacapread "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/read"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func LoadEntities(
	workspacePath string,
	capability schemacapread.Capability,
	typeFilters []string,
) ([]model.EntityView, *domainerrors.AppError) {
	markdownFiles, scanErr := documents.ScanMarkdownFiles(workspacePath)
	if scanErr != nil {
		return nil, scanErr
	}

	allEntities := make([]documents.Entity, 0, len(markdownFiles))
	for _, path := range markdownFiles {
		parsed, parseErr := documents.ParseEntityFile(path)
		if parseErr != nil {
			return nil, parseErr
		}
		allEntities = append(allEntities, *parsed)
	}

	for _, entity := range allEntities {
		if _, known := capability.EntityTypes[entity.Type]; known {
			continue
		}
		return nil, diagnostics.NewReadError(
			"failed to determine entity type",
			fmt.Sprintf("entity type '%s' is not declared in schema.entity", entity.Type),
			diagnostics.TypeStandardRef,
			nil,
		)
	}

	idIndex := references.BuildIDIndex(allEntities)
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
		metaPublic := views.BuildMetadata(entity.Frontmatter, entityType.MetaFields)
		metaWhere := views.BuildMetadata(entity.Frontmatter, entityType.MetaFields)
		refsPublic, refsWhere, refsErr := references.Resolve(entity.Frontmatter, entityType.RefFields, idIndex)
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
				"sections": views.SectionsToAnyMap(entity.Sections),
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
				"sections": views.BuildWhereSections(entity.Sections, entityType.Sections),
			},
		}

		entityViews = append(entityViews, model.EntityView{
			Type:         entity.Type,
			ID:           entity.ID,
			View:         publicView,
			WhereContext: whereView,
		})
	}

	return entityViews, nil
}
