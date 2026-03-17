package yamlnodes

import (
	"fmt"
	"strconv"
	"strings"

	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	"gopkg.in/yaml.v3"
)

type MappingEntry struct {
	Key   string
	Value *yaml.Node
}

func ReadMapping(node *yaml.Node, path string) ([]MappingEntry, map[string]*yaml.Node, error) {
	if node == nil {
		return nil, nil, fmt.Errorf("%s is required", path)
	}
	if node.Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("%s must be a mapping", path)
	}

	entries := make([]MappingEntry, 0, len(node.Content)/2)
	byKey := make(map[string]*yaml.Node, len(node.Content)/2)
	for idx := 0; idx < len(node.Content); idx += 2 {
		keyNode := node.Content[idx]
		valueNode := node.Content[idx+1]
		if keyNode.Kind != yaml.ScalarNode {
			return nil, nil, fmt.Errorf("%s contains non-scalar key", path)
		}
		key := strings.TrimSpace(keyNode.Value)
		if key == "" {
			return nil, nil, fmt.Errorf("%s contains empty key", path)
		}
		if _, exists := byKey[key]; exists {
			return nil, nil, fmt.Errorf("%s contains duplicate key '%s'", path, key)
		}
		byKey[key] = valueNode
		entries = append(entries, MappingEntry{Key: key, Value: valueNode})
	}
	return entries, byKey, nil
}

func EnsureAllowedKeys(entries []MappingEntry, allowed map[string]struct{}, path string) error {
	for _, entry := range entries {
		if _, ok := allowed[entry.Key]; ok {
			continue
		}
		return fmt.Errorf("%s has unsupported key '%s'", path, entry.Key)
	}
	return nil
}

func RequiredScalar(node *yaml.Node, path string) (*yaml.Node, error) {
	if node == nil {
		return nil, fmt.Errorf("%s is required", path)
	}
	if node.Kind != yaml.ScalarNode {
		return nil, fmt.Errorf("%s must be scalar", path)
	}
	return CloneScalar(node), nil
}

func ParseBoolScalar(node *yaml.Node, path string) (bool, error) {
	scalar, err := RequiredScalar(node, path)
	if err != nil {
		return false, err
	}
	parsed, parseErr := strconv.ParseBool(strings.TrimSpace(scalar.Value))
	if parseErr != nil {
		return false, fmt.Errorf("%s must be boolean", path)
	}
	return parsed, nil
}

func ScalarSequence(node *yaml.Node, path string, flowStyle bool) (*yaml.Node, error) {
	if node == nil {
		return nil, fmt.Errorf("%s is required", path)
	}
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%s must be array", path)
	}
	out := SequenceNode(flowStyle)
	for idx, item := range node.Content {
		if item.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("%s[%d] must be scalar", path, idx)
		}
		out.Content = append(out.Content, CloneScalar(item))
	}
	return out, nil
}

func MappingNode() *yaml.Node {
	return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{}}
}

func SequenceNode(flowStyle bool) *yaml.Node {
	node := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{}}
	if flowStyle {
		node.Style = yaml.FlowStyle
	}
	return node
}

func BoolScalar(value bool) *yaml.Node {
	if value {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"}
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"}
}

func CloneScalar(node *yaml.Node) *yaml.Node {
	if node == nil {
		return &yaml.Node{Kind: yaml.ScalarNode, Value: ""}
	}
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   node.Tag,
		Value: node.Value,
		Style: node.Style,
	}
}

func AppendMapping(mapping *yaml.Node, key string, value *yaml.Node) {
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		value,
	)
}

func ProjectionError(err error) *domainerrors.AppError {
	return domainerrors.New(
		domainerrors.CodeSchemaProjectionError,
		fmt.Sprintf("failed to project schema for help: %v", err),
		nil,
	)
}
