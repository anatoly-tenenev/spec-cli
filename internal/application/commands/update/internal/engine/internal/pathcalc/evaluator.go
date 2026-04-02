package pathcalc

import (
	"path"
	"strings"

	commandexpressions "github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/expressions"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/engine/internal/issues"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/model"
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
			whenValue, evalErr := commandexpressions.Evaluate(pathCase.WhenExpr, evaluationContext)
			if evalErr != nil {
				pathIssues = append(pathIssues, issues.New(
					"instance.pathTemplate.when_evaluation_failed",
					"failed to evaluate pathTemplate.when expression",
					"12.4",
					"schema.pathTemplate.when",
					candidate,
				))
				continue
			}
			shouldUse = commandexpressions.IsTruthy(whenValue)
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

	rendered, renderErr := commandexpressions.RenderTemplate(selectedCase.UseTemplate, evaluationContext)
	if renderErr != nil {
		pathIssues = append(pathIssues, issues.New(
			"instance.pathTemplate.placeholder_unresolved",
			"pathTemplate placeholder cannot be resolved: "+renderErrorLabel(renderErr),
			"12.4",
			"schema.pathTemplate",
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
			"schema.pathTemplate",
			candidate,
		))
		return "", pathIssues
	}
	return normalized, pathIssues
}

func renderErrorLabel(evalErr *commandexpressions.EvalError) string {
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
