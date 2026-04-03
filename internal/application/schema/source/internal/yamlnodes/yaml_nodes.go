package yamlnodes

import (
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"
)

type DuplicateKey struct {
	Path string
	Key  string
}

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

func FindDuplicateMappingKey(root *yaml.Node) (DuplicateKey, bool) {
	return findDuplicateMappingKey(root, "schema")
}

func findDuplicateMappingKey(node *yaml.Node, path string) (DuplicateKey, bool) {
	if node == nil {
		return DuplicateKey{}, false
	}

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if duplicate, ok := findDuplicateMappingKey(child, path); ok {
				return duplicate, true
			}
		}
		return DuplicateKey{}, false
	case yaml.MappingNode:
		seen := make(map[string]struct{}, len(node.Content)/2)
		for idx := 0; idx+1 < len(node.Content); idx += 2 {
			keyNode := node.Content[idx]
			valueNode := node.Content[idx+1]
			key := keyNode.Value
			if _, exists := seen[key]; exists {
				return DuplicateKey{
					Path: joinDot(path, key),
					Key:  key,
				}, true
			}
			seen[key] = struct{}{}

			if duplicate, ok := findDuplicateMappingKey(valueNode, joinDot(path, key)); ok {
				return duplicate, true
			}
		}
		return DuplicateKey{}, false
	case yaml.SequenceNode:
		for idx, child := range node.Content {
			childPath := fmt.Sprintf("%s[%s]", path, strconv.Itoa(idx))
			if duplicate, ok := findDuplicateMappingKey(child, childPath); ok {
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
