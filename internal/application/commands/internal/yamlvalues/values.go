// Package yamlvalues converts between the textual values a command receives on
// the command line and the YAML nodes written back to a document. Only
// commands need this direction of conversion; the tree operations it builds on
// live in internal/application/yamlnodes.
package yamlvalues

import (
	"fmt"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/yamlnodes"
	"gopkg.in/yaml.v3"
)

func ParseYAMLValue(raw string) (any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}

	var node yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &node); err != nil {
		return nil, fmt.Errorf("failed to parse value as yaml: %w", err)
	}

	content := yamlnodes.FirstContentNode(&node)
	if content == nil {
		return nil, fmt.Errorf("yaml value is empty")
	}
	if content.Kind == yaml.MappingNode {
		if duplicate, ok := yamlnodes.FindDuplicateMappingKey(content, ""); ok {
			return nil, fmt.Errorf("yaml value contains duplicate key '%s'", duplicate.Key)
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

	content := yamlnodes.FirstContentNode(&node)
	if content == nil {
		return nil, fmt.Errorf("yaml encoded value is empty")
	}
	copyNode := *content
	return &copyNode, nil
}
