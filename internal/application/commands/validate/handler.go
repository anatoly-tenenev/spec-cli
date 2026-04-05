package validate

import (
	"context"
	"fmt"
	"sort"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/engine"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/options"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/workspace"
	schemacapvalidate "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/validate"
	schemacompile "github.com/anatoly-tenenev/spec-cli/internal/application/schema/compile"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/requests"
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/responses"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
	"github.com/anatoly-tenenev/spec-cli/internal/output/errormap"
	outputpayload "github.com/anatoly-tenenev/spec-cli/internal/output/payload"
)

type Handler struct {
	newCompiler func() *schemacompile.Compiler
}

func NewHandler() *Handler {
	return &Handler{newCompiler: schemacompile.NewCompiler}
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

	compiler := h.newCompiler()
	compileResult, compileErr := compiler.Compile(schemaPath, request.Global.SchemaPath)
	schemaPayload := outputpayload.BuildSchemaPayload(compileResult)

	if compileErr != nil {
		return buildCompileErrorWithSchema(compileErr, schemaPayload), nil
	}

	validationCapability := schemacapvalidate.Build(compileResult.Schema)
	if typeFilterErr := validateTypeFilters(opts.TypeFilters, validationCapability); typeFilterErr != nil {
		return buildErrorWithSchema(typeFilterErr, schemaPayload), nil
	}

	candidates, candidateErr := workspace.BuildCandidateSet(workspacePath, opts.TypeFilters)
	if candidateErr != nil {
		return buildErrorWithSchema(candidateErr, schemaPayload), nil
	}

	validationRun, runErr := engine.RunValidation(candidates, validationCapability, opts, workspacePath)
	if runErr != nil {
		return buildErrorWithSchema(runErr, schemaPayload), nil
	}

	runtimeIssues := validationRun.Issues
	errorCount, warningCount := engine.CountIssuesByLevel(runtimeIssues)
	validatorConformant := validationRun.ValidatorConformant && !hasProfileIssues(runtimeIssues)

	resultState := responses.ResultStateValid
	if errorCount > 0 {
		resultState = responses.ResultStateInvalid
	}

	skippedEntities := validationRun.CandidateEntities - validationRun.CheckedEntities
	coverageComplete := skippedEntities == 0

	summary := map[string]any{
		"validator_conformant": validatorConformant,
		"entities_scanned":     validationRun.CheckedEntities,
		"entities_valid":       validationRun.EntitiesValid,
		"errors":               errorCount,
		"warnings":             warningCount,
		"coverage": map[string]any{
			"mode":               "strict",
			"complete":           coverageComplete,
			"candidate_entities": validationRun.CandidateEntities,
			"checked_entities":   validationRun.CheckedEntities,
			"skipped_entities":   skippedEntities,
		},
	}

	exitCode := 0
	if errorCount > 0 || (opts.WarningsAsErrors && (warningCount+compileResult.Summary.Warnings) > 0) {
		exitCode = 1
	}

	jsonResponse := map[string]any{
		"result_state":     resultState,
		"validation_scope": "full",
		"schema":           schemaPayload,
		"summary":          summary,
		"issues":           runtimeIssues,
	}

	return responses.CommandOutput{
		JSON:     jsonResponse,
		ExitCode: exitCode,
	}, nil
}

func validateTypeFilters(typeFilters map[string]struct{}, capability schemacapvalidate.Capability) *domainerrors.AppError {
	if len(typeFilters) == 0 {
		return nil
	}

	filteredTypes := make([]string, 0, len(typeFilters))
	for typeName := range typeFilters {
		filteredTypes = append(filteredTypes, typeName)
	}
	sort.Strings(filteredTypes)

	for _, typeName := range filteredTypes {
		if _, exists := capability.EntityTypes[typeName]; exists {
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

func hasProfileIssues(issues []domainvalidation.Issue) bool {
	for _, issue := range issues {
		if issue.Class == "ProfileError" {
			return true
		}
	}
	return false
}

func buildZeroRuntimeSummary() map[string]any {
	return map[string]any{
		"validator_conformant": true,
		"entities_scanned":     0,
		"entities_valid":       0,
		"errors":               0,
		"warnings":             0,
		"coverage": map[string]any{
			"mode":               "strict",
			"complete":           true,
			"candidate_entities": 0,
			"checked_entities":   0,
			"skipped_entities":   0,
		},
	}
}

func buildCompileErrorWithSchema(appErr *domainerrors.AppError, schemaPayload map[string]any) responses.CommandOutput {
	return responses.CommandOutput{
		JSON: map[string]any{
			"result_state":     errormap.ResultStateForCode(appErr.Code),
			"validation_scope": "full",
			"schema":           schemaPayload,
			"summary":          buildZeroRuntimeSummary(),
			"issues":           []domainvalidation.Issue{},
			"error":            outputpayload.BuildErrorPayload(appErr),
		},
		ExitCode: appErr.ExitCode,
	}
}

func buildErrorWithSchema(appErr *domainerrors.AppError, schemaPayload map[string]any) responses.CommandOutput {
	return responses.CommandOutput{
		JSON: map[string]any{
			"result_state":     errormap.ResultStateForCode(appErr.Code),
			"validation_scope": "full",
			"schema":           schemaPayload,
			"error":            outputpayload.BuildErrorPayload(appErr),
		},
		ExitCode: appErr.ExitCode,
	}
}
