// Package rulevalues holds the schema values that are not final until an
// entity exists. A const or an enum entry may be written as a template, and a
// template is only a value once it has been rendered against a concrete
// candidate, so the declaration has to travel as literal-or-template and be
// resolved at the point of use.
//
// The write and validate capabilities project the same declaration - one to
// say what may be written, the other to say what is demanded - and add,
// update and validate then resolve it against the entity in hand. That is one
// question, not three: a const the write side renders to "spec-7" and the
// validate side renders to something else would let a document be written and
// immediately be found non-conforming. One type and one resolver here keep
// the answer single.
package rulevalues

import (
	schemaexpressions "github.com/anatoly-tenenev/spec-cli/internal/application/schema/expressions"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
)

// RuleValue is a schema-declared value: either a literal the author wrote out,
// or a template to render against a candidate.
type RuleValue struct {
	Literal  any
	Template *schemaexpressions.CompiledTemplate
}

// FromLiteral projects one IR literal into the form the capabilities carry.
func FromLiteral(literal model.Literal) RuleValue {
	return RuleValue{Literal: literal.Value, Template: literal.Template}
}

// FromLiterals projects a list of IR literals, keeping declaration order.
// It returns nil for an empty list, so "no enum declared" stays
// distinguishable from "an enum with no members".
func FromLiterals(literals []model.Literal) []RuleValue {
	if len(literals) == 0 {
		return nil
	}

	values := make([]RuleValue, 0, len(literals))
	for _, literal := range literals {
		values = append(values, FromLiteral(literal))
	}
	return values
}

// Resolve renders a value against the evaluation context. A literal is
// returned unchanged; a template that fails to render yields the error rather
// than a partial string, because a half-rendered value would be judged as if
// the author had written it.
func Resolve(value RuleValue, context map[string]any) (any, *schemaexpressions.EvalError) {
	if value.Template == nil {
		return value.Literal, nil
	}

	rendered, renderErr := schemaexpressions.RenderTemplate(value.Template, context)
	if renderErr != nil {
		return nil, renderErr
	}

	return rendered, nil
}

// ResolveAll renders every value, stopping at the first failure. Callers
// report one interpolation failure per field, so there is nothing to gain from
// resolving the rest.
func ResolveAll(values []RuleValue, context map[string]any) ([]any, *schemaexpressions.EvalError) {
	if len(values) == 0 {
		return nil, nil
	}

	resolved := make([]any, 0, len(values))
	for _, value := range values {
		resolvedValue, resolveErr := Resolve(value, context)
		if resolveErr != nil {
			return nil, resolveErr
		}
		resolved = append(resolved, resolvedValue)
	}

	return resolved, nil
}
