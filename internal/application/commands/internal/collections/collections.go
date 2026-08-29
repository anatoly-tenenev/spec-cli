// Package collections holds the map iteration order commands depend on.
// Responses must be byte-stable, and Go randomizes map ranging, so every
// command that walks a map sorts its keys through here first.
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
