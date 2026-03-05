package workspace

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/support"
	"gopkg.in/yaml.v3"
)

var (
	headingPattern            = regexp.MustCompile(`^\s{0,3}#{1,6}\s+(.+?)\s*$`)
	headingLinkLabelPattern   = regexp.MustCompile(`^\[(.+)]\(#([^\s#()]+)\)\s*$`)
	headingSuffixLabelPattern = regexp.MustCompile(`^(.*?)\s+\{#([^\s{}]+)\}\s*$`)
)

func ParseFrontmatter(raw []byte) (map[string]any, string, error) {
	source := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(source, "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return nil, "", fmt.Errorf("frontmatter must start with '---' on the first line")
	}

	endIdx := -1
	for idx := 1; idx < len(lines); idx++ {
		if lines[idx] == "---" || lines[idx] == "..." {
			endIdx = idx
			break
		}
	}
	if endIdx == -1 {
		return nil, "", fmt.Errorf("frontmatter closing delimiter ('---' or '...') is missing")
	}

	frontmatterBody := strings.Join(lines[1:endIdx], "\n")
	body := strings.Join(lines[endIdx+1:], "\n")

	var root yaml.Node
	if err := yaml.Unmarshal([]byte(frontmatterBody), &root); err != nil {
		return nil, "", fmt.Errorf("frontmatter is not valid yaml: %w", err)
	}

	doc := support.FirstContentNode(&root)
	if doc == nil || doc.Kind != yaml.MappingNode {
		return nil, "", fmt.Errorf("frontmatter root must be a yaml mapping")
	}

	if duplicateKey, ok := support.FindDuplicateMappingKey(doc); ok {
		return nil, "", fmt.Errorf("frontmatter contains duplicate key '%s'", duplicateKey)
	}

	fields := map[string]any{}
	if err := doc.Decode(&fields); err != nil {
		return nil, "", fmt.Errorf("frontmatter decode failed: %w", err)
	}

	return fields, body, nil
}

func ExtractSectionLabels(body string) (map[string]string, []string) {
	labels := map[string]string{}
	duplicateSet := map[string]struct{}{}
	duplicates := make([]string, 0)

	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	for _, line := range lines {
		matches := headingPattern.FindStringSubmatch(line)
		if len(matches) != 2 {
			continue
		}

		heading := strings.TrimSpace(matches[1])
		label, title, ok := parseHeadingLabel(heading)
		if !ok {
			continue
		}

		if _, exists := labels[label]; exists {
			if _, seen := duplicateSet[label]; !seen {
				duplicateSet[label] = struct{}{}
				duplicates = append(duplicates, label)
			}
			continue
		}
		labels[label] = title
	}

	sort.Strings(duplicates)
	return labels, duplicates
}

func ReadStringField(values map[string]any, key string) (string, bool) {
	raw, exists := values[key]
	if !exists {
		return "", false
	}
	var value string
	switch typed := raw.(type) {
	case string:
		value = typed
	case time.Time:
		// yaml.v3 may decode plain YYYY-MM-DD scalars as timestamp values.
		value = typed.Format("2006-01-02")
	default:
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return value, true
}

func parseHeadingLabel(heading string) (label string, title string, ok bool) {
	if linkMatches := headingLinkLabelPattern.FindStringSubmatch(heading); len(linkMatches) == 3 {
		return linkMatches[2], strings.TrimSpace(linkMatches[1]), true
	}

	if suffixMatches := headingSuffixLabelPattern.FindStringSubmatch(heading); len(suffixMatches) == 3 {
		return suffixMatches[2], strings.TrimSpace(suffixMatches[1]), true
	}

	return "", "", false
}
