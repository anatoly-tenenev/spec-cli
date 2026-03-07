package validate

import (
	"context"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/engine"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/options"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/schema"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/workspace"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/responses"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) Handle(_ context.Context, request requests.Command) (responses.CommandOutput, *domainerrors.AppError) {
	opts, parseErr := options.Parse(request.Args)
	if parseErr != nil {
		return responses.CommandOutput{}, parseErr
	}

	workspacePath, schemaPath, pathErr := options.NormalizePaths(request.Global)
	if pathErr != nil {
		return responses.CommandOutput{}, pathErr
	}

	loadedSchema, schemaIssues, schemaErr := schema.Load(schemaPath)
	if schemaErr != nil {
		return responses.CommandOutput{}, schemaErr
	}

	candidates, candidateErr := workspace.BuildCandidateSet(workspacePath, opts.TypeFilters)
	if candidateErr != nil {
		return responses.CommandOutput{}, candidateErr
	}

	run, runErr := engine.RunValidation(candidates, loadedSchema, opts, workspacePath)
	if runErr != nil {
		return responses.CommandOutput{}, runErr
	}

	issues := make([]domainvalidation.Issue, 0, len(schemaIssues)+len(run.Issues))
	issues = append(issues, schemaIssues...)
	issues = append(issues, run.Issues...)

	errorCount, warningCount := engine.CountIssuesByLevel(issues)
	schemaValid := countSchemaErrors(issues) == 0
	validatorConformant := run.ValidatorConformant && !hasProfileIssues(issues)

	resultState := responses.ResultStateValid
	if errorCount > 0 {
		resultState = responses.ResultStateInvalid
	}

	skippedEntities := run.CandidateEntities - run.CheckedEntities
	coverageComplete := skippedEntities == 0

	summary := map[string]any{
		"schema_valid":         schemaValid,
		"validator_conformant": validatorConformant,
		"entities_scanned":     run.CheckedEntities,
		"entities_valid":       run.EntitiesValid,
		"errors":               errorCount,
		"warnings":             warningCount,
		"coverage": map[string]any{
			"mode":               "strict",
			"complete":           coverageComplete,
			"candidate_entities": run.CandidateEntities,
			"checked_entities":   run.CheckedEntities,
			"skipped_entities":   skippedEntities,
		},
	}

	jsonResponse := map[string]any{
		"result_state":     resultState,
		"validation_scope": "full",
		"summary":          summary,
		"issues":           issues,
	}

	ndjsonResponse := make([]map[string]any, 0, len(issues)+1)
	for _, issue := range issues {
		ndjsonResponse = append(ndjsonResponse, map[string]any{
			"record_type": "issue",
			"issue":       issue,
		})
	}
	ndjsonResponse = append(ndjsonResponse, map[string]any{
		"record_type":      "summary",
		"result_state":     resultState,
		"validation_scope": "full",
		"summary":          summary,
	})

	exitCode := 0
	if errorCount > 0 || (opts.WarningsAsErrors && warningCount > 0) {
		exitCode = 1
	}

	return responses.CommandOutput{
		JSON:     jsonResponse,
		NDJSON:   ndjsonResponse,
		ExitCode: exitCode,
	}, nil
}

func countSchemaErrors(issues []domainvalidation.Issue) int {
	count := 0
	for _, issue := range issues {
		if issue.Class == "SchemaError" && issue.Level == domainvalidation.LevelError {
			count++
		}
	}
	return count
}

func hasProfileIssues(issues []domainvalidation.Issue) bool {
	for _, issue := range issues {
		if issue.Class == "ProfileError" {
			return true
		}
	}
	return false
}
