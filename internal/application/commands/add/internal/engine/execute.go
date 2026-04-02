package engine

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/engine/internal/issues"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/engine/internal/markdown"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/engine/internal/pathcalc"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/engine/internal/payload"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/engine/internal/refresolve"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/engine/internal/storage"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/engine/internal/validation"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/engine/internal/writes"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/responses"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

func Execute(
	opts model.Options,
	typeSpec model.EntityTypeSpec,
	snapshot model.Snapshot,
	now func() time.Time,
) (map[string]any, *domainerrors.AppError) {
	appliedWrites, writesErr := writes.Apply(opts, typeSpec)
	if writesErr != nil {
		return nil, writesErr
	}

	if now == nil {
		now = time.Now
	}
	today := now().UTC().Format("2006-01-02")
	nextSuffix := snapshot.MaxSuffixByType[opts.EntityType] + 1
	candidateID := fmt.Sprintf("%s-%d", typeSpec.IDPrefix, nextSuffix)

	frontmatter := map[string]any{
		"type":        opts.EntityType,
		"id":          candidateID,
		"slug":        opts.Slug,
		"createdDate": today,
		"updatedDate": today,
	}
	for key, value := range appliedWrites.FrontmatterValues {
		frontmatter[key] = value
	}

	candidate := &model.Candidate{
		Type:         opts.EntityType,
		ID:           candidateID,
		Slug:         opts.Slug,
		CreatedDate:  today,
		UpdatedDate:  today,
		Frontmatter:  frontmatter,
		Meta:         appliedWrites.MetaPayload,
		RefIDs:       appliedWrites.RefIDs,
		RefIDArrays:  appliedWrites.RefIDArrays,
		Refs:         map[string]model.ResolvedRef{},
		RefArrays:    map[string][]model.ResolvedRef{},
		Body:         writes.BuildBody(typeSpec, appliedWrites),
		Sections:     map[string]string{},
		PathRelPOSIX: "",
	}

	resolvedRefs, resolvedRefArrays, refIssues := refresolve.Resolve(typeSpec, candidate, snapshot)
	candidate.Refs = resolvedRefs
	candidate.RefArrays = resolvedRefArrays

	evaluationContext := buildEvaluationContext(candidate)

	pathRelPOSIX, pathIssues := pathcalc.Evaluate(typeSpec, candidate, evaluationContext)
	if pathRelPOSIX != "" {
		candidate.PathRelPOSIX = pathRelPOSIX
		candidate.PathAbs = filepath.Join(snapshot.WorkspacePath, filepath.FromSlash(pathRelPOSIX))
		if storage.IsPathConflict(candidate.PathAbs, snapshot.ExistingPaths) {
			return nil, domainerrors.New(
				domainerrors.CodePathConflict,
				"canonical entity path already exists",
				map[string]any{"path": pathRelPOSIX},
			)
		}
	}

	validationIssues := validation.Validate(typeSpec, candidate, snapshot, pathIssues, refIssues, evaluationContext)
	if len(validationIssues) > 0 {
		return nil, validation.AsAppError(validationIssues)
	}

	serialized, serializeErr := markdown.Serialize(candidate, typeSpec)
	if serializeErr != nil {
		return nil, domainerrors.New(
			domainerrors.CodeInternalError,
			"failed to serialize created document",
			map[string]any{"reason": serializeErr.Error()},
		)
	}
	candidate.Serialized = serialized
	candidate.Revision = markdown.ComputeRevision(serialized)

	if !opts.DryRun {
		if candidate.PathAbs == "" {
			return nil, validation.AsAppError([]domainvalidation.Issue{
				issues.New(
					"instance.pathTemplate.no_matching_case",
					"pathTemplate has no matching case for created entity",
					"12.4",
					"schema.pathTemplate",
					candidate,
				),
			})
		}
		writeErr := storage.WriteAtomically(candidate.PathAbs, candidate.Serialized)
		if writeErr != nil {
			return nil, writeErr
		}
	}

	entityPayload := payload.BuildEntity(typeSpec, candidate)

	return map[string]any{
		"result_state": responses.ResultStateValid,
		"dry_run":      opts.DryRun,
		"created":      true,
		"entity":       entityPayload,
		"validation": map[string]any{
			"ok":     true,
			"issues": []any{},
		},
	}, nil
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
