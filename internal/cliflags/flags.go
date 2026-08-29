// Package cliflags reads the long-option syntax every command accepts:
// `--name value` and `--name=value`, and `--flag` as a bare boolean.
//
// The rules are small but they are a contract: whether an empty inline value is
// an error, whether the next argument may start with a dash, what a boolean
// without a value means. Each command parses its own options, so these live in
// one place to keep `--type=x` from meaning something different per command.
package cliflags

import (
	"fmt"
	"strconv"
	"strings"

	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

// SplitLong splits `--name=value` into its parts. Only the first `=` separates,
// so a value may contain further ones.
func SplitLong(token string) (string, string, bool) {
	return strings.Cut(token, "=")
}

// Value reads the value of an option, inline or from the next argument. A next
// argument starting with a dash is treated as the following option rather than
// as this one's value, so a forgotten value is reported instead of swallowing
// the next flag.
func Value(args []string, currentIdx int, hasInlineValue bool, inlineValue string) (string, int, *domainerrors.AppError) {
	if hasInlineValue {
		if inlineValue == "" {
			return "", currentIdx, emptyValueError()
		}
		return inlineValue, currentIdx, nil
	}

	nextIdx := currentIdx + 1
	if nextIdx >= len(args) || strings.HasPrefix(args[nextIdx], "-") {
		return "", currentIdx, missingValueError()
	}

	return args[nextIdx], nextIdx, nil
}

// ValueAllowDash is Value for options whose value may legitimately start with a
// dash - `-` for stdin, or a negative number.
func ValueAllowDash(args []string, currentIdx int, hasInlineValue bool, inlineValue string) (string, int, *domainerrors.AppError) {
	if hasInlineValue {
		if inlineValue == "" {
			return "", currentIdx, emptyValueError()
		}
		return inlineValue, currentIdx, nil
	}

	nextIdx := currentIdx + 1
	if nextIdx >= len(args) {
		return "", currentIdx, missingValueError()
	}
	return args[nextIdx], nextIdx, nil
}

// Bool reads a boolean option. Without a value the flag means true; with one it
// must parse, so `--dry-run=maybe` is rejected rather than read as false.
func Bool(name string, hasInlineValue bool, inlineValue string) (bool, *domainerrors.AppError) {
	if !hasInlineValue {
		return true, nil
	}

	parsed, err := strconv.ParseBool(inlineValue)
	if err != nil {
		return false, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			fmt.Sprintf("%s accepts boolean values only", name),
			map[string]any{"value": inlineValue},
		)
	}

	return parsed, nil
}

func emptyValueError() *domainerrors.AppError {
	return domainerrors.New(domainerrors.CodeInvalidArgs, "option value cannot be empty", nil)
}

func missingValueError() *domainerrors.AppError {
	return domainerrors.New(domainerrors.CodeInvalidArgs, "option value is required", nil)
}

// ValueLoose reads a value for commands that accept `-` as a source name: it
// stops only at the next `--option`, and lets an inline value be empty.
//
// This is a second contract, not a variant: `--file=` is an error under Value
// and accepted here. The graphql commands use it, the rest use Value, and
// nothing records why - worth settling rather than copying further.
func ValueLoose(args []string, currentIdx int, hasInlineValue bool, inlineValue string) (string, int, *domainerrors.AppError) {
	if hasInlineValue {
		return inlineValue, currentIdx, nil
	}
	nextIdx := currentIdx + 1
	if nextIdx >= len(args) || strings.HasPrefix(args[nextIdx], "--") {
		return "", currentIdx, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			fmt.Sprintf("missing value for %s", args[currentIdx]),
			nil,
		)
	}
	return args[nextIdx], nextIdx, nil
}

// BoolWords accepts the spoken forms of a boolean on top of what Bool takes.
//
// Also a second contract: `--dry-run=yes` is accepted by add and update and
// rejected by validate and the global options. The difference is not recorded
// anywhere and looks accidental.
func BoolWords(name string, hasInlineValue bool, inlineValue string) (bool, *domainerrors.AppError) {
	if !hasInlineValue {
		return true, nil
	}

	switch strings.ToLower(strings.TrimSpace(inlineValue)) {
	case "true", "1", "yes", "y", "on":
		return true, nil
	case "false", "0", "no", "n", "off":
		return false, nil
	default:
		return false, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			fmt.Sprintf("%s accepts boolean values only", name),
			map[string]any{"value": inlineValue},
		)
	}
}
