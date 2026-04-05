package pathcalc

import (
	"path"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/engine/internal/issues"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/model"
	schemaexpressions "github.com/anatoly-tenenev/spec-cli/internal/application/schema/expressions"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

func Evaluate(
	typeSpec model.EntityTypeSpec,
	candidate *model.Candidate,
	evaluationContext map[string]any,
) (string, []domainvalidation.Issue) {
	pathIssues := make([]domainvalidation.Issue, 0)

	selectedCase := (*model.PathPatternCase)(nil)
	for idx := range typeSpec.PathPattern.Cases {
		pathCase := &typeSpec.PathPattern.Cases[idx]
		shouldUse := false

		switch {
		case !pathCase.HasWhen:
			shouldUse = true
		case pathCase.WhenExpr != nil:
			whenValue, evalErr := schemaexpressions.Evaluate(pathCase.WhenExpr, evaluationContext)
			if evalErr != nil {
				pathIssues = append(pathIssues, issues.New(
					"instance.pathTemplate.when_evaluation_failed",
					"failed to evaluate pathTemplate.when expression",
					"12.4",
					schemaPathOrDefault(pathCase.WhenPath, "schema.pathTemplate.when"),
					candidate,
				))
				continue
			}
			shouldUse = schemaexpressions.IsTruthy(whenValue)
		default:
			shouldUse = pathCase.When
		}

		if shouldUse {
			selectedCase = pathCase
			break
		}
	}

	if selectedCase == nil {
		pathIssues = append(pathIssues, issues.New(
			"instance.pathTemplate.no_matching_case",
			"pathTemplate has no matching case for created entity",
			"12.4",
			"schema.pathTemplate",
			candidate,
		))
		return "", pathIssues
	}

	rendered, renderErr := schemaexpressions.RenderTemplate(selectedCase.UseTemplate, evaluationContext)
	if renderErr != nil {
		pathIssues = append(pathIssues, issues.New(
			"instance.pathTemplate.placeholder_unresolved",
			"pathTemplate placeholder cannot be resolved: "+renderErrorLabel(renderErr),
			"12.4",
			schemaPathOrDefault(selectedCase.UsePath, "schema.pathTemplate"),
			candidate,
		))
		return "", pathIssues
	}

	normalized := path.Clean(strings.ReplaceAll(rendered, "\\", "/"))
	if normalized == "." || strings.HasPrefix(normalized, "../") || strings.HasPrefix(normalized, "/") {
		pathIssues = append(pathIssues, issues.New(
			"instance.pathTemplate.placeholder_unresolved",
			"pathTemplate resolved outside workspace",
			"12.4",
			schemaPathOrDefault(selectedCase.UsePath, "schema.pathTemplate"),
			candidate,
		))
		return "", pathIssues
	}
	return normalized, pathIssues
}

func renderErrorLabel(evalErr *schemaexpressions.EvalError) string {
	if evalErr == nil {
		return "<unknown>"
	}
	if expression := strings.TrimSpace(evalErr.Expression); expression != "" {
		return expression
	}
	if message := strings.TrimSpace(evalErr.Message); message != "" {
		return message
	}
	return "<unknown>"
}

func schemaPathOrDefault(path string, fallback string) string {
	if strings.TrimSpace(path) != "" {
		return path
	}
	return fallback
}
