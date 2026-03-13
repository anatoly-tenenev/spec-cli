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
	return current, true
}
