package engine

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/engine/internal/markdown"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/engine/internal/pathcalc"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/engine/internal/payload"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/engine/internal/refresolve"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/engine/internal/storage"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/engine/internal/validation"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/engine/internal/writes"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/support"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/workspace"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/responses"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

func Execute(
	opts model.Options,
	writeCapability model.WriteCapability,
	snapshot model.Snapshot,
	now func() time.Time,
) (map[string]any, *domainerrors.AppError) {
	targetMatch, locateErr := locateTarget(snapshot, opts.ID)
	if locateErr != nil {
		return nil, locateErr
	}

	frontmatter, body, parseErr := workspace.ParseFrontmatter(targetMatch.Raw)
	if parseErr != nil {
		return nil, domainerrors.New(
			domainerrors.CodeReadFailed,
			"failed to parse target frontmatter",
			map[string]any{"reason": parseErr.Error()},
		)
	}

	entityType, hasType := workspace.ReadStringField(frontmatter, "type")
	if !hasType {
		return nil, domainerrors.New(
			domainerrors.CodeReadFailed,
			"failed to determine entity type",
			map[string]any{"field": "type"},
		)
	}
	entityID, hasID := workspace.ReadStringField(frontmatter, "id")
	if !hasID {
		return nil, domainerrors.New(
			domainerrors.CodeReadFailed,
			"failed to determine entity id",
			map[string]any{"field": "id"},
		)
	}
	if entityID != opts.ID {
		return nil, domainerrors.New(
			domainerrors.CodeReadFailed,
			"target document id does not match requested --id",
			map[string]any{"expected_id": opts.ID, "actual_id": entityID},
		)
	}

	typeSpec, knownType := writeCapability.EntityTypes[entityType]
	if !knownType {
		return nil, domainerrors.New(
			domainerrors.CodeEntityTypeUnknown,
			fmt.Sprintf("unknown entity type: %s", entityType),
			map[string]any{"entity_type": entityType},
		)
	}

	currentRevision := markdown.ComputePersistedRevision(targetMatch.Raw)
	if opts.ExpectRevision != "" {
		if opts.ExpectRevision != currentRevision {
			return nil, domainerrors.New(
				domainerrors.CodeConcurrencyConflict,
				"--expect-revision does not match current revision",
				map[string]any{
					"expected_revision": opts.ExpectRevision,
					"current_revision":  currentRevision,
				},
			)
		}
	}

	applied, applyErr := writes.Apply(opts, typeSpec, frontmatter, body)
	if applyErr != nil {
		return nil, applyErr
	}

	if now == nil {
		now = time.Now
	}
	if applied.UserChanged {
		applied.Frontmatter["updatedDate"] = now().UTC().Format("2006-01-02")
	}

	candidate := buildCandidate(applied.Frontmatter, applied.Body)
	hydrateMetaAndRefIDs(candidate, typeSpec)

	resolvedRefs, resolvedRefArrays, refIssues := refresolve.Resolve(typeSpec, candidate, snapshot)
	candidate.Refs = resolvedRefs
	candidate.RefArrays = resolvedRefArrays

	evaluationContext := buildEvaluationContext(candidate)

	pathRelPOSIX, pathIssues := pathcalc.Evaluate(typeSpec, candidate, evaluationContext)
	if pathRelPOSIX != "" {
		candidate.PathRelPOSIX = pathRelPOSIX
		candidate.PathAbs = filepath.Join(snapshot.WorkspacePath, filepath.FromSlash(pathRelPOSIX))
	}

	validationIssues := validation.Validate(
		typeSpec,
		candidate,
		snapshot,
		targetMatch.PathAbs,
		pathIssues,
		refIssues,
		evaluationContext,
	)
	if len(validationIssues) > 0 {
		return nil, validation.AsAppError(validationIssues)
	}

	originalRelPOSIX, relErr := filepath.Rel(snapshot.WorkspacePath, targetMatch.PathAbs)
	if relErr != nil {
		return nil, domainerrors.New(
			domainerrors.CodeReadFailed,
			"failed to resolve source relative path",
			map[string]any{"reason": relErr.Error()},
		)
	}
	originalRelPOSIX = filepath.ToSlash(originalRelPOSIX)

	if !applied.UserChanged {
		if candidate.PathRelPOSIX != originalRelPOSIX {
			return nil, validation.AsAppError([]domainvalidation.Issue{
				pathMismatchIssue(candidate),
			})
		}

		candidate.Revision = currentRevision
		entityPayload := payload.BuildEntity(typeSpec, candidate)
		return map[string]any{
			"result_state": responses.ResultStateValid,
			"dry_run":      opts.DryRun,
			"updated":      false,
			"noop":         true,
			"changes":      []any{},
			"entity":       entityPayload,
			"validation": map[string]any{
				"ok":     true,
				"issues": []any{},
			},
		}, nil
	}

	if candidate.PathAbs == "" {
		return nil, validation.AsAppError([]domainvalidation.Issue{
			{
				Code:        "instance.pathTemplate.no_matching_case",
				Level:       domainvalidation.LevelError,
				Class:       "InstanceError",
				Message:     "pathTemplate has no matching case for updated entity",
				StandardRef: "12.4",
				Field:       "schema.pathTemplate",
				Entity:      issueEntity(candidate),
			},
		})
	}

	if storage.IsPathConflict(candidate.PathAbs, snapshot.ExistingPaths, targetMatch.PathAbs) {
		return nil, domainerrors.New(
			domainerrors.CodePathConflict,
			"canonical entity path already exists",
			map[string]any{"path": candidate.PathRelPOSIX},
		)
	}

	serialized, serializeErr := markdown.Serialize(candidate, typeSpec)
	if serializeErr != nil {
		return nil, domainerrors.New(
			domainerrors.CodeInternalError,
			"failed to serialize updated document",
			map[string]any{"reason": serializeErr.Error()},
		)
	}
	candidate.Serialized = serialized
	candidate.Revision = markdown.ComputeRevision(serialized)

	if !opts.DryRun {
		if writeErr := storage.Persist(targetMatch.PathAbs, candidate.PathAbs, serialized); writeErr != nil {
			return nil, writeErr
		}
	}

	entityPayload := payload.BuildEntity(typeSpec, candidate)
	return map[string]any{
		"result_state": responses.ResultStateValid,
		"dry_run":      opts.DryRun,
		"updated":      true,
		"noop":         false,
		"changes":      asAnySlice(applied.Changes),
		"entity":       entityPayload,
		"validation": map[string]any{
			"ok":     true,
			"issues": []any{},
		},
	}, nil
}

