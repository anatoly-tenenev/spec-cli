package expressions

import (
	"fmt"
	"reflect"
	"strconv"
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

func IsTruthy(value any) bool {
	return !isFalse(value)
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

func isFalse(value any) bool {
	switch typed := value.(type) {
	case bool:
		return !typed
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	case string:
		return len(typed) == 0
	case nil:
		return true
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Struct:
		return false
	case reflect.Slice, reflect.Map:
		return rv.Len() == 0
	case reflect.Ptr:
		if rv.IsNil() {
			return true
		}
		return isFalse(rv.Elem().Interface())
	}

	return false
}
