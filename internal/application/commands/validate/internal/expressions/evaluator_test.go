package expressions

import "testing"

type testContext struct {
	values map[string]any
}

func (context testContext) ResolveReference(reference Reference) (any, bool) {
	value, exists := context.values[reference.Raw]
	return value, exists
}

func TestEvaluate_EqStrictMissing(t *testing.T) {
	expression := &Expression{
		Operator: OpEq,
		Operands: []Operand{{Reference: &Reference{Kind: ReferenceMeta, Field: "status", Raw: "meta.status"}}, {Literal: "approved"}},
	}

	result, evalErr := Evaluate(expression, testContext{values: map[string]any{}})
	if evalErr == nil {
		t.Fatalf("expected evaluation error")
	}
	if evalErr.Code != "instance.expression.missing_operand_strict" {
		t.Fatalf("unexpected error code: %s", evalErr.Code)
	}
	if result {
		t.Fatalf("expected false result")
	}
}

func TestEvaluate_EqSafeMissing(t *testing.T) {
	expression := &Expression{
		Operator: OpEqSafe,
		Operands: []Operand{{Reference: &Reference{Kind: ReferenceMeta, Field: "status", Raw: "meta.status"}}, {Literal: "approved"}},
	}

	result, evalErr := Evaluate(expression, testContext{values: map[string]any{}})
	if evalErr != nil {
		t.Fatalf("did not expect evaluation error: %v", evalErr)
	}
	if result {
		t.Fatalf("expected false result")
	}
}

func TestEvaluate_InStrictMissing(t *testing.T) {
	expression := &Expression{
		Operator: OpIn,
		Operands: []Operand{{Reference: &Reference{Kind: ReferenceMeta, Field: "status", Raw: "meta.status"}}},
		ListOperands: []Operand{
			{Literal: "approved"},
			{Reference: &Reference{Kind: ReferenceMeta, Field: "missing", Raw: "meta.missing"}},
		},
	}

	result, evalErr := Evaluate(expression, testContext{values: map[string]any{"meta.status": "draft"}})
	if evalErr == nil {
		t.Fatalf("expected strict missing error")
	}
	if evalErr.Code != "instance.expression.missing_operand_strict" {
		t.Fatalf("unexpected error code: %s", evalErr.Code)
	}
	if result {
		t.Fatalf("expected false result")
	}
}

func TestEvaluate_InSafeMissing(t *testing.T) {
	expression := &Expression{
		Operator: OpInSafe,
		Operands: []Operand{{Reference: &Reference{Kind: ReferenceMeta, Field: "status", Raw: "meta.status"}}},
		ListOperands: []Operand{
			{Literal: "approved"},
			{Reference: &Reference{Kind: ReferenceMeta, Field: "missing", Raw: "meta.missing"}},
		},
	}

	result, evalErr := Evaluate(expression, testContext{values: map[string]any{"meta.status": "draft"}})
	if evalErr != nil {
		t.Fatalf("did not expect error for in? with missing operand: %v", evalErr)
	}
	if result {
		t.Fatalf("expected false result")
	}
}
