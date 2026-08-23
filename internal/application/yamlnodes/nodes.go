// Package yamlnodes holds the YAML tree operations shared by every layer that
// reads YAML: entity frontmatter, schema sources, and command inputs. Keeping
// one implementation here is what stops the traversal rules from drifting
// apart between those layers.
package yamlnodes

import (
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"
)

// DuplicateKey is a repeated mapping key together with the dotted path where
// it was found. Callers that only report the key can ignore Path.
type DuplicateKey struct {
	Path string
	Key  string
}

// FirstContentNode unwraps a document node down to its first content node.
func FirstContentNode(root *yaml.Node) *yaml.Node {
	if root == nil {
		return nil
	}
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			return nil
		}
		return root.Content[0]
	}
	return root
}

// FindDuplicateMappingKey reports the first key repeated within a mapping,
// searching nested mappings and sequences as well. rootPath names the node
// being searched and prefixes the reported path; pass "" when the caller does
// not report paths.
func FindDuplicateMappingKey(root *yaml.Node, rootPath string) (DuplicateKey, bool) {
	if root == nil {
		return DuplicateKey{}, false
	}

	switch root.Kind {
	case yaml.DocumentNode:
		for _, child := range root.Content {
			if duplicate, ok := FindDuplicateMappingKey(child, rootPath); ok {
				return duplicate, true
			}
		}
		return DuplicateKey{}, false
	case yaml.MappingNode:
		seen := make(map[string]struct{}, len(root.Content)/2)
		// idx+1 keeps the value lookup in range if a mapping ever carries an
		// unpaired trailing key.
		for idx := 0; idx+1 < len(root.Content); idx += 2 {
			keyNode := root.Content[idx]
			valueNode := root.Content[idx+1]
			key := keyNode.Value
			if _, exists := seen[key]; exists {
				return DuplicateKey{Path: joinDot(rootPath, key), Key: key}, true
			}
			seen[key] = struct{}{}

			if duplicate, ok := FindDuplicateMappingKey(valueNode, joinDot(rootPath, key)); ok {
				return duplicate, true
			}
		}
		return DuplicateKey{}, false
	case yaml.SequenceNode:
		for idx, child := range root.Content {
			childPath := fmt.Sprintf("%s[%s]", rootPath, strconv.Itoa(idx))
			if duplicate, ok := FindDuplicateMappingKey(child, childPath); ok {
				return duplicate, true
			}
		}
		return DuplicateKey{}, false
	default:
		return DuplicateKey{}, false
	}
}

func joinDot(base string, part string) string {
	if base == "" {
		return part
	}
	if part == "" {
		return base
	}
	return base + "." + part
}
