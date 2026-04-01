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

type resolvedRef struct {
	ID       string
	Resolved bool
	Type     any
	Slug     any
	Reason   any
}

const (
	queryWorkspaceFrontmatterStandardRef = "10.2"
	queryWorkspaceTypeStandardRef        = "5.3"
	queryWorkspaceIDStandardRef          = "11.1"
	queryWorkspaceSlugStandardRef        = "11.2"
	queryWorkspaceRefsStandardRef        = "6"
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

	for _, entity := range allEntities {
		if _, known := index.EntityTypes[entity.Type]; known {
			continue
		}
		return nil, newWorkspaceReadError(
			"failed to determine entity type",
			fmt.Sprintf("entity type '%s' is not declared in schema.entity", entity.Type),
			queryWorkspaceTypeStandardRef,
			nil,
		)
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

		entityType := index.EntityTypes[entity.Type]
		metaPublic := buildMetadata(entity.Frontmatter, entityType.MetaFields)
		metaWhere := buildMetadata(entity.Frontmatter, entityType.MetaFields)
		refsPublic, refsWhere, refsErr := resolveRefs(entity.Frontmatter, entityType.RefFields, idIndex)
		if refsErr != nil {
			return nil, refsErr
		}

		publicView := map[string]any{
			"type":         entity.Type,
			"id":           entity.ID,
			"slug":         entity.Slug,
			"revision":     entity.Revision,
			"createdDate":  entity.CreatedDate,
			"updatedDate":  entity.UpdatedDate,
			"meta":         metaPublic,
			"refs":         refsPublic,
			"content": map[string]any{
				"raw":      entity.RawContent,
				"sections": sectionsToAnyMap(entity.Sections),
			},
		}

		whereView := map[string]any{
			"type":         entity.Type,
			"id":           entity.ID,
			"slug":         entity.Slug,
			"revision":     entity.Revision,
			"createdDate":  entity.CreatedDate,
			"updatedDate":  entity.UpdatedDate,
			"meta":         metaWhere,
			"refs":         refsWhere,
			"content": map[string]any{
				"sections": buildWhereSections(entity.Sections, entityType.SectionFields),
			},
		}

		views = append(views, model.EntityView{
			Type:         entity.Type,
			ID:           entity.ID,
			View:         publicView,
			WhereContext: whereView,
		})
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
			domainerrors.CodeReadFailed,
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
			domainerrors.CodeReadFailed,
			"failed to read workspace document",
			map[string]any{"reason": err.Error()},
		)
	}

	frontmatter, body, parseErr := parseFrontmatter(raw)
	if parseErr != nil {
		return nil, newWorkspaceReadError(
			"failed to parse workspace document",
			parseErr.Error(),
			queryWorkspaceFrontmatterStandardRef,
			nil,
		)
	}

	typeName, ok := readStringField(frontmatter, "type")
	if !ok {
		return nil, newWorkspaceReadError(
			"failed to determine entity type",
			requiredBuiltinFieldMessage("type"),
			queryWorkspaceTypeStandardRef,
			nil,
		)
	}
	id, ok := readStringField(frontmatter, "id")
	if !ok {
		return nil, newWorkspaceReadError(
			"failed to determine entity id",
			requiredBuiltinFieldMessage("id"),
			queryWorkspaceIDStandardRef,
			nil,
		)
	}
	slug, ok := readStringField(frontmatter, "slug")
	if !ok {
		return nil, newWorkspaceReadError(
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

func buildMetadata(frontmatter map[string]any, knownMeta map[string]model.MetadataFieldSpec) map[string]any {
	meta := map[string]any{}
	for _, field := range support.SortedMapKeys(knownMeta) {
		value, exists := frontmatter[field]
		if !exists {
			continue
		}
		meta[field] = normalizeValue(value)
	}
	return meta
}

func buildWhereSections(parsedSections map[string]string, knownSections map[string]model.SectionFieldSpec) map[string]any {
	sections := map[string]any{}
	for _, sectionName := range support.SortedMapKeys(knownSections) {
		sectionValue, exists := parsedSections[sectionName]
		if !exists {
			continue
		}
		sections[sectionName] = sectionValue
	}
	return sections
}

func normalizeMap(values map[string]any) map[string]any {
	normalized := make(map[string]any, len(values))
	for key, value := range values {
		normalized[key] = normalizeValue(value)
	}
	return normalized
}

func normalizeValue(value any) any {
	if number, ok := support.NumberToFloat64(value); ok {
		return number
	}

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
		idIndex[entity.ID] = append(idIndex[entity.ID], entityIdentity{
			Type: entity.Type,
			ID:   entity.ID,
			Slug: entity.Slug,
		})
	}
	return idIndex
}

func resolveRefs(
	frontmatter map[string]any,
	refFields map[string]model.RefFieldSpec,
	idIndex map[string][]entityIdentity,
) (map[string]any, map[string]any, *domainerrors.AppError) {
	publicRefs := map[string]any{}
	whereRefs := map[string]any{}

	for _, refField := range support.SortedMapKeys(refFields) {
		refSpec := refFields[refField]
		rawTarget, exists := frontmatter[refField]
		if !exists {
			publicRefs[refField] = nil
			continue
		}

		if refSpec.Cardinality == model.RefCardinalityArray {
			publicValue, whereValue, err := resolveArrayRef(rawTarget, refSpec, idIndex, refField)
			if err != nil {
				return nil, nil, err
			}
			publicRefs[refField] = publicValue
			whereRefs[refField] = whereValue
			continue
		}

		publicValue, whereValue, includeInWhere, err := resolveScalarRef(rawTarget, refSpec, idIndex, refField)
		if err != nil {
			return nil, nil, err
		}
		publicRefs[refField] = publicValue
		if includeInWhere {
			whereRefs[refField] = whereValue
		}
	}

	return publicRefs, whereRefs, nil
}

func resolveScalarRef(
	rawTarget any,
	refSpec model.RefFieldSpec,
	idIndex map[string][]entityIdentity,
	refField string,
) (public any, where any, includeInWhere bool, err *domainerrors.AppError) {
	if rawTarget == nil {
		return nil, nil, false, nil
	}

	targetID, ok := readRefID(rawTarget)
	if !ok {
		return nil, nil, false, invalidRefReadError(refField)
	}

	resolved := classifyResolvedRef(targetID, refSpec, idIndex)
	return toPublicRefObject(resolved), toWhereRefObject(resolved), true, nil
}

func resolveArrayRef(
	rawTarget any,
	refSpec model.RefFieldSpec,
	idIndex map[string][]entityIdentity,
	refField string,
) (any, any, *domainerrors.AppError) {
	if rawTarget == nil {
		return nil, nil, nil
	}

	items, ok := rawTarget.([]any)
	if !ok {
		return nil, nil, invalidRefReadError(refField)
	}

	publicItems := make([]any, 0, len(items))
	whereItems := make([]any, 0, len(items))
	for _, item := range items {
		if item == nil {
			publicItems = append(publicItems, nil)
			whereItems = append(whereItems, nil)
			continue
		}
		targetID, ok := readRefID(item)
		if !ok {
			return nil, nil, invalidRefReadError(refField)
		}
		resolved := classifyResolvedRef(targetID, refSpec, idIndex)
		publicItems = append(publicItems, toPublicRefObject(resolved))
		whereItems = append(whereItems, toWhereRefObject(resolved))
	}

	return publicItems, whereItems, nil
}

func classifyResolvedRef(targetID string, refSpec model.RefFieldSpec, idIndex map[string][]entityIdentity) resolvedRef {
	targets := idIndex[targetID]
	compatibleTargets := filterTargetsByRefTypes(targets, refSpec.RefTypes)

	if len(compatibleTargets) == 1 {
		target := compatibleTargets[0]
		return resolvedRef{
			ID:       targetID,
			Resolved: true,
			Type:     target.Type,
			Slug:     target.Slug,
			Reason:   nil,
		}
	}

	reason := "ambiguous"
	switch {
	case len(targets) == 0:
		reason = "missing"
	case len(compatibleTargets) == 0:
		reason = "type_mismatch"
	}

	return resolvedRef{
		ID:       targetID,
		Resolved: false,
		Type:     deterministicRefTypeHint(compatibleTargets, refSpec.RefTypes),
		Slug:     nil,
		Reason:   reason,
	}
}

func toPublicRefObject(ref resolvedRef) map[string]any {
	value := map[string]any{
		"resolved": ref.Resolved,
		"id":       ref.ID,
		"type":     ref.Type,
		"slug":     ref.Slug,
	}
	if !ref.Resolved {
		value["reason"] = ref.Reason
	}
	return value
}

func toWhereRefObject(ref resolvedRef) map[string]any {
	return map[string]any{
		"resolved": ref.Resolved,
		"id":       ref.ID,
		"type":     ref.Type,
		"slug":     ref.Slug,
		"reason":   ref.Reason,
	}
}

func filterTargetsByRefTypes(targets []entityIdentity, refTypes []string) []entityIdentity {
	if len(refTypes) == 0 {
		return targets
	}
	allowed := map[string]struct{}{}
	for _, refType := range refTypes {
		allowed[refType] = struct{}{}
	}
	filtered := make([]entityIdentity, 0, len(targets))
	for _, target := range targets {
		if _, ok := allowed[target.Type]; ok {
			filtered = append(filtered, target)
		}
	}
	return filtered
}

func deterministicRefTypeHint(targets []entityIdentity, refTypes []string) any {
	if len(refTypes) == 1 {
		return refTypes[0]
	}
	if len(targets) == 0 {
		return nil
	}
	candidate := targets[0].Type
	for idx := 1; idx < len(targets); idx++ {
		if targets[idx].Type != candidate {
			return nil
		}
	}
	return candidate
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

func invalidRefReadError(refField string) *domainerrors.AppError {
	return newWorkspaceReadError(
		"failed to compute refs",
		fmt.Sprintf("refs field '%s' has invalid value in frontmatter", refField),
		queryWorkspaceRefsStandardRef,
		map[string]any{"field": refField},
	)
}

func newWorkspaceReadError(message string, issueMessage string, standardRef string, details map[string]any) *domainerrors.AppError {
	issue := support.ValidationIssue(
		support.ValidationIssueLevelError,
		support.ValidationIssueClassInstanceError,
		issueMessage,
		standardRef,
	)
	return domainerrors.New(
		domainerrors.CodeReadFailed,
		message,
		support.WithValidationIssues(details, issue),
	)
}
