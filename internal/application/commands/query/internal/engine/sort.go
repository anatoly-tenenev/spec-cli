package engine

import (
	"fmt"
	"sort"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

var defaultSort = []model.SortTerm{
	{Path: "type", Direction: model.SortDirectionAsc},
	{Path: "id", Direction: model.SortDirectionAsc},
}

func buildEffectiveSort(requested []model.SortTerm, index model.QuerySchemaIndex) ([]model.SortTerm, *domainerrors.AppError) {
	terms := requested
	if len(terms) == 0 {
		terms = append([]model.SortTerm(nil), defaultSort...)
	}

	for _, term := range terms {
		if _, exists := index.SortFields[term.Path]; !exists {
			return nil, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("invalid filter-namespace sort field '%s'", term.Path),
				nil,
			)
		}
	}

	effective := append([]model.SortTerm(nil), terms...)
	if len(effective) < 2 ||
		effective[len(effective)-2].Path != "type" ||
		effective[len(effective)-2].Direction != model.SortDirectionAsc ||
		effective[len(effective)-1].Path != "id" ||
		effective[len(effective)-1].Direction != model.SortDirectionAsc {
		effective = append(effective,
			model.SortTerm{Path: "type", Direction: model.SortDirectionAsc},
			model.SortTerm{Path: "id", Direction: model.SortDirectionAsc},
		)
	}
	return effective, nil
}

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
	if leftNum, leftOK := support.NumberToFloat64(left); leftOK {
		if rightNum, rightOK := support.NumberToFloat64(right); rightOK {
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
