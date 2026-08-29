// Package ordered fixes the iteration order the read layer walks maps in.
// Responses must be byte-stable and Go randomizes map ranging, so every place
// that turns a map into output goes through here.
package ordered

import "sort"

func MapKeys[T any](input map[string]T) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
