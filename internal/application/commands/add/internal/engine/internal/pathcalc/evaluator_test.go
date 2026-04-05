package pathcalc

import (
	"testing"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/model"
	schemaexpressions "github.com/anatoly-tenenev/spec-cli/internal/application/schema/expressions"
)

func TestEvaluateWhenEvaluationFailureUsesCompilerOwnedPath(t *testing.T) {
	whenExpr := compileExpression(t, "contains(meta.trigger, 'x')")
	typeSpec := model.EntityTypeSpec{
		PathPattern: model.PathPattern{
			Cases: []model.PathPatternCase{
				{
					Use:         "features/static.md",
					UseTemplate: compileTemplate(t, "features/static.md"),
					HasWhen:     true,
					WhenExpr:    whenExpr,
					WhenPath:    "entity.feature.pathTemplate.cases[0].when",
					UsePath:     "entity.feature.pathTemplate.cases[0].use",
				},
				{
					Use:         "features/${slug}.md",
					UseTemplate: compileTemplate(t, "features/${slug}.md"),
					HasWhen:     false,
					WhenPath:    "entity.feature.pathTemplate.cases[1].when",
					UsePath:     "entity.feature.pathTemplate.cases[1].use",
				},
			},
		},
	}
	candidate := &model.Candidate{Type: "feature", ID: "FEAT-1", Slug: "retry-window"}

	resolvedPath, issues := Evaluate(typeSpec, candidate, map[string]any{
		"slug": "retry-window",
		"meta": map[string]any{
			"trigger": 1,
		},
	})

	if resolvedPath != "features/retry-window.md" {
		t.Fatalf("unexpected resolved path: %q", resolvedPath)
	}
	if len(issues) != 1 {
		t.Fatalf("unexpected issues count: %d", len(issues))
	}
	if issues[0].Code != "instance.pathTemplate.when_evaluation_failed" {
		t.Fatalf("unexpected issue code: %q", issues[0].Code)
	}
	if issues[0].Field != "entity.feature.pathTemplate.cases[0].when" {
		t.Fatalf("unexpected issue field: %q", issues[0].Field)
	}
}

func TestEvaluateUseRenderFailureUsesCompilerOwnedPath(t *testing.T) {
	typeSpec := model.EntityTypeSpec{
		PathPattern: model.PathPattern{
			Cases: []model.PathPatternCase{
				{
					Use:         "${refs.owner.slug}/features/${slug}.md",
					UseTemplate: compileTemplate(t, "${refs.owner.slug}/features/${slug}.md"),
					HasWhen:     false,
					WhenPath:    "entity.feature.pathTemplate.cases[0].when",
					UsePath:     "entity.feature.pathTemplate.cases[0].use",
				},
			},
		},
	}
	candidate := &model.Candidate{Type: "feature", ID: "FEAT-1", Slug: "retry-window"}

	resolvedPath, issues := Evaluate(typeSpec, candidate, map[string]any{
		"slug": "retry-window",
		"refs": map[string]any{
			"owner": nil,
		},
	})

	if resolvedPath != "" {
		t.Fatalf("expected empty path on render error, got %q", resolvedPath)
	}
	if len(issues) != 1 {
		t.Fatalf("unexpected issues count: %d", len(issues))
	}
	if issues[0].Code != "instance.pathTemplate.placeholder_unresolved" {
		t.Fatalf("unexpected issue code: %q", issues[0].Code)
	}
	if issues[0].Field != "entity.feature.pathTemplate.cases[0].use" {
		t.Fatalf("unexpected issue field: %q", issues[0].Field)
	}
}

func compileTemplate(t *testing.T, raw string) *schemaexpressions.CompiledTemplate {
	t.Helper()

	engine := schemaexpressions.NewEngine()
	template, compileErr := schemaexpressions.CompileTemplate(raw, engine)
	if compileErr != nil {
		t.Fatalf("compile template %q: %#v", raw, compileErr)
	}
	return template
}

func compileExpression(t *testing.T, source string) *schemaexpressions.CompiledExpression {
	t.Helper()

	engine := schemaexpressions.NewEngine()
	expression, compileErr := engine.Compile(source, schemaexpressions.CompileModeScalar)
	if compileErr != nil {
		t.Fatalf("compile expression %q: %#v", source, compileErr)
	}
	return expression
}
