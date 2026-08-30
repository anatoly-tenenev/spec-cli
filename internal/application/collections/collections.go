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
