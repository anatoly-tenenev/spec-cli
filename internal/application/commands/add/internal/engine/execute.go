// Package engine builds the new entity and writes it: apply the requested
// writes, assign the next id, resolve references, compute the path, validate,
// and only then serialize to disk. Validation comes before the write because a
// write command must not leave the workspace non-conforming - a rejected add
// changes nothing.
//
// Under --dry-run everything up to the write still runs, so the response
// reports what would have been created rather than a guess.
//
// execute.go orchestrates the run; writes.go turns --set into values,
// validation.go judges the result, storage.go puts it on disk, and payload.go
// shapes what comes back.
package engine

import (
	"path/filepath"
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/entityids"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/issues"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/markdown"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/pathcalc"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/refresolve"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/schemarules"
	"github.com/anatoly-tenenev/spec-cli/internal/application/entitydoc"
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
	appliedWrites, writesErr := applyWrites(opts, typeSpec)
	if writesErr != nil {
		return nil, writesErr
	}

	if now == nil {
		now = time.Now
	}
	today := now().UTC().Format("2006-01-02")
	nextSuffix := snapshot.MaxSuffixByType[opts.EntityType] + 1
	candidateID := entityids.Format(typeSpec.IDPrefix, nextSuffix)

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
		Body:         buildBody(typeSpec, appliedWrites),
		Sections:     map[string]string{},
		PathRelPOSIX: "",
	}

	resolvedRefs, resolvedRefArrays, refIssues := refresolve.Resolve(typeSpec, candidate, snapshot.EntitiesByID)
	candidate.Refs = resolvedRefs
	candidate.RefArrays = resolvedRefArrays

	evaluationContext := schemarules.BuildContext(candidate)

	pathRelPOSIX, pathIssues := pathcalc.Evaluate(typeSpec, candidate, evaluationContext)
	if pathRelPOSIX != "" {
		candidate.PathRelPOSIX = pathRelPOSIX
		candidate.PathAbs = filepath.Join(snapshot.WorkspacePath, filepath.FromSlash(pathRelPOSIX))
		if isPathConflict(candidate.PathAbs, snapshot.ExistingPaths) {
			return nil, domainerrors.New(
				domainerrors.CodePathConflict,
				"canonical entity path already exists",
				map[string]any{"path": pathRelPOSIX},
			)
		}
	}

	validationIssues := validateCandidate(typeSpec, candidate, snapshot, pathIssues, refIssues, evaluationContext)
	if len(validationIssues) > 0 {
		return nil, validationFailedError(validationIssues)
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
	candidate.Revision = entitydoc.Revision(serialized)

	if !opts.DryRun {
		if candidate.PathAbs == "" {
			return nil, validationFailedError([]domainvalidation.Issue{
				issues.New(
					"instance.pathTemplate.no_matching_case",
					"pathTemplate has no matching case for created entity",
					"8.4",
					"schema.pathTemplate",
					candidate,
				),
			})
		}
		writeErr := writeAtomically(candidate.PathAbs, candidate.Serialized)
		if writeErr != nil {
			return nil, writeErr
		}
	}

	entityPayload := buildEntityPayload(typeSpec, candidate)

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
