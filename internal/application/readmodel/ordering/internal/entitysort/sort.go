package entitysort

import (
	"fmt"
	"sort"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/internal/values"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/model"
)

func SortEntities(entities []model.EntityView, terms []model.SortTerm) {
	sort.SliceStable(entities, func(leftIdx int, rightIdx int) bool {
		left := entities[leftIdx]
		right := entities[rightIdx]
		for _, term := range terms {
			leftValue, leftPresent := resolveReadValue(left.View, term.Path)
			rightValue, rightPresent := resolveReadValue(right.View, term.Path)

			if !leftPresent && !rightPresent {
				continue
			}
			if !leftPresent && rightPresent {
				return term.Direction == model.SortDirectionAsc
			}
			if leftPresent && !rightPresent {
				return term.Direction == model.SortDirectionDesc
			}

			compared := compareLiteralValues(leftValue, rightValue)
			if compared == 0 {
				continue
			}
			if term.Direction == model.SortDirectionAsc {
				return compared < 0
			}
			return compared > 0
		}
		return false
	})
}

func compareLiteralValues(left any, right any) int {
	if leftNum, leftOK := values.NumberToFloat64(left); leftOK {
		if rightNum, rightOK := values.NumberToFloat64(right); rightOK {
			switch {
			case leftNum < rightNum:
				return -1
			case leftNum > rightNum:
				return 1
			default:
				return 0
			}
		}
	}

	leftString, leftStringOK := left.(string)
	rightString, rightStringOK := right.(string)
	if leftStringOK && rightStringOK {
		switch {
		case leftString < rightString:
			return -1
		case leftString > rightString:
			return 1
		default:
			return 0
		}
	}

	leftBool, leftBoolOK := left.(bool)
	rightBool, rightBoolOK := right.(bool)
	if leftBoolOK && rightBoolOK {
		switch {
		case leftBool == rightBool:
			return 0
		case !leftBool && rightBool:
			return -1
		default:
			return 1
		}
	}

	leftRendered := fmt.Sprintf("%v", left)
	rightRendered := fmt.Sprintf("%v", right)
	switch {
	case leftRendered < rightRendered:
		return -1
	case leftRendered > rightRendered:
		return 1
	default:
		return 0
	}
}

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
	return parts[2] == "type" || parts[2] == "slug" || parts[2] == "reason"
}
