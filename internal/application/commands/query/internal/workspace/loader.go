package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/support"
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
	Frontmatter map[string]any
	Sections    map[string]string
	RawContent  string
}

const (
	queryWorkspaceFrontmatterStandardRef = "10.2"
	queryWorkspaceTypeStandardRef        = "5.3"
	queryWorkspaceIDStandardRef          = "11.1"
	queryWorkspaceSlugStandardRef        = "11.2"
)

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
				RefTypeHints:  map[string]string{},
				SectionFields: map[string]struct{}{},
			}
		}
		meta := buildMetadata(entity.Frontmatter, entityTypeSpec.RefFields)
		refs := resolveRefs(entity, entityTypeSpec, idIndex)
		view := map[string]any{
			"type":         entity.Type,
			"id":           entity.ID,
			"slug":         entity.Slug,
			"revision":     entity.Revision,
			"createdDate": entity.CreatedDate,
			"updatedDate": entity.UpdatedDate,
			"meta":         meta,
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
		return nil, newWorkspaceValidationError(
			"failed to parse workspace document",
			parseErr.Error(),
			queryWorkspaceFrontmatterStandardRef,
			nil,
		)
	}

	typeName, ok := readStringField(frontmatter, "type")
	if !ok {
		return nil, newWorkspaceValidationError(
			"failed to determine entity type",
			requiredBuiltinFieldMessage("type"),
			queryWorkspaceTypeStandardRef,
			nil,
		)
	}
	id, ok := readStringField(frontmatter, "id")
	if !ok {
		return nil, newWorkspaceValidationError(
			"failed to determine entity id",
			requiredBuiltinFieldMessage("id"),
			queryWorkspaceIDStandardRef,
			nil,
		)
	}
	slug, ok := readStringField(frontmatter, "slug")
	if !ok {
		return nil, newWorkspaceValidationError(
			"failed to determine entity slug",
			requiredBuiltinFieldMessage("slug"),
			queryWorkspaceSlugStandardRef,
			nil,
		)
	}

	createdDate, _ := readStringField(frontmatter, "createdDate")
	updatedDate, _ := readStringField(frontmatter, "updatedDate")
	normalizedFrontmatter := normalizeMap(frontmatter)

	revisionHash := sha256.Sum256(raw)
	revision := "sha256:" + hex.EncodeToString(revisionHash[:])

	return &parsedEntity{
		Type:        typeName,
		ID:          id,
		Slug:        slug,
		CreatedDate: createdDate,
		UpdatedDate: updatedDate,
		Revision:    revision,
		Frontmatter: normalizedFrontmatter,
		Sections:    extractSections(body),
		RawContent:  body,
	}, nil
}

func buildMetadata(frontmatter map[string]any, refFields map[string]struct{}) map[string]any {
	meta := map[string]any{}
	for key, value := range frontmatter {
		switch key {
		case "type", "id", "slug", "createdDate", "updatedDate":
			continue
		default:
			if _, isRefField := refFields[key]; isRefField {
				continue
			}
			meta[key] = normalizeValue(value)
		}
	}
	return meta
}

func normalizeMap(values map[string]any) map[string]any {
	normalized := make(map[string]any, len(values))
	for key, value := range values {
		normalized[key] = normalizeValue(value)
	}
	return normalized
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
	for _, refField := range support.SortedMapKeys(entityType.RefFields) {
		rawTarget, exists := entity.Frontmatter[refField]
		if !exists {
			refs[refField] = nil
			continue
		}
		hintedType := entityType.RefTypeHints[refField]
		refValue, ok := resolveRefValue(rawTarget, hintedType, idIndex)
		if !ok {
			refs[refField] = nil
			continue
		}
		refs[refField] = refValue
	}
	return refs
}

