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