func locateTarget(snapshot model.Snapshot, requestedID string) (model.TargetMatch, *domainerrors.AppError) {
	switch len(snapshot.TargetMatches) {
	case 0:
		return model.TargetMatch{}, domainerrors.New(
			domainerrors.CodeEntityNotFound,
			"target entity is not found",
			map[string]any{"id": requestedID},
		)
	case 1:
		return snapshot.TargetMatches[0], nil
	default:
		return model.TargetMatch{}, domainerrors.New(
			domainerrors.CodeTargetAmbiguous,
			"target id is ambiguous",
			map[string]any{"id": requestedID, "matches": len(snapshot.TargetMatches)},
		)
	}
}

func buildCandidate(frontmatter map[string]any, body string) *model.Candidate {
	entityType, _ := workspace.ReadStringField(frontmatter, "type")
	id, _ := workspace.ReadStringField(frontmatter, "id")
	slug, _ := workspace.ReadStringField(frontmatter, "slug")
	createdDate, _ := workspace.ReadStringField(frontmatter, "createdDate")
	updatedDate, _ := workspace.ReadStringField(frontmatter, "updatedDate")

	return &model.Candidate{
		Type:         entityType,
		ID:           id,
		Slug:         slug,
		CreatedDate:  createdDate,
		UpdatedDate:  updatedDate,
		Frontmatter:  frontmatter,
		Meta:         map[string]any{},
		RefIDs:       map[string]string{},
		RefIDArrays:  map[string][]string{},
		Refs:         map[string]model.ResolvedRef{},
		RefArrays:    map[string][]model.ResolvedRef{},
		Body:         strings.ReplaceAll(body, "\r\n", "\n"),
		Sections:     map[string]string{},
		PathRelPOSIX: "",
		PathAbs:      "",
	}
}

