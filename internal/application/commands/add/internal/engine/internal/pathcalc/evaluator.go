package pathcalc

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/engine/internal/expr"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/engine/internal/issues"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/engine/internal/lookup"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/model"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

var placeholderPattern = regexp.MustCompile(`\{([^{}]+)\}`)

func Evaluate(typeSpec model.EntityTypeSpec, candidate *model.Candidate) (string, []domainvalidation.Issue) {
	pathIssues := make([]domainvalidation.Issue, 0)
	lookupValues := lookup.Candidate{Candidate: candidate}

	selectedUse := ""
	for _, pathCase := range typeSpec.PathPattern.Cases {
		if pathCase.HasWhen {
			matched, evalErr := expr.Evaluate(pathCase.When, lookupValues)
			if evalErr != nil {
				pathIssues = append(pathIssues, issues.New(
					"instance.path_pattern.when_evaluation_failed",
					"failed to evaluate path_pattern.when expression",
					"12.4",
					"schema.path_pattern.when",
					candidate,
				))
				continue
			}
			if !matched {
				continue
			}
		}
		selectedUse = pathCase.Use
		break
	}

	if selectedUse == "" {
		pathIssues = append(pathIssues, issues.New(
			"instance.path_pattern.no_matching_case",
			"path_pattern has no matching case for created entity",
			"12.4",
			"schema.path_pattern",
			candidate,
		))
		return "", pathIssues
	}

	rendered, unresolved := renderPathPattern(selectedUse, lookupValues)
	if len(unresolved) > 0 {
		for _, placeholder := range unresolved {
			pathIssues = append(pathIssues, issues.New(
				"instance.path_pattern.placeholder_unresolved",
				"path_pattern placeholder cannot be resolved: "+placeholder,
				"12.4",
				"schema.path_pattern",
				candidate,
			))
		}
		return "", pathIssues
	}

	normalized := path.Clean(strings.ReplaceAll(rendered, "\\", "/"))
	if normalized == "." || strings.HasPrefix(normalized, "../") || strings.HasPrefix(normalized, "/") {
		pathIssues = append(pathIssues, issues.New(
			"instance.path_pattern.placeholder_unresolved",
			"path_pattern resolved outside workspace",
			"12.4",
			"schema.path_pattern",
			candidate,
		))
		return "", pathIssues
	}
	return normalized, pathIssues
}

func renderPathPattern(pattern string, lookupValues expr.Lookup) (string, []string) {
	if strings.TrimSpace(pattern) == "" {
		return "", []string{"<empty>"}
	}

	matches := placeholderPattern.FindAllStringSubmatchIndex(pattern, -1)
	if len(matches) == 0 {
		return pattern, nil
	}

	var builder strings.Builder
	unresolvedSet := map[string]struct{}{}
	cursor := 0
	for _, match := range matches {
		builder.WriteString(pattern[cursor:match[0]])
		placeholder := strings.TrimSpace(pattern[match[2]:match[3]])
		value, exists := lookupValues.Lookup(placeholder)
		if !exists {
			unresolvedSet[placeholder] = struct{}{}
		} else if rendered, ok := stringifyPlaceholderValue(value); ok {
			builder.WriteString(rendered)
		} else {
			unresolvedSet[placeholder] = struct{}{}
		}
		cursor = match[1]
	}
	builder.WriteString(pattern[cursor:])

	if len(unresolvedSet) == 0 {
		return builder.String(), nil
	}

	unresolved := make([]string, 0, len(unresolvedSet))
	for placeholder := range unresolvedSet {
		unresolved = append(unresolved, placeholder)
	}
	sort.Strings(unresolved)
	return "", unresolved
}

func stringifyPlaceholderValue(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		if typed == "" {
			return "", false
		}
		return typed, true
	case bool:
		if typed {
			return "true", true
		}
		return "false", true
	case int:
		return strconv.Itoa(typed), true
	case int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprintf("%v", typed), true
	case time.Time:
		return typed.Format("2006-01-02"), true
	default:
		return "", false
	}
}