func resolveRefValue(rawTarget any, hintedType string, idIndex map[string][]entityIdentity) (any, bool) {
	if items, ok := rawTarget.([]any); ok {
		resolvedItems := make([]any, 0, len(items))
		for _, item := range items {
			targetID, ok := readRefID(item)
			if !ok {
				return nil, false
			}
			resolvedItems = append(resolvedItems, buildResolvedRefValue(targetID, hintedType, idIndex))
		}
		return resolvedItems, true
	}

	targetID, ok := readRefID(rawTarget)
	if !ok {
		return nil, false
	}
	return buildResolvedRefValue(targetID, hintedType, idIndex), true
}

func buildResolvedRefValue(targetID string, hintedType string, idIndex map[string][]entityIdentity) map[string]any {
	targets := idIndex[targetID]
	compatibleTargets := filterTargetsByHint(targets, hintedType)
	refValue := map[string]any{
		"id":       targetID,
		"resolved": false,
		"type":     nil,
		"slug":     nil,
	}

	if len(targets) == 1 && isResolvedRefTarget(targets[0], hintedType) {
		target := targets[0]
		refValue["resolved"] = true
		refValue["type"] = target.Type
		refValue["slug"] = target.Slug
		return refValue
	}

	if deterministicType := deterministicRefType(compatibleTargets, hintedType); deterministicType != "" {
		refValue["type"] = deterministicType
	}
	if deterministicSlug := deterministicRefSlug(compatibleTargets); deterministicSlug != "" {
		refValue["slug"] = deterministicSlug
	}

	return refValue
}

func readRefID(rawTarget any) (string, bool) {
	switch typed := rawTarget.(type) {
	case string:
		return normalizeRefID(typed)
	case map[string]any:
		rawID, ok := typed["id"]
		if !ok {
			return "", false
		}
		targetID, ok := rawID.(string)
		if !ok {
			return "", false
		}
		return normalizeRefID(targetID)
	default:
		return "", false
	}
}

func normalizeRefID(raw string) (string, bool) {
	targetID := strings.TrimSpace(raw)
	if targetID == "" {
		return "", false
	}
	return targetID, true
}

func isResolvedRefTarget(target entityIdentity, hintedType string) bool {
	hintedType = strings.TrimSpace(hintedType)
	return hintedType == "" || target.Type == hintedType
}

func filterTargetsByHint(targets []entityIdentity, hintedType string) []entityIdentity {
	hintedType = strings.TrimSpace(hintedType)
	if hintedType == "" {
		return targets
	}
	filtered := make([]entityIdentity, 0, len(targets))
	for _, target := range targets {
		if target.Type == hintedType {
			filtered = append(filtered, target)
		}
	}
	return filtered
}

func deterministicRefType(targets []entityIdentity, hintedType string) string {
	hintedType = strings.TrimSpace(hintedType)
	if hintedType != "" {
		return hintedType
	}
	if len(targets) == 0 {
		return ""
	}
	deterministic := targets[0].Type
	for idx := 1; idx < len(targets); idx++ {
		if targets[idx].Type != deterministic {
			return ""
		}
	}
	return deterministic
}

func deterministicRefSlug(targets []entityIdentity) string {
	if len(targets) == 0 {
		return ""
	}
	deterministic := targets[0].Slug
	for idx := 1; idx < len(targets); idx++ {
		if targets[idx].Slug != deterministic {
			return ""
		}
	}
	return deterministic
}

func sectionsToAnyMap(sections map[string]string) map[string]any {
	mapped := make(map[string]any, len(sections))
	for name, value := range sections {
		mapped[name] = value
	}
	return mapped
}

func requiredBuiltinFieldMessage(field string) string {
	return fmt.Sprintf("built-in field '%s' is required", field)
}

func newWorkspaceValidationError(message string, issueMessage string, standardRef string, details map[string]any) *domainerrors.AppError {
	issue := support.ValidationIssue(
		support.ValidationIssueLevelError,
		support.ValidationIssueClassInstanceError,
		issueMessage,
		standardRef,
	)
	return domainerrors.New(
		domainerrors.CodeWriteFailed,
		message,
		support.WithValidationIssues(details, issue),
	)
}
