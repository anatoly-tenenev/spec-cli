package expressions

import (
	"testing"

	jmespath "github.com/anatoly-tenenev/go-jmespath"
)

func TestSchemaAwareCompileRequiredAndWhen(t *testing.T) {
	engine := mustNewSchemaAwareEngine(t, "doc")

	requiredExpr, requiredErr := CompileScalarInterpolation("${meta.status == 'active'}", engine)
	if requiredErr != nil {
		t.Fatalf("required compile failed: %+v", requiredErr)
	}
	if requiredExpr == nil {
		t.Fatalf("expected required expression")
	}

	whenExpr, whenErr := CompileScalarInterpolation("${refs.owner != `null`}", engine)
	if whenErr != nil {
		t.Fatalf("when compile failed: %+v", whenErr)
	}
	if whenExpr == nil {
		t.Fatalf("expected when expression")
	}
}

func TestCompileMapsUnknownPropertyToStaticCode(t *testing.T) {
	engine := mustNewSchemaAwareEngine(t, "doc")

	_, compileErr := CompileScalarInterpolation("${meta.unknown}", engine)
	if compileErr == nil {
		t.Fatalf("expected compile error")
	}
	if compileErr.Code != "schema.expression.unknown_property" {
		t.Fatalf("unexpected compile error code: %s", compileErr.Code)
	}
}

func TestCompileMapsInvalidFunctionArgTypeToStaticCode(t *testing.T) {
	engine := mustNewSchemaAwareEngine(t, "doc")

	_, compileErr := CompileScalarInterpolation("${abs(meta.status)}", engine)
	if compileErr == nil {
		t.Fatalf("expected compile error")
	}
	if compileErr.Code != "schema.expression.invalid_function_arg_type" {
		t.Fatalf("unexpected compile error code: %s", compileErr.Code)
	}
}

func TestInferTypeBooleanCompatibleForWhen(t *testing.T) {
	engine := mustNewSchemaAwareEngine(t, "doc")

	expression, compileErr := CompileScalarInterpolation("${meta.status == 'active'}", engine)
	if compileErr != nil {
		t.Fatalf("compile failed: %+v", compileErr)
	}
	inferred := expression.InferredType()
	if inferred == nil {
		t.Fatalf("expected inferred type")
	}
	if !inferred.MayBeBoolean() {
		t.Fatalf("expected boolean-compatible inferred type")
	}
}

func TestInferTypeRejectsDeterministicObjectArrayNullForTemplate(t *testing.T) {
	engine := mustNewSchemaAwareEngine(t, "doc")

	testCases := []struct {
		name string
		raw  string
	}{
		{name: "object", raw: "${meta.payload}"},
		{name: "array", raw: "${meta.tags}"},
		{name: "null", raw: "${`null`}"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, compileErr := CompileTemplate(tc.raw, engine)
			if compileErr == nil {
				t.Fatalf("expected compile error")
			}
			if compileErr.Code != "schema.interpolation.invalid_result_type" {
				t.Fatalf("unexpected compile error code: %s", compileErr.Code)
			}
		})
	}
}

func TestGuardAnalysisProtectsWhenTrue(t *testing.T) {
	engine := mustNewSchemaAwareEngine(t, "doc")

	expression, compileErr := CompileScalarInterpolation("${refs.owner && refs.owner.type == 'service'}", engine)
	if compileErr != nil {
		t.Fatalf("compile failed: %+v", compileErr)
	}
	if !expression.ProtectsWhenTrue("refs.owner") {
		t.Fatalf("expected refs.owner to be protected when expression is true")
	}
}

func TestCompileUsesCacheBySourceAndMode(t *testing.T) {
	engine := mustNewSchemaAwareEngine(t, "doc")

	first, firstErr := engine.Compile("meta.status", CompileModeScalar)
	if firstErr != nil {
		t.Fatalf("first compile failed: %+v", firstErr)
	}
	second, secondErr := engine.Compile("meta.status", CompileModeScalar)
	if secondErr != nil {
		t.Fatalf("second compile failed: %+v", secondErr)
	}
	if first.query != second.query {
		t.Fatalf("expected cached compiled expression to be reused")
	}

	third, thirdErr := engine.Compile("meta.status", CompileModeTemplatePart)
	if thirdErr != nil {
		t.Fatalf("third compile failed: %+v", thirdErr)
	}
	if first.query == third.query {
		t.Fatalf("expected mode-specific cache entries")
	}
}

func TestEvaluateMapsLibraryErrors(t *testing.T) {
	engine := NewEngine()
	expression, compileErr := engine.Compile("length(meta.value)", CompileModeScalar)
	if compileErr != nil {
		t.Fatalf("compile failed: %+v", compileErr)
	}

	_, evalErr := Evaluate(expression, map[string]any{"meta": map[string]any{"value": true}})
	if evalErr == nil {
		t.Fatalf("expected evaluation error")
	}
	if evalErr.Code != "instance.expression.evaluation_failed" {
		t.Fatalf("unexpected evaluation error code: %s", evalErr.Code)
	}
}

func mustNewSchemaAwareEngine(t *testing.T, entityType string) *Engine {
	t.Helper()

	engine, compileErr := NewSchemaAwareEngine(entityType, testExpressionSchema())
	if compileErr != nil {
		t.Fatalf("failed to build schema-aware engine: %+v", compileErr)
	}
	return engine
}

func testExpressionSchema() jmespath.JSONSchema {
	return jmespath.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"type":        map[string]any{"type": "string"},
			"id":          map[string]any{"type": "string"},
			"slug":        map[string]any{"type": "string"},
			"createdDate": map[string]any{"type": "string"},
			"updatedDate": map[string]any{"type": "string"},
			"meta": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"status":  map[string]any{"type": "string", "enum": []any{"active", "archived"}},
					"owner":   map[string]any{"type": "string"},
					"payload": map[string]any{"type": "object", "properties": map[string]any{"x": map[string]any{"type": "number"}}, "additionalProperties": false},
					"tags":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				},
				"additionalProperties": false,
			},
			"refs": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"owner": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id":      map[string]any{"type": "string"},
							"type":    map[string]any{"type": "string"},
							"slug":    map[string]any{"type": "string"},
							"dirPath": map[string]any{"type": "string"},
						},
						"additionalProperties": false,
					},
				},
				"additionalProperties": false,
			},
		},
		"additionalProperties": false,
	}
}
