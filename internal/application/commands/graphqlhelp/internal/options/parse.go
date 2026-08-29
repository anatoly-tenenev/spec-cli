// Package options parses the graphql-help command's arguments and resolves
// workspace and schema paths. --entity narrows the SDL, so it is refused
// without --schema-only rather than silently ignored while the catalog is
// printed in full.
package options

import (
	"fmt"
	"github.com/anatoly-tenenev/spec-cli/internal/cliflags"
	"strings"

	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

type Options struct {
	SchemaOnly bool
	Entities   []string
}

func Parse(args []string) (Options, *domainerrors.AppError) {
	opts := Options{Entities: []string{}}
	for idx := 0; idx < len(args); idx++ {
		token := args[idx]
		name, inline, hasInline := cliflags.SplitLong(token)
		if !strings.HasPrefix(name, "--") {
			return Options{}, domainerrors.New(domainerrors.CodeInvalidArgs, fmt.Sprintf("unknown graphql-help option: %s", token), nil)
		}
		switch name {
		case "--schema-only":
			if hasInline {
				return Options{}, domainerrors.New(domainerrors.CodeInvalidArgs, "--schema-only does not take a value", nil)
			}
			opts.SchemaOnly = true
		case "--entity":
			value, next, err := cliflags.ValueLoose(args, idx, hasInline, inline)
			if err != nil {
				return Options{}, err
			}
			value = strings.TrimSpace(value)
			if value == "" {
				return Options{}, domainerrors.New(domainerrors.CodeInvalidArgs, "--entity value cannot be empty", nil)
			}
			opts.Entities = append(opts.Entities, value)
			idx = next
		default:
			return Options{}, domainerrors.New(domainerrors.CodeInvalidArgs, fmt.Sprintf("unknown graphql-help option: %s", name), nil)
		}
	}
	if len(opts.Entities) > 0 && !opts.SchemaOnly {
		return Options{}, domainerrors.New(domainerrors.CodeInvalidArgs, "--entity is allowed only with --schema-only", nil)
	}
	return opts, nil
}
