package expressions

import (
	"fmt"
	"strconv"

	"github.com/anatoly-tenenev/spec-cli/internal/application/values"
)

type EvalError struct {
	Code       string
	Message    string
	Expression string
}

func Evaluate(expression *CompiledExpression, context any) (any, *EvalError) {
	if expression == nil || expression.query == nil {
		return nil, &EvalError{
			Code:       "instance.expression.invalid",
			Message:    "expression is not compiled",
			Expression: "",
		}
	}

	value, err := expression.query.Search(context)
	if err != nil {
		return nil, &EvalError{
			Code:       "instance.expression.evaluation_failed",
			Message:    err.Error(),
			Expression: expression.Source,
		}
	}

	return value, nil
}

// IsTruthy is the shared JMESPath truthiness rule; see
// internal/application/values. It stays part of this package's surface because
// callers ask the expression layer whether an expression held, not what
// JMESPath thinks of a value.
func IsTruthy(value any) bool {
	return values.IsTruthy(value)
}

func StringifyInterpolationValue(value any) (string, *EvalError) {
	switch typed := value.(type) {
	case string:
		return typed, nil
	case bool:
		if typed {
			return "true", nil
		}
		return "false", nil
	case int:
		return strconv.FormatInt(int64(typed), 10), nil
	case int8:
		return strconv.FormatInt(int64(typed), 10), nil
	case int16:
		return strconv.FormatInt(int64(typed), 10), nil
	case int32:
		return strconv.FormatInt(int64(typed), 10), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case uint:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint64:
		return strconv.FormatUint(typed, 10), nil
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), nil
	case nil:
		return "", &EvalError{Code: "instance.interpolation.type_mismatch", Message: "interpolation result must be string, number, or boolean"}
	default:
		return "", &EvalError{
			Code:    "instance.interpolation.type_mismatch",
			Message: fmt.Sprintf("interpolation result has unsupported type %T", value),
		}
	}
}
