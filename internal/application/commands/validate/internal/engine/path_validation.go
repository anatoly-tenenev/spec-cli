package engine

import (
	"fmt"
	pathpkg "path"
	"strconv"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/expressions"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

func validatePathPattern(
	issues *[]domainvalidation.Issue,
	entity *model.CheckedEntity,
	relativePath string,
	typeSpec model.SchemaEntityType,
	context runtimeExpressionContext,
) {
	if len(typeSpec.PathPattern.Cases) == 0 {
		return
	}

	var selected *model.PathPatternCase
	for idx := range typeSpec.PathPattern.Cases {
		caseRule := &typeSpec.PathPattern.Cases[idx]
		shouldUse := false

		switch {
		case !caseRule.HasWhen:
			shouldUse = true
		case caseRule.WhenExpr != nil:
			value, evalErr := expressions.Evaluate(caseRule.WhenExpr, context)
			if evalErr != nil {
				addIssue(issues, entity, domainvalidation.Issue{
					Code:        "instance.pathTemplate.when_evaluation_failed",
					Level:       domainvalidation.LevelError,
					Class:       "InstanceError",
					Message:     fmt.Sprintf("failed to evaluate pathTemplate case condition: %s", evalErr.Message),
					StandardRef: evalErr.StandardRef,
					Field:       caseRule.WhenPath,
				})
				return
			}
			shouldUse = value
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

	expectedPath, renderErr := renderPathTemplate(selected.Use, context)
	if renderErr != nil {
		addIssue(issues, entity, domainvalidation.Issue{
			Code:        "instance.pathTemplate.placeholder_unresolved",
			Level:       domainvalidation.LevelError,
			Class:       "InstanceError",
			Message:     renderErr.Error(),
			StandardRef: "8.5",
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

func renderPathTemplate(template string, context runtimeExpressionContext) (string, error) {
	var builder strings.Builder
	for idx := 0; idx < len(template); idx++ {
		current := template[idx]
		if current == '}' {
			return "", fmt.Errorf("pathTemplate template contains unexpected '}'")
		}
		if current != '{' {
			builder.WriteByte(current)
			continue
		}

		endOffset := strings.IndexByte(template[idx+1:], '}')
		if endOffset < 0 {
			return "", fmt.Errorf("pathTemplate template contains unclosed '{'")
		}
		token := template[idx+1 : idx+1+endOffset]
		if token == "" {
			return "", fmt.Errorf("pathTemplate template contains empty placeholder")
		}

		value, valueErr := resolvePathPlaceholder(token, context)
		if valueErr != nil {
			return "", valueErr
		}
		builder.WriteString(value)

		idx = idx + endOffset + 1
	}

	return builder.String(), nil
}

func resolvePathPlaceholder(token string, context runtimeExpressionContext) (string, error) {
	switch token {
	case "id", "slug", "createdDate", "updatedDate":
		value, exists := context.ResolveReference(expressions.Reference{Kind: expressions.ReferenceMeta, Field: token, Raw: "meta." + token})
		if !exists {
			return "", fmt.Errorf("pathTemplate placeholder '{%s}' is missing", token)
		}
		return stringifyPathValue(value, token)
	}

	if strings.HasPrefix(token, "meta.") {
		fieldName := strings.TrimPrefix(token, "meta.")
		if fieldName == "" || strings.Contains(fieldName, ".") {
			return "", fmt.Errorf("pathTemplate placeholder '{%s}' has invalid format", token)
		}
		value, exists := context.ResolveReference(expressions.Reference{Kind: expressions.ReferenceMeta, Field: fieldName, Raw: "meta." + fieldName})
		if !exists {
			return "", fmt.Errorf("pathTemplate placeholder '{%s}' is missing", token)
		}
		return stringifyPathValue(value, token)
	}

	if strings.HasPrefix(token, "refs.") {
		parts := strings.Split(token, ".")
		if len(parts) != 3 {
			return "", fmt.Errorf("pathTemplate placeholder '{%s}' has invalid format", token)
		}
		if parts[1] == "" {
			return "", fmt.Errorf("pathTemplate placeholder '{%s}' has invalid format", token)
		}
		reference := expressions.Reference{Kind: expressions.ReferenceRefs, Field: parts[1], Part: parts[2], Raw: "refs." + parts[1] + "." + parts[2]}
		value, exists := context.ResolveReference(reference)
		if !exists {
			return "", fmt.Errorf("pathTemplate placeholder '{%s}' is missing", token)
		}
		return stringifyPathValue(value, token)
	}

	return "", fmt.Errorf("unsupported pathTemplate placeholder '{%s}'", token)
}

func stringifyPathValue(value any, token string) (string, error) {
	switch typed := value.(type) {
	case string:
		return typed, nil
	case bool:
		if typed {
			return "true", nil
		}
		return "false", nil
	case nil:
		return "null", nil
	case int:
		return strconv.Itoa(typed), nil
	case int8:
		return strconv.FormatInt(int64(typed), 10), nil
	case int16:
		return strconv.FormatInt(int64(typed), 10), nil
	case int32:
		return strconv.FormatInt(int64(typed), 10), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case uint:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint64:
		return strconv.FormatUint(typed, 10), nil
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), nil
	default:
		return "", fmt.Errorf("pathTemplate placeholder '{%s}' resolved to non-scalar value", token)
	}
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
