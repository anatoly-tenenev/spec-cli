package pathpattern

import (
	"strings"
	"testing"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/expressions"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	expressioncontext "github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/schema/internal/entity/internal/expressioncontext"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	"github.com/anatoly-tenenev/spec-cli/internal/domain/reservedkeys"
)

func TestParsePrioritizesWhenContextErrorOverUseGuardAnalysis(t *testing.T) {
	rules := []model.RequiredFieldRule{
		{Name: "owner", Type: reservedkeys.SchemaTypeEntityRef},
	}
	constraints := map[string]expressioncontext.MetaFieldConstraints{
		"owner": {},
	}
	engine, compileErr := expressions.NewSchemaAwareEngine(
		"doc",
		expressioncontext.BuildEntityExpressionSchema(rules, constraints),
	)
	if compileErr != nil {
		t.Fatalf("failed to build expression engine: %+v", compileErr)
	}

	pathRule := map[string]any{
		"cases": []any{
			map[string]any{
				"when": "${meta.owner != `null`}",
				"use":  "docs/${refs.owner.id}/${slug}.md",
			},
			map[string]any{
				"use": "${slug}.md",
			},
		},
	}

	fieldsByName := map[string]model.RequiredFieldRule{
		"owner": rules[0],
	}

	_, _, parseErr := Parse("doc", pathRule, engine, fieldsByName)
	if parseErr == nil {
		t.Fatalf("expected parse error")
	}
	if parseErr.Code != domainerrors.CodeSchemaInvalid {
		t.Fatalf("unexpected parse error code: %s", parseErr.Code)
	}
	if !strings.Contains(parseErr.Message, "pathTemplate.cases[0].when has invalid expression in when context") {
		t.Fatalf("unexpected parse error message: %s", parseErr.Message)
	}
	if strings.Contains(parseErr.Message, "use_missing_guard") {
		t.Fatalf("when compile error must be primary, got: %s", parseErr.Message)
	}
}
