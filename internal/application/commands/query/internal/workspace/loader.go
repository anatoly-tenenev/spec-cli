package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

type entityIdentity struct {
	Type string
	ID   string
	Slug string
}

type parsedEntity struct {
	Type        string
	ID          string
	Slug        string
	CreatedDate string
	UpdatedDate string
	Revision    string
	Meta        map[string]any
	Sections    map[string]string
	RawContent  string
}

func LoadEntities(workspacePath string, index model.QuerySchemaIndex, typeFilters []string) ([]model.EntityView, *domainerrors.AppError) {
	markdownFiles, scanErr := scanMarkdownFiles(workspacePath)
	if scanErr != nil {
		return nil, scanErr
	}

	allEntities := make([]parsedEntity, 0, len(markdownFiles))
	for _, path := range markdownFiles {
		parsed, parseErr := parseEntityFile(path)
		if parseErr != nil {
			return nil, parseErr
		}
		allEntities = append(allEntities, *parsed)
	}

	idIndex := buildIDIndex(allEntities)
	allowedTypes := make(map[string]struct{}, len(typeFilters))
	for _, typeName := range typeFilters {
		allowedTypes[typeName] = struct{}{}
	}

	views := make([]model.EntityView, 0, len(allEntities))
	for _, entity := range allEntities {
		if len(allowedTypes) > 0 {
			if _, keep := allowedTypes[entity.Type]; !keep {
				continue
			}
		}

		entityTypeSpec, knownType := index.EntityTypes[entity.Type]
		if !knownType {
			entityTypeSpec = model.EntityTypeSpec{
				Name:          entity.Type,
				RefFields:     map[string]struct{}{},
				SectionFields: map[string]struct{}{},
			}
		}
		refs := resolveRefs(entity, entityTypeSpec, idIndex)
		view := map[string]any{
			"type":         entity.Type,
			"id":           entity.ID,
			"slug":         entity.Slug,
			"revision":     entity.Revision,
			"created_date": entity.CreatedDate,
			"updated_date": entity.UpdatedDate,
			"meta":         entity.Meta,
			"refs":         refs,
			"content": map[string]any{
				"raw":      entity.RawContent,
				"sections": sectionsToAnyMap(entity.Sections),
			},
		}
		views = append(views, model.EntityView{Type: entity.Type, ID: entity.ID, View: view})
	}

	return views, nil
}

func scanMarkdownFiles(workspacePath string) ([]string, *domainerrors.AppError) {
	markdownFiles := make([]string, 0)
	walkErr := filepath.WalkDir(workspacePath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			markdownFiles = append(markdownFiles, path)
		}
		return nil
	})
	if walkErr != nil {
		return nil, domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to scan workspace",
			map[string]any{"reason": walkErr.Error()},
		)
	}

	sort.Strings(markdownFiles)
	return markdownFiles, nil
}

func parseEntityFile(path string) (*parsedEntity, *domainerrors.AppError) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to read workspace document",
			map[string]any{"reason": err.Error()},
		)
	}

	frontmatter, body, parseErr := parseFrontmatter(raw)
	if parseErr != nil {
		return nil, domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to parse workspace document",
			nil,
		)
	}

	typeName, ok := readStringField(frontmatter, "type")
	if !ok {
		return nil, domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to determine entity type",
			nil,
		)
	}
	id, ok := readStringField(frontmatter, "id")
	if !ok {
		return nil, domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to determine entity id",
			nil,
		)
	}
	slug, ok := readStringField(frontmatter, "slug")
	if !ok {
		return nil, domainerrors.New(
			domainerrors.CodeWriteFailed,
			"failed to determine entity slug",
			nil,
		)
	}

	createdDate, _ := readStringField(frontmatter, "created_date")
	updatedDate, _ := readStringField(frontmatter, "updated_date")
	meta := buildMetadata(frontmatter)

	revisionHash := sha256.Sum256(raw)
	revision := "sha256:" + hex.EncodeToString(revisionHash[:])

	return &parsedEntity{
		Type:        typeName,
		ID:          id,
		Slug:        slug,
		CreatedDate: createdDate,
		UpdatedDate: updatedDate,
		Revision:    revision,
		Meta:        meta,
		Sections:    extractSections(body),
		RawContent:  body,
	}, nil
}

func buildMetadata(frontmatter map[string]any) map[string]any {
	meta := map[string]any{}
	for key, value := range frontmatter {
		switch key {
		case "type", "id", "slug", "created_date", "updated_date":
			continue
		default:
			meta[key] = normalizeValue(value)
		}
	}
	return meta
}

func normalizeValue(value any) any {
	switch typed := value.(type) {
	case time.Time:
		return typed.Format("2006-01-02")
	case map[string]any:
		normalized := make(map[string]any, len(typed))
		for key, item := range typed {
			normalized[key] = normalizeValue(item)
		}
		return normalized
	case []any:
		normalized := make([]any, len(typed))
		for idx := range typed {
			normalized[idx] = normalizeValue(typed[idx])
		}
		return normalized
	default:
		return typed
	}
}

func buildIDIndex(entities []parsedEntity) map[string][]entityIdentity {
	idIndex := map[string][]entityIdentity{}
	for _, entity := range entities {
		idIndex[entity.ID] = append(idIndex[entity.ID], entityIdentity{Type: entity.Type, ID: entity.ID, Slug: entity.Slug})
	}
	return idIndex
}

func resolveRefs(entity parsedEntity, entityType model.EntityTypeSpec, idIndex map[string][]entityIdentity) map[string]any {
	refs := map[string]any{}
	for refField := range entityType.RefFields {
		rawTarget, exists := entity.Meta[refField]
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
		targets := idIndex[targetID]
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

func sectionsToAnyMap(sections map[string]string) map[string]any {
	mapped := make(map[string]any, len(sections))
	for name, value := range sections {
		mapped[name] = value
	}
	return mapped
}
