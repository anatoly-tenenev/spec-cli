package entitydoc

import (
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var locatorIDPattern = regexp.MustCompile(`^id\s*:\s*(.+?)\s*$`)

// ExtractIDLenient digs the `id` out of a document whose frontmatter may be
// malformed, falling back to a line scan when YAML parsing fails.
//
// This tolerance is deliberate and belongs only to commands that address a
// single document by id: it lets them report that the document is broken
// instead of masking it as NOT_FOUND. Commands that judge conformance must use
// ParseFrontmatter instead, so that a malformed document stays an error.
func ExtractIDLenient(raw []byte) (string, bool) {
	source := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(source, "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return "", false
	}

	endIdx := -1
	for idx := 1; idx < len(lines); idx++ {
		if lines[idx] == "---" || lines[idx] == "..." {
			endIdx = idx
			break
		}
	}
	if endIdx == -1 {
		return extractIDFromLines(strings.Join(lines[1:], "\n"))
	}

	frontmatterBody := strings.Join(lines[1:endIdx], "\n")
	if parsedID, ok := extractIDFromYAML(frontmatterBody); ok {
		return parsedID, true
	}
	return extractIDFromLines(frontmatterBody)
}

func extractIDFromYAML(frontmatterBody string) (string, bool) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(frontmatterBody), &root); err != nil {
		return "", false
	}

	doc := FirstContentNode(&root)
	if doc == nil || doc.Kind != yaml.MappingNode {
		return "", false
	}

	fields := map[string]any{}
	if err := doc.Decode(&fields); err != nil {
		return "", false
	}

	return ReadStringField(fields, "id")
}

func extractIDFromLines(frontmatterBody string) (string, bool) {
	for _, line := range strings.Split(frontmatterBody, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		matches := locatorIDPattern.FindStringSubmatch(trimmed)
		if len(matches) != 2 {
			continue
		}

		value := strings.TrimSpace(matches[1])
		if index := strings.Index(value, " #"); index >= 0 {
			value = strings.TrimSpace(value[:index])
		}
		value = trimWrappingQuotes(value)
		if value == "" {
			continue
		}
		return value, true
	}
	return "", false
}

func trimWrappingQuotes(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) >= 2 {
		if (trimmed[0] == '\'' && trimmed[len(trimmed)-1] == '\'') ||
			(trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"') {
			return strings.TrimSpace(trimmed[1 : len(trimmed)-1])
		}
	}
	return trimmed
}
