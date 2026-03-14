package validate

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/engine"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
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

	loadedSchema, schemaIssues, schemaErr := schema.Load(schemaPath, request.Global.SchemaPath)
	if schemaErr != nil {
		if schemaErr.Code != domainerrors.CodeSchemaInvalid {
			return responses.CommandOutput{}, schemaErr
		}
		schemaIssues = append(schemaIssues, schemaIssueFromAppError(schemaErr))
	}

	run := model.ValidationRun{ValidatorConformant: true}
	if countSchemaErrors(schemaIssues) == 0 {
		if typeFilterErr := validateTypeFilters(opts.TypeFilters, loadedSchema); typeFilterErr != nil {
			return responses.CommandOutput{}, typeFilterErr
		}

		candidates, candidateErr := workspace.BuildCandidateSet(workspacePath, opts.TypeFilters)
		if candidateErr != nil {
			return responses.CommandOutput{}, candidateErr
		}

		validationRun, runErr := engine.RunValidation(candidates, loadedSchema, opts, workspacePath)
		if runErr != nil {
			return responses.CommandOutput{}, runErr
		}
		run = validationRun
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

	exitCode := 0
	if errorCount > 0 || (opts.WarningsAsErrors && warningCount > 0) {
		exitCode = 1
	}

	return responses.CommandOutput{
		JSON:     jsonResponse,
		ExitCode: exitCode,
	}, nil
}

func validateTypeFilters(typeFilters map[string]struct{}, schema model.ValidationSchema) *domainerrors.AppError {
	if len(typeFilters) == 0 {
		return nil
	}

	filteredTypes := make([]string, 0, len(typeFilters))
	for typeName := range typeFilters {
		filteredTypes = append(filteredTypes, typeName)
	}
	sort.Strings(filteredTypes)

	for _, typeName := range filteredTypes {
		if _, exists := schema.Entity[typeName]; exists {
			continue
		}
		return domainerrors.New(
			domainerrors.CodeEntityTypeUnknown,
			fmt.Sprintf("unknown entity type: %s", typeName),
			map[string]any{"entity_type": typeName},
		)
	}

	return nil
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

func schemaIssueFromAppError(schemaErr *domainerrors.AppError) domainvalidation.Issue {
	issue := domainvalidation.Issue{
		Code:        "schema.invalid",
		Level:       domainvalidation.LevelError,
		Class:       "SchemaError",
		Message:     schemaErr.Message,
		StandardRef: "7",
	}

	if code, ok := detailString(schemaErr.Details, "code"); ok && code != "" {
		issue.Code = code
	}
	if field, ok := detailString(schemaErr.Details, "field"); ok {
		issue.Field = field
	}
	if standardRef, ok := detailString(schemaErr.Details, "standard_ref"); ok && strings.TrimSpace(standardRef) != "" {
		issue.StandardRef = strings.TrimSpace(standardRef)
	}

	return issue
}

func detailString(details map[string]any, key string) (string, bool) {
	if len(details) == 0 {
		return "", false
	}

	raw, exists := details[key]
	if !exists {
		return "", false
	}

	value, ok := raw.(string)
	if !ok {
		return "", false
	}

	return value, true
}
