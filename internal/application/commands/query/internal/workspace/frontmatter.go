package workspace

import (
	"fmt"
	"strings"
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/support"
	"gopkg.in/yaml.v3"
)

func parseFrontmatter(raw []byte) (map[string]any, string, error) {
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

func readStringField(values map[string]any, key string) (string, bool) {
	raw, exists := values[key]
	if !exists {
		return "", false
	}

	var value string
	switch typed := raw.(type) {
	case string:
		value = typed
	case time.Time:
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
