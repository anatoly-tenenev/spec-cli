package support

import "gopkg.in/yaml.v3"

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

func FindDuplicateMappingKey(node *yaml.Node) (string, bool) {
	if node == nil || node.Kind != yaml.MappingNode {
		return "", false
	}

	seen := make(map[string]struct{}, len(node.Content)/2)
	for idx := 0; idx+1 < len(node.Content); idx += 2 {
		key := node.Content[idx]
		if _, exists := seen[key.Value]; exists {
			return key.Value, true
		}
		seen[key.Value] = struct{}{}
	}
	return "", false
}
