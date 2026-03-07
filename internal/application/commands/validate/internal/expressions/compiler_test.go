package expressions

import "testing"

func TestCompile_Success(t *testing.T) {
	context := CompileContext{MetaFields: map[string]MetaFieldSpec{
		"status": {Type: "string", Comparable: true},
		"owner":  {Type: "entity_ref", Comparable: true, EntityRef: true},
	}}

	expression, issues := Compile(map[string]any{
		"all": []any{
			map[string]any{"eq": []any{"meta.status", "approved"}},
			map[string]any{"eq?": []any{"ref.owner.type", "service"}},
		},
	}, "schema.entity.doc.meta.fields[0].required_when", context)
	if len(issues) > 0 {
		t.Fatalf("expected no compile issues, got: %+v", issues)
	}
	if expression == nil {
		t.Fatalf("expected compiled expression")
	}
}

func TestCompile_InvalidOperator(t *testing.T) {
	context := CompileContext{MetaFields: map[string]MetaFieldSpec{"status": {Type: "string", Comparable: true}}}

	expression, issues := Compile(map[string]any{"unknown": []any{"meta.status", "approved"}}, "schema.path", context)
	if expression != nil {
		t.Fatalf("expected nil expression for invalid operator")
	}
	if len(issues) != 1 {
		t.Fatalf("expected single compile issue, got %d", len(issues))
	}
	if issues[0].Code != "schema.expression.invalid_operator" {
		t.Fatalf("unexpected issue code: %s", issues[0].Code)
	}
}

func TestCompile_InvalidArity(t *testing.T) {
	context := CompileContext{MetaFields: map[string]MetaFieldSpec{"status": {Type: "string", Comparable: true}}}

	expression, issues := Compile(map[string]any{"eq": []any{"meta.status"}}, "schema.path", context)
	if expression != nil {
		t.Fatalf("expected nil expression for invalid arity")
	}
	if len(issues) != 1 {
		t.Fatalf("expected single issue, got %d", len(issues))
	}
	if issues[0].Code != "schema.expression.invalid_arity" {
		t.Fatalf("unexpected issue code: %s", issues[0].Code)
	}
}

func TestCompile_InvalidReference(t *testing.T) {
	context := CompileContext{MetaFields: map[string]MetaFieldSpec{"status": {Type: "string", Comparable: true}}}

	expression, issues := Compile(map[string]any{"eq": []any{"meta.unknown", "approved"}}, "schema.path", context)
	if expression != nil {
		t.Fatalf("expected nil expression for invalid reference")
	}
	if len(issues) != 1 {
		t.Fatalf("expected single issue, got %d", len(issues))
	}
	if issues[0].Code != "schema.expression.invalid_reference" {
		t.Fatalf("unexpected issue code: %s", issues[0].Code)
	}
}
