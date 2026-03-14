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
		"type":         opts.EntityType,
		"id":           candidateID,
		"slug":         opts.Slug,
		"created_date": today,
		"updated_date": today,
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
		Refs:         map[string]model.ResolvedRef{},
		Body:         writes.BuildBody(typeSpec, appliedWrites),
		Sections:     map[string]string{},
		PathRelPOSIX: "",
	}

	resolvedRefs, refIssues := refresolve.Resolve(typeSpec, candidate, snapshot)
	candidate.Refs = resolvedRefs

	pathRelPOSIX, pathIssues := pathcalc.Evaluate(typeSpec, candidate)
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

	validationIssues := validation.Validate(typeSpec, candidate, snapshot, pathIssues, refIssues)
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
					"instance.path_pattern.no_matching_case",
					"path_pattern has no matching case for created entity",
					"12.4",
					"schema.path_pattern",
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
