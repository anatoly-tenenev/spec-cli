package expr

import (
	"fmt"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/support"
)

type Lookup interface {
	Lookup(path string) (any, bool)
}

type EvalError struct {
	Message string
}

func Evaluate(expression any, lookup Lookup) (bool, *EvalError) {
	if expression == nil {
		return false, nil
	}

	if boolExpr, ok := expression.(bool); ok {
		return boolExpr, nil
	}

	exprMap, ok := support.ToStringMap(expression)
	if !ok || len(exprMap) != 1 {
		return false, &EvalError{Message: "expression must be a single-operator object"}
	}

	var operator string
	var operand any
	for key, value := range exprMap {
		operator = strings.TrimSpace(key)
		operand = value
		break
	}

	switch operator {
	case "exists":
		pathValue, ok := operand.(string)
		if !ok || strings.TrimSpace(pathValue) == "" {
			return false, &EvalError{Message: "exists operator expects non-empty path string"}
		}
		_, exists := lookup.Lookup(strings.TrimSpace(pathValue))
		return exists, nil
	case "eq", "eq?":
		args, ok := support.ToSlice(operand)
		if !ok || len(args) != 2 {
			return false, &EvalError{Message: fmt.Sprintf("%s operator expects exactly 2 operands", operator)}
		}
		left, leftExists, leftErr := resolveOperand(args[0], lookup)
		if leftErr != nil {
			return false, leftErr
		}
		right, rightExists, rightErr := resolveOperand(args[1], lookup)
		if rightErr != nil {
			return false, rightErr
		}
		if !leftExists || !rightExists {
			if operator == "eq?" {
				return false, nil
			}
			return false, &EvalError{Message: fmt.Sprintf("%s operand references missing value", operator)}
		}
		return support.LiteralEqual(left, right), nil
	case "in", "in?":
		args, ok := support.ToSlice(operand)
		if !ok || len(args) != 2 {
			return false, &EvalError{Message: fmt.Sprintf("%s operator expects exactly 2 operands", operator)}
		}
		left, leftExists, leftErr := resolveOperand(args[0], lookup)
		if leftErr != nil {
			return false, leftErr
		}
		right, rightExists, rightErr := resolveOperand(args[1], lookup)
		if rightErr != nil {
			return false, rightErr
		}
		if !leftExists || !rightExists {
			if operator == "in?" {
				return false, nil
			}
			return false, &EvalError{Message: fmt.Sprintf("%s operand references missing value", operator)}
		}

		rightList, ok := support.ToSlice(right)
		if !ok {
			return false, &EvalError{Message: fmt.Sprintf("%s right operand must be array", operator)}
		}
		for _, item := range rightList {
			if support.LiteralEqual(left, item) {
				return true, nil
			}
		}
		return false, nil
	case "all":
		filters, ok := support.ToSlice(operand)
		if !ok || len(filters) == 0 {
			return false, &EvalError{Message: "all operator expects non-empty array"}
		}
		for _, filter := range filters {
			result, evalErr := Evaluate(filter, lookup)
			if evalErr != nil {
				return false, evalErr
			}
			if !result {
				return false, nil
			}
		}
		return true, nil
	case "any":
		filters, ok := support.ToSlice(operand)
		if !ok || len(filters) == 0 {
			return false, &EvalError{Message: "any operator expects non-empty array"}
		}
		for _, filter := range filters {
			result, evalErr := Evaluate(filter, lookup)
			if evalErr != nil {
				return false, evalErr
			}
			if result {
				return true, nil
			}
		}
		return false, nil
	case "not":
		result, evalErr := Evaluate(operand, lookup)
		if evalErr != nil {
			return false, evalErr
		}
		return !result, nil
	default:
		return false, &EvalError{Message: fmt.Sprintf("unsupported operator '%s'", operator)}
	}
}

func resolveOperand(value any, lookup Lookup) (any, bool, *EvalError) {
	pathValue, isPath := value.(string)
	if isPath && isReferencePath(pathValue) {
		resolved, exists := lookup.Lookup(strings.TrimSpace(pathValue))
		if !exists {
			return nil, false, nil
		}
		return resolved, true, nil
	}

	return support.NormalizeValue(value), true, nil
}

func isReferencePath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	switch value {
	case "type", "id", "slug", "created_date", "updated_date":
		return true
	}
	return strings.HasPrefix(value, "meta.") || strings.HasPrefix(value, "refs.")
}
