package support

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

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
		for idx := 0; idx+1 < len(node.Content); idx += 2 {
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

func ToStringMap(value any) (map[string]any, bool) {
	typed, ok := value.(map[string]any)
	if ok {
		return typed, true
	}
	return nil, false
}

func ToSlice(value any) ([]any, bool) {
	typed, ok := value.([]any)
	if ok {
		return typed, true
	}
	return nil, false
}

func ParseYAMLValue(raw string) (any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}

	var node yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &node); err != nil {
		return nil, fmt.Errorf("failed to parse value as yaml: %w", err)
	}

	content := FirstContentNode(&node)
	if content == nil {
		return nil, fmt.Errorf("yaml value is empty")
	}
	if content.Kind == yaml.MappingNode {
		if duplicateKey, ok := FindDuplicateMappingKey(content); ok {
			return nil, fmt.Errorf("yaml value contains duplicate key '%s'", duplicateKey)
		}
	}

	var decoded any
	if err := content.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("failed to decode yaml value: %w", err)
	}
	return decoded, nil
}

func EncodeYAMLNode(value any) (*yaml.Node, error) {
	raw, err := yaml.Marshal(value)
	if err != nil {
		return nil, err
	}

	var node yaml.Node
	if err := yaml.Unmarshal(raw, &node); err != nil {
		return nil, err
	}

	content := FirstContentNode(&node)
	if content == nil {
		return nil, fmt.Errorf("yaml encoded value is empty")
	}
	copyNode := *content
	return &copyNode, nil
}
