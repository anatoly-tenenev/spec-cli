package expressions

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"

	jmespath "github.com/anatoly-tenenev/go-jmespath"
)

type Engine struct {
	mu    sync.RWMutex
	cache map[string]*jmespath.JMESPath
}

type CompiledExpression struct {
	Source string
	query  *jmespath.JMESPath
}

type CompiledTemplate struct {
	Raw   string
	Parts []TemplatePart
}

type TemplatePart struct {
	Literal    string
	Expression *CompiledExpression
}

type CompileError struct {
	Code    string
	Message string
	Offset  int
}

type EvalError struct {
	Code       string
	Message    string
	Expression string
}

func NewEngine() *Engine {
	return &Engine{cache: make(map[string]*jmespath.JMESPath)}
}

func (e *Engine) Compile(source string) (*CompiledExpression, *CompileError) {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return nil, &CompileError{
			Code:    "schema.expression.empty",
			Message: "expression is empty",
			Offset:  0,
		}
	}
	if e == nil {
		return nil, &CompileError{
			Code:    "schema.expression.internal_engine_missing",
			Message: "expression engine is not configured",
			Offset:  0,
		}
	}

	e.mu.RLock()
	cached, ok := e.cache[trimmed]
	e.mu.RUnlock()
	if ok {
		return &CompiledExpression{Source: trimmed, query: cached}, nil
	}

	compiled, err := jmespath.Compile(trimmed)
	if err != nil {
		return nil, mapCompileError(err)
	}

	e.mu.Lock()
	if existing, exists := e.cache[trimmed]; exists {
		e.mu.Unlock()
		return &CompiledExpression{Source: trimmed, query: existing}, nil
	}
	e.cache[trimmed] = compiled
	e.mu.Unlock()

	return &CompiledExpression{Source: trimmed, query: compiled}, nil
}

func CompileScalarInterpolation(raw string, engine *Engine) (*CompiledExpression, *CompileError) {
	template, err := CompileTemplate(raw, engine)
	if err != nil {
		return nil, err
	}

	if len(template.Parts) != 1 || template.Parts[0].Expression == nil {
		return nil, &CompileError{
			Code:    "schema.expression.invalid_scalar_interpolation",
			Message: "value must be a single interpolation in form ${expr}",
			Offset:  0,
		}
	}

	return template.Parts[0].Expression, nil
}

func CompileTemplate(raw string, engine *Engine) (*CompiledTemplate, *CompileError) {
	if engine == nil {
		return nil, &CompileError{
			Code:    "schema.expression.internal_engine_missing",
			Message: "expression engine is not configured",
			Offset:  0,
		}
	}

	parts := make([]TemplatePart, 0, 4)
	cursor := 0
	for cursor < len(raw) {
		relativeStart := strings.Index(raw[cursor:], "${")
		if relativeStart < 0 {
			if cursor < len(raw) {
				parts = append(parts, TemplatePart{Literal: raw[cursor:]})
			}
			break
		}

		start := cursor + relativeStart
		if start > cursor {
			parts = append(parts, TemplatePart{Literal: raw[cursor:start]})
		}

		exprStart := start + 2
		exprEnd, parseErr := findInterpolationEnd(raw, exprStart)
		if parseErr != nil {
			return nil, &CompileError{
				Code:    "schema.interpolation.syntax_error",
				Message: parseErr.Error(),
				Offset:  start,
			}
		}

		expressionSource := strings.TrimSpace(raw[exprStart:exprEnd])
		if expressionSource == "" {
			return nil, &CompileError{
				Code:    "schema.interpolation.empty_expression",
				Message: "interpolation ${...} must contain a non-empty expression",
				Offset:  start,
			}
		}

		compiledExpression, compileErr := engine.Compile(expressionSource)
		if compileErr != nil {
			return nil, compileErr
		}
		parts = append(parts, TemplatePart{Expression: compiledExpression})
		cursor = exprEnd + 1
	}

	if len(parts) == 0 {
		parts = append(parts, TemplatePart{Literal: raw})
	}

	return &CompiledTemplate{Raw: raw, Parts: parts}, nil
}

