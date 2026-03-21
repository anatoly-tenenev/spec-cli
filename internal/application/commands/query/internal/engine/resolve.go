package engine

import "strings"

func resolveReadValue(view map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	var current any = view
	for _, part := range parts {
		typedMap, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		next, exists := typedMap[part]
		if !exists {
			return nil, false
		}
		current = next
	}
	if isOptionalRefLeaf(path) && current == nil {
		return nil, false
	}
	return current, true
}

func isOptionalRefLeaf(path string) bool {
	parts := strings.Split(path, ".")
	if len(parts) != 3 {
		return false
	}
	if parts[0] != "refs" {
		return false
	}
	return parts[2] == "type" || parts[2] == "slug"
}
