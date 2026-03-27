package markdown

import (
	"crypto/sha256"
	"encoding/hex"
	"runtime"
	"sort"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/support"
	"gopkg.in/yaml.v3"
)

var builtinFrontmatterOrder = []string{"type", "id", "slug", "createdDate", "updatedDate"}

func Serialize(candidate *model.Candidate, typeSpec model.EntityTypeSpec) ([]byte, error) {
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
	if candidate.Body != "" {
		document += "\n\n" + strings.ReplaceAll(candidate.Body, "\r\n", "\n")
	}
	document = applyPlatformNewlines(withTrailingNewline(document))

	return []byte(document), nil
}

// ComputeRevision returns the revision token for serialized candidate bytes.
func ComputeRevision(serialized []byte) string {
	return computeSHA256Revision(serialized)
}

// ComputePersistedRevision returns the revision token for bytes as persisted on disk.
func ComputePersistedRevision(raw []byte) string {
	return computeSHA256Revision(raw)
}

func computeSHA256Revision(data []byte) string {
	hash := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(hash[:])
}

func appendYAMLField(mapping *yaml.Node, key string, value any) error {
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	valueNode, err := support.EncodeYAMLNode(value)
	if err != nil {
		return err
	}
	mapping.Content = append(mapping.Content, keyNode, valueNode)
	return nil
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