func hydrateMetaAndRefIDs(candidate *model.Candidate, typeSpec model.EntityTypeSpec) {
	for _, fieldName := range typeSpec.MetaFieldOrder {
		field := typeSpec.MetaFields[fieldName]
		value, exists := candidate.Frontmatter[fieldName]
		if !exists {
			continue
		}

		if field.IsEntityRef {
			text, ok := value.(string)
			if !ok {
				continue
			}
			text = strings.TrimSpace(text)
			if text == "" {
				continue
			}
			candidate.RefIDs[fieldName] = text
			continue
		}
		if field.IsEntityRefArray {
			items, ok := value.([]any)
			if !ok {
				continue
			}
			ids := make([]string, 0, len(items))
			for _, item := range items {
				itemText, ok := item.(string)
				if !ok || strings.TrimSpace(itemText) == "" {
					ids = nil
					break
				}
				ids = append(ids, strings.TrimSpace(itemText))
			}
			candidate.RefIDArrays[fieldName] = ids
			continue
		}
		candidate.Meta[fieldName] = support.NormalizeValue(value)
	}
}

func pathMismatchIssue(candidate *model.Candidate) domainvalidation.Issue {
	return domainvalidation.Issue{
		Code:        "instance.pathTemplate.path_mismatch",
		Level:       domainvalidation.LevelError,
		Class:       "InstanceError",
		Message:     "entity path does not match canonical pathTemplate result",
		StandardRef: "12.4",
		Field:       "schema.pathTemplate",
		Entity:      issueEntity(candidate),
	}
}

func issueEntity(candidate *model.Candidate) *domainvalidation.Entity {
	return &domainvalidation.Entity{
		Type: candidate.Type,
		ID:   candidate.ID,
		Slug: candidate.Slug,
	}
}

func asAnySlice(items []map[string]any) []any {
	out := make([]any, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	return out
}

func buildEvaluationContext(candidate *model.Candidate) map[string]any {
	if candidate == nil {
		return map[string]any{}
	}

	meta := map[string]any{}
	for key, value := range candidate.Meta {
		meta[key] = value
	}

	refs := map[string]any{}
	for fieldName := range candidate.RefIDs {
		refs[fieldName] = nil
	}
	for fieldName, resolvedRef := range candidate.Refs {
		refs[fieldName] = map[string]any{
			"id":      resolvedRef.ID,
			"type":    resolvedRef.Type,
			"slug":    resolvedRef.Slug,
			"dirPath": resolvedRef.DirPath,
		}
	}
	for fieldName, resolvedRefs := range candidate.RefArrays {
		refItems := make([]any, 0, len(resolvedRefs))
		for _, resolvedRef := range resolvedRefs {
			refItems = append(refItems, map[string]any{
				"id":      resolvedRef.ID,
				"type":    resolvedRef.Type,
				"slug":    resolvedRef.Slug,
				"dirPath": resolvedRef.DirPath,
			})
		}
		refs[fieldName] = refItems
	}

	return map[string]any{
		"type":        candidate.Type,
		"id":          candidate.ID,
		"slug":        candidate.Slug,
		"createdDate": candidate.CreatedDate,
		"updatedDate": candidate.UpdatedDate,
		"meta":        meta,
		"refs":        refs,
	}
}
