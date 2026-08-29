// Package markdown turns an entity into the document that lands on disk.
// Frontmatter is emitted builtins first, then meta fields in schema order, so
// the same entity always produces the same bytes and an edit to one field
// shows up as a one-field diff.
//
// add and update share this: a document created by one and then edited by the
// other must stay formatted the same way. They once had a copy each, the fix
// for date formatting reached only one of them, and every update of an added
// document reformatted the whole file.
package markdown

import (
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/writemodel"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/yamlvalues"
	"gopkg.in/yaml.v3"
)

var builtinFrontmatterOrder = []string{"type", "id", "slug", "createdDate", "updatedDate"}

func Serialize(candidate *writemodel.Candidate, typeSpec writemodel.EntityTypeSpec) ([]byte, error) {
	mapping := &yaml.Node{Kind: yaml.MappingNode}
	seen := map[string]struct{}{}

	for _, key := range builtinFrontmatterOrder {
		value, exists := candidate.Frontmatter[key]
		if !exists {
			continue
		}
		if err := appendYAMLField(mapping, key, value); err != nil {
			return nil, err
		}
		seen[key] = struct{}{}
	}

	for _, fieldName := range typeSpec.MetaFieldOrder {
		value, exists := candidate.Frontmatter[fieldName]
		if !exists {
			continue
		}
		if err := appendYAMLField(mapping, fieldName, value); err != nil {
			return nil, err
		}
		seen[fieldName] = struct{}{}
	}

	extraKeys := make([]string, 0)
	for key := range candidate.Frontmatter {
		if _, exists := seen[key]; exists {
			continue
		}
		extraKeys = append(extraKeys, key)
	}
	sort.Strings(extraKeys)
	for _, key := range extraKeys {
		if err := appendYAMLField(mapping, key, candidate.Frontmatter[key]); err != nil {
			return nil, err
		}
	}

	frontmatterRaw, err := yaml.Marshal(mapping)
	if err != nil {
		return nil, err
	}

	frontmatterText := strings.TrimSuffix(string(frontmatterRaw), "\n")
	document := "---\n" + frontmatterText + "\n---"
	if body := normalizeDocumentBody(candidate.Body); body != "" {
		document += "\n\n" + body
	}
	document = applyPlatformNewlines(withTrailingNewline(document))

	return []byte(document), nil
}

func appendYAMLField(mapping *yaml.Node, key string, value any) error {
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	var valueNode *yaml.Node
	if builtinDateNode, ok := buildBuiltinDateNode(key, value); ok {
		valueNode = builtinDateNode
	} else {
		var err error
		valueNode, err = yamlvalues.EncodeYAMLNode(value)
		if err != nil {
			return err
		}
	}
	mapping.Content = append(mapping.Content, keyNode, valueNode)
	return nil
}

func buildBuiltinDateNode(key string, value any) (*yaml.Node, bool) {
	if key != "createdDate" && key != "updatedDate" {
		return nil, false
	}

	switch typed := value.(type) {
	case time.Time:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: typed.Format("2006-01-02")}, true
	case string:
		trimmed := strings.TrimSpace(typed)
		if _, err := time.Parse("2006-01-02", trimmed); err != nil {
			return nil, false
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Value: trimmed}, true
	default:
		return nil, false
	}
}

func applyPlatformNewlines(value string) string {
	normalized := strings.ReplaceAll(value, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	if runtime.GOOS == "windows" {
		return strings.ReplaceAll(normalized, "\n", "\r\n")
	}
	return normalized
}

func withTrailingNewline(value string) string {
	trimmed := strings.TrimRight(value, "\r\n")
	return trimmed + "\n"
}

// normalizeDocumentBody strips the blank line that separates frontmatter from
// the body, so that re-serializing an already serialized document is stable.
// Parsed bodies keep that separator; freshly built ones do not.
func normalizeDocumentBody(body string) string {
	normalized := strings.ReplaceAll(body, "\r\n", "\n")
	return strings.TrimLeft(normalized, "\n")
}
