package expressions

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	jmespath "github.com/anatoly-tenenev/go-jmespath"
)

type CompileMode string

const (
	CompileModeScalar       CompileMode = "scalar"
	CompileModeTemplatePart CompileMode = "template-part"
)

var staticErrorCodeMap = map[string]string{
	"unsupported_schema":        "schema.expression.unsupported_schema",
	"unknown_property":          "schema.expression.unknown_property",
	"unverifiable_property":     "schema.expression.unverifiable_property",
	"invalid_field_target":      "schema.expression.invalid_field_target",
	"invalid_index_target":      "schema.expression.invalid_index_target",
	"invalid_projection_target": "schema.expression.invalid_projection_target",
	"invalid_comparator_types":  "schema.expression.invalid_comparator_types",
	"unknown_function":          "schema.expression.unknown_function",
	"invalid_function_arity":    "schema.expression.invalid_function_arity",
	"invalid_function_arg_type": "schema.expression.invalid_function_arg_type",
	"unverifiable_type":         "schema.expression.unverifiable_type",
	"invalid_enum_value":        "schema.expression.invalid_enum_value",
}

type Engine struct {
	mu             sync.RWMutex
	entityType     string
	compiledSchema *jmespath.CompiledSchema
	cache          map[string]compiledCacheEntry
}

type compiledCacheEntry struct {
	query    *jmespath.JMESPath
	inferred *jmespath.InferredType
}

type CompiledExpression struct {
	Source   string
	Mode     CompileMode
	query    *jmespath.JMESPath
	inferred *jmespath.InferredType
}

type CompileError struct {
	Code       string
	Message    string
	Expression string
	Offset     int
}

func NewEngine() *Engine {
	return &Engine{cache: make(map[string]compiledCacheEntry)}
}

func NewSchemaAwareEngine(entityType string, schema jmespath.JSONSchema) (*Engine, *CompileError) {
	compiledSchema, err := jmespath.CompileSchema(schema)
	if err != nil {
		compileErr := mapCompileError("", err)
		compileErr.Message = fmt.Sprintf("failed to compile expression context schema: %s", compileErr.Message)
		return nil, compileErr
	}

	return &Engine{
		entityType:     strings.TrimSpace(entityType),
		compiledSchema: compiledSchema,
		cache:          make(map[string]compiledCacheEntry),
	}, nil
}

func (e *Engine) Compile(source string, mode CompileMode) (*CompiledExpression, *CompileError) {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return nil, &CompileError{
			Code:       "schema.expression.empty",
			Message:    "expression is empty",
			Expression: source,
			Offset:     0,
		}
	}

	if mode == "" {
		mode = CompileModeScalar
	}

	cacheKey := e.cacheKey(trimmed, mode)
	e.mu.RLock()
	cached, exists := e.cache[cacheKey]
	e.mu.RUnlock()
	if exists {
		return &CompiledExpression{Source: trimmed, Mode: mode, query: cached.query, inferred: cached.inferred}, nil
	}

	compiled, inferred, compileErr := e.compileWithMode(trimmed, mode)
	if compileErr != nil {
		return nil, compileErr
	}

	e.mu.Lock()
	if existing, ok := e.cache[cacheKey]; ok {
		e.mu.Unlock()
		return &CompiledExpression{Source: trimmed, Mode: mode, query: existing.query, inferred: existing.inferred}, nil
	}
	e.cache[cacheKey] = compiledCacheEntry{query: compiled, inferred: inferred}
	e.mu.Unlock()

	return &CompiledExpression{Source: trimmed, Mode: mode, query: compiled, inferred: inferred}, nil
}

func (e *Engine) cacheKey(source string, mode CompileMode) string {
	return e.entityType + "\x00" + string(mode) + "\x00" + source
}

func (e *Engine) compileWithMode(source string, mode CompileMode) (*jmespath.JMESPath, *jmespath.InferredType, *CompileError) {
	if e.compiledSchema == nil {
		compiled, err := jmespath.Compile(source)
		if err != nil {
			return nil, nil, mapCompileError(source, err)
		}
		return compiled, nil, nil
	}

	compiled, err := jmespath.CompileWithCompiledSchema(source, e.compiledSchema)
	if err != nil {
		return nil, nil, mapCompileError(source, err)
	}

	inferred, inferErr := jmespath.InferTypeWithCompiledSchema(source, e.compiledSchema)
	if inferErr != nil {
		return nil, nil, mapCompileError(source, inferErr)
	}

	if typeErr := validateInferredTypeForMode(source, mode, inferred); typeErr != nil {
		return nil, nil, typeErr
	}

	return compiled, inferred, nil
}

func validateInferredTypeForMode(expression string, mode CompileMode, inferred *jmespath.InferredType) *CompileError {
	if inferred == nil {
		return nil
	}

	switch mode {
	case CompileModeScalar:
		if inferred.IsNull() {
			return &CompileError{
				Code:       "schema.expression.invalid_boolean_type",
				Message:    "expression for boolean context cannot be deterministically null",
				Expression: expression,
				Offset:     0,
			}
		}
	case CompileModeTemplatePart:
		if inferred.MayBeString() || inferred.MayBeNumber() || inferred.MayBeBoolean() {
			return nil
		}
		return &CompileError{
			Code:       "schema.interpolation.invalid_result_type",
			Message:    "interpolation expression must be string, number, or boolean in at least one deterministic branch",
			Expression: expression,
			Offset:     0,
		}
	}

	return nil
}

func (e *CompileError) AsStaticCode() string {
	if e == nil {
		return ""
	}
	if mapped, ok := staticErrorCodeMap[e.Code]; ok {
		return mapped
	}
	return ""
}

func (e *CompileError) StaticOffset() int {
	if e == nil {
		return 0
	}
	return e.Offset
}

func mapCompileError(expression string, err error) *CompileError {
	var syntaxErr jmespath.SyntaxError
	if errors.As(err, &syntaxErr) {
		return &CompileError{
			Code:       "schema.expression.syntax_error",
			Message:    "invalid expression syntax",
			Expression: expression,
			Offset:     syntaxErr.Offset,
		}
	}

	var staticErr *jmespath.StaticError
	if errors.As(err, &staticErr) {
		mappedCode, ok := staticErrorCodeMap[staticErr.Code]
		if !ok {
			mappedCode = "schema.expression.static_error"
		}
		return &CompileError{
			Code:       mappedCode,
			Message:    staticErrorMessage(staticErr),
			Expression: expression,
			Offset:     staticErr.Offset,
		}
	}

	return &CompileError{
		Code:       "schema.expression.compile_error",
		Message:    "failed to compile expression",
		Expression: expression,
		Offset:     0,
	}
}

func staticErrorMessage(staticErr *jmespath.StaticError) string {
	if staticErr == nil {
		return "static expression error"
	}
	message := strings.TrimSpace(staticErr.Message)
	if message == "" {
		return "static expression error"
	}
	return message
}

func (e *CompiledExpression) ProtectsWhenTrue(path string) bool {
	if e == nil || e.query == nil {
		return false
	}
	return e.query.ProtectsWhenTrue(path)
}

func (e *CompiledExpression) GuardedPathsWhenTrue() []string {
	if e == nil || e.query == nil {
		return nil
	}
	guards := e.query.GuardsWhenTrue()
	if guards == nil {
		return nil
	}
	return guards.ProtectedPaths()
}

func (e *CompiledExpression) InferredType() *jmespath.InferredType {
	if e == nil {
		return nil
	}
	return e.inferred
}