func ContainsLegacyPlaceholder(raw string) (bool, *CompileError) {
	cursor := 0
	for cursor < len(raw) {
		relativeStart := strings.Index(raw[cursor:], "${")
		if relativeStart < 0 {
			return literalContainsBraces(raw[cursor:]), nil
		}

		start := cursor + relativeStart
		if literalContainsBraces(raw[cursor:start]) {
			return true, nil
		}

		exprEnd, parseErr := findInterpolationEnd(raw, start+2)
		if parseErr != nil {
			return false, &CompileError{
				Code:    "schema.interpolation.syntax_error",
				Message: parseErr.Error(),
				Offset:  start,
			}
		}
		cursor = exprEnd + 1
	}

	return false, nil
}

func Evaluate(expression *CompiledExpression, context any) (any, *EvalError) {
	if expression == nil || expression.query == nil {
		return nil, &EvalError{
			Code:    "instance.expression.invalid",
			Message: "expression is not compiled",
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

func RenderTemplate(template *CompiledTemplate, context any) (string, *EvalError) {
	if template == nil {
		return "", &EvalError{
			Code:    "instance.interpolation.invalid_template",
			Message: "template is not compiled",
		}
	}

	var builder strings.Builder
	for _, part := range template.Parts {
		if part.Expression == nil {
			builder.WriteString(part.Literal)
			continue
		}

		value, evalErr := Evaluate(part.Expression, context)
		if evalErr != nil {
			return "", evalErr
		}
		stringValue, stringifyErr := StringifyInterpolationValue(value)
		if stringifyErr != nil {
			if strings.TrimSpace(stringifyErr.Expression) == "" {
				stringifyErr.Expression = part.Expression.Source
			}
			return "", stringifyErr
		}
		builder.WriteString(stringValue)
	}

	return builder.String(), nil
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

func mapCompileError(err error) *CompileError {
	var syntaxErr jmespath.SyntaxError
	if errors.As(err, &syntaxErr) {
		return &CompileError{
			Code:    "schema.expression.syntax_error",
			Message: "invalid expression syntax",
			Offset:  syntaxErr.Offset,
		}
	}

	var staticErr *jmespath.StaticError
	if errors.As(err, &staticErr) {
		code := "schema.expression.static_error"
		if trimmed := strings.TrimSpace(staticErr.Code); trimmed != "" {
			code = "schema.expression." + trimmed
		}
		message := strings.TrimSpace(staticErr.Message)
		if message == "" {
			message = "static expression error"
		}
		return &CompileError{
			Code:    code,
			Message: message,
			Offset:  staticErr.Offset,
		}
	}

	return &CompileError{
		Code:    "schema.expression.compile_error",
		Message: "failed to compile expression",
		Offset:  0,
	}
}

func findInterpolationEnd(input string, start int) (int, error) {
	braceDepth := 0
	bracketDepth := 0
	parenDepth := 0

	inSingleQuote := false
	inDoubleQuote := false
	inBacktick := false
	escaped := false

	for idx := start; idx < len(input); idx++ {
		current := input[idx]

		if inSingleQuote {
			if escaped {
				escaped = false
				continue
			}
			if current == '\\' {
				escaped = true
				continue
			}
			if current == '\'' {
				inSingleQuote = false
			}
			continue
		}
		if inDoubleQuote {
			if escaped {
				escaped = false
				continue
			}
			if current == '\\' {
				escaped = true
				continue
			}
			if current == '"' {
				inDoubleQuote = false
			}
			continue
		}
		if inBacktick {
			if escaped {
				escaped = false
				continue
			}
			if current == '\\' {
				escaped = true
				continue
			}
			if current == '`' {
				inBacktick = false
			}
			continue
		}

		switch current {
		case '\'':
			inSingleQuote = true
		case '"':
			inDoubleQuote = true
		case '`':
			inBacktick = true
		case '{':
			braceDepth++
		case '}':
			if braceDepth == 0 && bracketDepth == 0 && parenDepth == 0 {
				return idx, nil
			}
			if braceDepth > 0 {
				braceDepth--
			}
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		}
	}

	return -1, fmt.Errorf("unterminated interpolation, missing closing '}'")
}

func literalContainsBraces(value string) bool {
	return strings.Contains(value, "{") || strings.Contains(value, "}")
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
