// Package collections holds the map iteration order the application depends
// on. Responses must be byte-stable, and Go randomizes map ranging, so every
// place that walks a map sorts its keys through here first.
//
// It sits above the commands because the schema layer needs the same order:
// the capability builders and the compiler walk schema maps, and the order
// they choose reaches responses through the commands that read them. One
// ordering rule, not one per layer.
package collections

import "sort"

func SortedMapKeys[T any](input map[string]T) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// OrderedKeys walks a map in the order its keys were declared, then appends
// whatever the declaration left out, sorted.
//
// The schema keeps a declaration order next to every map, and it is the order
// a caller sees: the fields of an entity in a write response, in the GraphQL
// projection, and in the help rendering are the same fields in the same
// sequence. Names the declaration mentions but the map does not are dropped,
// and a name mentioned twice appears once, so a stale or careless schema
// cannot bend the output.
func OrderedKeys[T any](declared []string, values map[string]T) []string {
	seen := make(map[string]struct{}, len(values))
	ordered := make([]string, 0, len(values))
	for _, key := range declared {
		if _, exists := values[key]; !exists {
			continue
		}
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		ordered = append(ordered, key)
	}
	if len(ordered) == len(values) {
		return ordered
	}

	remaining := make([]string, 0, len(values)-len(ordered))
	for key := range values {
		if _, exists := seen[key]; exists {
			continue
		}
		remaining = append(remaining, key)
	}
	sort.Strings(remaining)
	return append(ordered, remaining...)
}
