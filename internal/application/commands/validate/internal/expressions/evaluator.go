package expressions

import "fmt"

type Context interface {
	ResolveReference(reference Reference) (any, bool)
}

type EvaluationError struct {
	Code        string
	Message     string
	StandardRef string
}

func Evaluate(expression *Expression, ctx Context) (bool, *EvaluationError) {
	if expression == nil {
		return false, &EvaluationError{
			Code:        "instance.expression.invalid_operand_type",
			Message:     "expression is not compiled",
			StandardRef: "11.6",
		}
	}

	switch expression.Operator {
	case OpEq, OpEqSafe:
		return evaluateEq(expression, ctx)
	case OpIn, OpInSafe:
		return evaluateIn(expression, ctx)
	case OpAll:
		for _, subexpression := range expression.Subexpressions {
			result, evalErr := Evaluate(subexpression, ctx)
			if evalErr != nil {
				return false, evalErr
			}
			if !result {
				return false, nil
			}
		}
		return true, nil
	case OpAny:
		for _, subexpression := range expression.Subexpressions {
			result, evalErr := Evaluate(subexpression, ctx)
			if evalErr != nil {
				return false, evalErr
			}
			if result {
				return true, nil
			}
		}
		return false, nil
	case OpNot:
		result, evalErr := Evaluate(expression.Subexpression, ctx)
		if evalErr != nil {
			return false, evalErr
		}
		return !result, nil
	case OpExists:
		if expression.ExistsRef == nil {
			return false, &EvaluationError{
				Code:        "instance.expression.invalid_operand_type",
				Message:     "exists operator expects a context reference",
				StandardRef: "11.6",
			}
		}
		_, exists := ctx.ResolveReference(*expression.ExistsRef)
		return exists, nil
	default:
		return false, &EvaluationError{
			Code:        "instance.expression.invalid_operator",
			Message:     fmt.Sprintf("unsupported compiled operator '%s'", expression.Operator),
			StandardRef: "11.6",
		}
	}
}

func evaluateEq(expression *Expression, ctx Context) (bool, *EvaluationError) {
	if len(expression.Operands) != 2 {
		return false, invalidOperandTypeError("eq operands must contain two items")
	}

	left, leftMissing, leftErr := resolveComparableOperand(expression.Operands[0], ctx)
	if leftErr != nil {
		return false, leftErr
	}
	right, rightMissing, rightErr := resolveComparableOperand(expression.Operands[1], ctx)
	if rightErr != nil {
		return false, rightErr
	}

	if leftMissing || rightMissing {
		if expression.Operator == OpEqSafe {
			return false, nil
		}
		return false, missingOperandError(expression.Operator)
	}

	return literalEqual(left, right), nil
}

func evaluateIn(expression *Expression, ctx Context) (bool, *EvaluationError) {
	if len(expression.Operands) != 1 {
		return false, invalidOperandTypeError("in operator expects exactly one test operand")
	}
	if len(expression.ListOperands) == 0 {
		return false, invalidOperandTypeError("in operator expects non-empty comparison list")
	}

	needle, missingNeedle, needleErr := resolveComparableOperand(expression.Operands[0], ctx)
	if needleErr != nil {
		return false, needleErr
	}

	if missingNeedle {
		if expression.Operator == OpInSafe {
			return false, nil
		}
		return false, missingOperandError(expression.Operator)
	}

	for _, rawCandidate := range expression.ListOperands {
		candidate, missingCandidate, candidateErr := resolveComparableOperand(rawCandidate, ctx)
		if candidateErr != nil {
			return false, candidateErr
		}
		if missingCandidate {
			if expression.Operator == OpInSafe {
				return false, nil
			}
			return false, missingOperandError(expression.Operator)
		}

		if literalEqual(needle, candidate) {
			return true, nil
		}
	}

	return false, nil
}

func resolveComparableOperand(operand Operand, ctx Context) (any, bool, *EvaluationError) {
	if operand.Reference != nil {
		value, exists := ctx.ResolveReference(*operand.Reference)
		if !exists {
			return nil, true, nil
		}
		if !isScalarLiteral(value) {
			return nil, false, invalidOperandTypeError("resolved reference is not a scalar value")
		}
		return value, false, nil
	}

	if !isScalarLiteral(operand.Literal) {
		return nil, false, invalidOperandTypeError("operand literal is not scalar")
	}

	return operand.Literal, false, nil
}

func missingOperandError(operator Operator) *EvaluationError {
	return &EvaluationError{
		Code:        "instance.expression.missing_operand_strict",
		Message:     fmt.Sprintf("operator '%s' received missing operand", operator),
		StandardRef: "11.6",
	}
}

func invalidOperandTypeError(message string) *EvaluationError {
	return &EvaluationError{
		Code:        "instance.expression.invalid_operand_type",
		Message:     message,
		StandardRef: "11.6",
	}
}

func literalEqual(left any, right any) bool {
	leftNumber, leftIsNumber := numberToFloat64(left)
	rightNumber, rightIsNumber := numberToFloat64(right)
	if leftIsNumber && rightIsNumber {
		return leftNumber == rightNumber
	}

	switch leftTyped := left.(type) {
	case string:
		rightTyped, ok := right.(string)
		return ok && leftTyped == rightTyped
	case bool:
		rightTyped, ok := right.(bool)
		return ok && leftTyped == rightTyped
	case nil:
		return right == nil
	default:
		return false
	}
}

func numberToFloat64(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	default:
		return 0, false
	}
}
