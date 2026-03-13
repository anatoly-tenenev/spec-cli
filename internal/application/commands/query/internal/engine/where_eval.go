package engine

import (
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/support"
)

func EvaluateFilter(filter model.FilterNode, entityView map[string]any) bool {
	switch filter.Kind {
	case model.FilterNodeAnd:
		for _, nested := range filter.Filters {
			if !EvaluateFilter(nested, entityView) {
				return false
			}
		}
		return true
	case model.FilterNodeOr:
		for _, nested := range filter.Filters {
			if EvaluateFilter(nested, entityView) {
				return true
			}
		}
		return false
	case model.FilterNodeNot:
		if filter.Filter == nil {
			return false
		}
		return !EvaluateFilter(*filter.Filter, entityView)
	case model.FilterNodeLeaf:
		value, present := resolveReadValue(entityView, filter.Field)
		return evaluateLeaf(filter, value, present)
	default:
		return false
	}
}

func evaluateLeaf(filter model.FilterNode, actual any, present bool) bool {
	switch filter.Op {
	case "exists":
		return present
	case "not_exists":
		return !present
	}

	if !present {
		return false
	}

	switch filter.Op {
	case "eq":
		return support.LiteralEqual(actual, filter.Value)
	case "neq":
		return !support.LiteralEqual(actual, filter.Value)
	case "in":
		for _, candidate := range filter.Value.([]any) {
			if support.LiteralEqual(actual, candidate) {
				return true
			}
		}
		return false
	case "not_in":
		for _, candidate := range filter.Value.([]any) {
			if support.LiteralEqual(actual, candidate) {
				return false
			}
		}
		return true
	case "gt", "gte", "lt", "lte":
		comparison := compareForRange(actual, filter.Value, filter.Spec.Kind)
		switch filter.Op {
		case "gt":
			return comparison > 0
		case "gte":
			return comparison >= 0
		case "lt":
			return comparison < 0
		case "lte":
			return comparison <= 0
		}
	case "contains":
		switch typed := actual.(type) {
		case string:
			substr, ok := filter.Value.(string)
			return ok && strings.Contains(typed, substr)
		case []any:
			for _, candidate := range typed {
				if support.LiteralEqual(candidate, filter.Value) {
					return true
				}
			}
			return false
		default:
			return false
		}
	}

	return false
}

func compareForRange(left any, right any, kind model.SchemaFieldKind) int {
	switch kind {
	case model.FieldKindDate:
		leftDate, _ := left.(string)
		rightDate, _ := right.(string)
		switch {
		case leftDate < rightDate:
			return -1
		case leftDate > rightDate:
			return 1
		default:
			return 0
		}
	default:
		leftNum, _ := support.NumberToFloat64(left)
		rightNum, _ := support.NumberToFloat64(right)
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
