package engine

import (
	"sort"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/delete/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/delete/internal/storage"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/delete/internal/workspace"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/responses"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func Execute(opts model.Options, schema model.Schema, snapshot model.Snapshot) (map[string]any, *domainerrors.AppError) {
	target, locateErr := locateTarget(opts.ID, snapshot)
	if locateErr != nil {
		return nil, locateErr
	}

	if opts.ExpectRevision != "" && opts.ExpectRevision != target.Revision {
		return nil, domainerrors.New(
			domainerrors.CodeConcurrencyConflict,
			"--expect-revision does not match current revision",
			map[string]any{
				"expected_revision": opts.ExpectRevision,
				"current_revision":  target.Revision,
			},
		)
	}

	blockingRefs := findBlockingReferences(schema, snapshot, target)
	if len(blockingRefs) > 0 {
		return nil, domainerrors.New(
			domainerrors.CodeDeleteBlockedByRefs,
			"delete is blocked by incoming entityRef references",
			map[string]any{"blocking_refs": blockingRefsAsAny(blockingRefs)},
		)
	}

	if !opts.DryRun {
		if deleteErr := storage.Delete(target.PathAbs); deleteErr != nil {
			return nil, deleteErr
		}
	}

	return map[string]any{
		"result_state": responses.ResultStateValid,
		"dry_run":      opts.DryRun,
		"deleted":      true,
		"target": map[string]any{
			"id":       target.ID,
			"revision": target.Revision,
		},
	}, nil
}

func locateTarget(requestedID string, snapshot model.Snapshot) (model.ParsedDocument, *domainerrors.AppError) {
	switch len(snapshot.TargetMatches) {
	case 0:
		return model.ParsedDocument{}, domainerrors.New(
			domainerrors.CodeEntityNotFound,
			"target entity is not found",
			map[string]any{"id": requestedID},
		)
	case 1:
		matched := snapshot.TargetMatches[0]
		target, ok := workspace.FindTargetDocument(snapshot, matched.PathAbs)
		if !ok || target.Type == "" || target.ID == "" || target.ID != requestedID || target.Revision == "" {
			return model.ParsedDocument{}, domainerrors.New(
				domainerrors.CodeRevisionUnavailable,
				"target revision is unavailable",
				map[string]any{"id": requestedID},
			)
		}
		return target, nil
	default:
		return model.ParsedDocument{}, domainerrors.New(
			domainerrors.CodeAmbiguousEntityID,
			"target id is ambiguous",
			map[string]any{"id": requestedID, "matches": len(snapshot.TargetMatches)},
		)
	}
}

func findBlockingReferences(
	schema model.Schema,
	snapshot model.Snapshot,
	target model.ParsedDocument,
) []model.BlockingReference {
	blocking := make([]model.BlockingReference, 0)
	seen := map[string]struct{}{}

	for _, source := range snapshot.Documents {
		if source.PathAbs == target.PathAbs {
			continue
		}

		slots := schema.ReferenceSlotsByType[source.Type]
		if len(slots) == 0 {
			continue
		}

		for _, slot := range slots {
			rawValue, exists := source.Frontmatter[slot.FieldName]
			if !exists {
				continue
			}

			if !slotMatchesTarget(rawValue, slot.Kind, target.ID) {
				continue
			}

			key := source.Type + "\x00" + source.ID + "\x00" + slot.FieldName
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			blocking = append(blocking, model.BlockingReference{
				SourceID:   source.ID,
				SourceType: source.Type,
				Field:      slot.FieldName,
			})
		}
	}

	sort.Slice(blocking, func(left, right int) bool {
		if blocking[left].SourceType != blocking[right].SourceType {
			return blocking[left].SourceType < blocking[right].SourceType
		}
		if blocking[left].SourceID != blocking[right].SourceID {
			return blocking[left].SourceID < blocking[right].SourceID
		}
		return blocking[left].Field < blocking[right].Field
	})

	return blocking
}

func slotMatchesTarget(value any, kind model.ReferenceSlotKind, targetID string) bool {
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return false
	}

	switch kind {
	case model.ReferenceSlotScalar:
		text, ok := value.(string)
		if !ok {
			return false
		}
		return strings.TrimSpace(text) == targetID
	case model.ReferenceSlotArray:
		if values, ok := value.([]any); ok {
			for _, item := range values {
				text, ok := item.(string)
				if !ok {
					continue
				}
				if strings.TrimSpace(text) == targetID {
					return true
				}
			}
			return false
		}
		if values, ok := value.([]string); ok {
			for _, item := range values {
				if strings.TrimSpace(item) == targetID {
					return true
				}
			}
		}
		return false
	default:
		return false
	}
}

func blockingRefsAsAny(items []model.BlockingReference) []any {
	out := make([]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"source_id":   item.SourceID,
			"source_type": item.SourceType,
			"field":       item.Field,
		})
	}
	return out
}
