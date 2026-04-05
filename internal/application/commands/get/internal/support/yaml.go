package support

import "gopkg.in/yaml.v3"

func FirstContentNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) == 0 {
			return nil
		}
		return node.Content[0]
	}
	return node
}

func FindDuplicateMappingKey(node *yaml.Node) (string, bool) {
	if node == nil {
		return "", false
	}

	switch node.Kind {
	case yaml.MappingNode:
		seen := map[string]struct{}{}
		for idx := 0; idx < len(node.Content); idx += 2 {
			keyNode := node.Content[idx]
			valueNode := node.Content[idx+1]
			if _, exists := seen[keyNode.Value]; exists {
				return keyNode.Value, true
			}
			seen[keyNode.Value] = struct{}{}

			if duplicateKey, ok := FindDuplicateMappingKey(valueNode); ok {
				return duplicateKey, true
			}
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			if duplicateKey, ok := FindDuplicateMappingKey(child); ok {
				return duplicateKey, true
			}
		}
	}

	return "", false
}
