// Package duplicates reports which keys of a workspace-wide index were claimed
// more than once - the ids, slugs and id suffixes that must be unique. The
// result is sorted, because these become issues in a response that has to be
// byte-stable.
package duplicates

import "sort"

func DuplicatedStringKeys(index map[string][]int) []string {
	duplicates := make([]string, 0)
	for value, indexes := range index {
		if len(indexes) > 1 {
			duplicates = append(duplicates, value)
		}
	}
	sort.Strings(duplicates)
	return duplicates
}

func DuplicatedIntKeys(index map[int][]int) []int {
	duplicates := make([]int, 0)
	for value, indexes := range index {
		if len(indexes) > 1 {
			duplicates = append(duplicates, value)
		}
	}
	sort.Ints(duplicates)
	return duplicates
}
