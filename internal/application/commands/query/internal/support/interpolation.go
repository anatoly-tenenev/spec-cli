package support

import (
	"fmt"
	"strings"

	jmespath "github.com/anatoly-tenenev/go-jmespath"
)

func ValidateSingleInterpolation(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "${") {
		return fmt.Errorf("value must be a single interpolation in form ${expr}")
	}
	exprEnd, err := findInterpolationEnd(trimmed, 2)
	if err != nil {
		return err
	}
	if exprEnd != len(trimmed)-1 {
		return fmt.Errorf("value must be a single interpolation in form ${expr}")
	}
	expressionSource := strings.TrimSpace(trimmed[2:exprEnd])
	if expressionSource == "" {
		return fmt.Errorf("interpolation ${...} must contain a non-empty expression")
	}
	_, err = jmespath.Compile(expressionSource)
	return err
}

func findInterpolationEnd(input string, start int) (int, error) {
	braceDepth, bracketDepth, parenDepth := 0, 0, 0
	inSingleQuote, inDoubleQuote, inBacktick, escaped := false, false, false, false
	for idx := start; idx < len(input); idx++ {
		current := input[idx]
		if inSingleQuote || inDoubleQuote || inBacktick {
			if escaped {
				escaped = false
				continue
			}
			if current == '\\' {
				escaped = true
				continue
			}
			if (inSingleQuote && current == '\'') || (inDoubleQuote && current == '"') || (inBacktick && current == '`') {
				inSingleQuote, inDoubleQuote, inBacktick = false, false, false
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
