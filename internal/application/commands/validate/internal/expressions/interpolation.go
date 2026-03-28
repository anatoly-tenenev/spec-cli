package expressions

import (
	"fmt"
	"strings"
)

type CompiledTemplate struct {
	Raw   string
	Parts []TemplatePart
}

type TemplatePart struct {
	Literal    string
	Expression *CompiledExpression
}

func ContainsInterpolation(value string) bool {
	return strings.Contains(value, "${")
}

func CompileScalarInterpolation(raw string, engine *Engine) (*CompiledExpression, *CompileError) {
	template, err := compileTemplateWithMode(raw, engine, CompileModeScalar)
	if err != nil {
		return nil, err
	}

	if len(template.Parts) != 1 || template.Parts[0].Expression == nil {
		return nil, &CompileError{
			Code:       "schema.expression.invalid_scalar_interpolation",
			Message:    "value must be a single interpolation in form ${expr}",
			Expression: raw,
			Offset:     0,
		}
	}

	return template.Parts[0].Expression, nil
}

func CompileTemplate(raw string, engine *Engine) (*CompiledTemplate, *CompileError) {
	return compileTemplateWithMode(raw, engine, CompileModeTemplatePart)
}

func compileTemplateWithMode(raw string, engine *Engine, mode CompileMode) (*CompiledTemplate, *CompileError) {
	if engine == nil {
		return nil, &CompileError{
			Code:       "schema.expression.internal_engine_missing",
			Message:    "expression engine is not configured",
			Expression: raw,
			Offset:     0,
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
				Code:       "schema.interpolation.syntax_error",
				Message:    parseErr.Error(),
				Expression: raw,
				Offset:     start,
			}
		}

		expressionSource := strings.TrimSpace(raw[exprStart:exprEnd])
		if expressionSource == "" {
			return nil, &CompileError{
				Code:       "schema.interpolation.empty_expression",
				Message:    "interpolation ${...} must contain a non-empty expression",
				Expression: raw,
				Offset:     start,
			}
		}

		compiledExpression, compileErr := engine.Compile(expressionSource, mode)
		if compileErr != nil {
			compileErr.Expression = expressionSource
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
			stringifyErr.Expression = part.Expression.Source
			return "", stringifyErr
		}
		builder.WriteString(stringValue)
	}

	return builder.String(), nil
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
