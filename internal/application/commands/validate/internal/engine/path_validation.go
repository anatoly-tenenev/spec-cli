package engine

import (
	"fmt"
	pathpkg "path"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	schemacapvalidate "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/validate"
	schemaexpressions "github.com/anatoly-tenenev/spec-cli/internal/application/schema/expressions"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

func validatePathPattern(
	issues *[]domainvalidation.Issue,
	entity *model.CheckedEntity,
	relativePath string,
	typeSpec schemacapvalidate.EntityValidationModel,
	context map[string]any,
) {
	if len(typeSpec.PathPattern.Cases) == 0 {
		return
	}

	var selected *schemacapvalidate.PathPatternCase
	for idx := range typeSpec.PathPattern.Cases {
		caseRule := &typeSpec.PathPattern.Cases[idx]
		shouldUse := false

		switch {
		case !caseRule.HasWhen:
			shouldUse = true
		case caseRule.WhenExpr != nil:
			value, evalErr := schemaexpressions.Evaluate(caseRule.WhenExpr, context)
			if evalErr != nil {
				addIssue(issues, entity, domainvalidation.Issue{
					Code:        "instance.pathTemplate.when_evaluation_failed",
					Level:       domainvalidation.LevelError,
					Class:       "InstanceError",
					Message:     fmt.Sprintf("failed to evaluate pathTemplate case condition: %s", evalErr.Message),
					StandardRef: "11.6",
					Field:       caseRule.WhenPath,
				})
				return
			}
			shouldUse = schemaexpressions.IsTruthy(value)
		default:
			shouldUse = caseRule.When
		}

		if shouldUse {
			selected = caseRule
			break
		}
	}

	if selected == nil {
		addIssue(issues, entity, domainvalidation.Issue{
			Code:        "instance.pathTemplate.no_matching_case",
			Level:       domainvalidation.LevelError,
			Class:       "InstanceError",
			Message:     "pathTemplate has no matching case for entity",
			StandardRef: "8.4",
		})
		return
	}

	expectedPath, renderErr := schemaexpressions.RenderTemplate(selected.UseTemplate, context)
	if renderErr != nil {
		addIssue(issues, entity, domainvalidation.Issue{
			Code:        "instance.pathTemplate.use_interpolation_failed",
			Level:       domainvalidation.LevelError,
			Class:       "InstanceError",
			Message:     fmt.Sprintf("failed to evaluate pathTemplate use: %s", renderErr.Message),
			StandardRef: "8.6",
		})
		return
	}

	normalizedExpected := normalizeRelativePath(expectedPath)
	normalizedActual := normalizeRelativePath(relativePath)
	if normalizedExpected == normalizedActual {
		return
	}

	addIssue(issues, entity, domainvalidation.Issue{
		Code:        "path.pattern_mismatch",
		Level:       domainvalidation.LevelError,
		Class:       "InstanceError",
		Message:     fmt.Sprintf("entity path '%s' does not match expected pattern '%s'", normalizedActual, normalizedExpected),
		StandardRef: "8",
	})
}

func normalizeRelativePath(input string) string {
	normalized := strings.ReplaceAll(input, "\\", "/")
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = strings.TrimPrefix(normalized, "/")
	if normalized == "" {
		return ""
	}
	return pathpkg.Clean(normalized)
}
