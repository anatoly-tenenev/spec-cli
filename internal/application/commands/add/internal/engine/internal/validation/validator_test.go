package validation

import (
	"reflect"
	"testing"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/model"
	schemaexpressions "github.com/anatoly-tenenev/spec-cli/internal/application/schema/expressions"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

func TestValidateMetaFieldValueDynamicConst(t *testing.T) {
	constTemplate := compileTemplate(t, "${refs.owner.slug}")
	fieldSpec := model.MetaField{
		Name:     "ownerSlug",
		Type:     "string",
		HasConst: true,
		Const: model.RuleValue{
			Literal:  "${refs.owner.slug}",
			Template: constTemplate,
		},
	}
	candidate := &model.Candidate{Type: "feature", ID: "FEAT-1", Slug: "retry-window"}

	tests := []struct {
		name         string
		value        any
		context      map[string]any
		expectedCode []string
	}{
		{
			name:  "happy_path",
			value: "alice",
			context: map[string]any{
				"refs": map[string]any{
					"owner": map[string]any{
						"slug": "alice",
					},
				},
			},
			expectedCode: nil,
		},
		{
			name:  "mismatch",
			value: "bob",
			context: map[string]any{
				"refs": map[string]any{
					"owner": map[string]any{
						"slug": "alice",
					},
				},
			},
			expectedCode: []string{"meta.required_value_mismatch"},
		},
		{
			name:  "interpolation_failure",
			value: "alice",
			context: map[string]any{
				"refs": map[string]any{
					"owner": nil,
				},
			},
			expectedCode: []string{"meta.required_const_interpolation_failed"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			issues := validateMetaFieldValue(fieldSpec, testCase.value, candidate, testCase.context)
			if got := issueCodes(issues); !reflect.DeepEqual(got, testCase.expectedCode) {
				t.Fatalf("unexpected issue codes: got=%v want=%v", got, testCase.expectedCode)
			}
		})
	}
}

func TestValidateMetaFieldValueDynamicEnum(t *testing.T) {
	enumTemplate := compileTemplate(t, "${refs.owner.slug}")
	fieldSpec := model.MetaField{
		Name: "status",
		Type: "string",
		Enum: []model.RuleValue{
			{Literal: "draft"},
			{Literal: "${refs.owner.slug}", Template: enumTemplate},
		},
	}
	candidate := &model.Candidate{Type: "feature", ID: "FEAT-1", Slug: "retry-window"}

	tests := []struct {
		name         string
		value        any
		context      map[string]any
		expectedCode []string
	}{
		{
			name:  "happy_path",
			value: "alice",
			context: map[string]any{
				"refs": map[string]any{
					"owner": map[string]any{
						"slug": "alice",
					},
				},
			},
			expectedCode: nil,
		},
		{
			name:  "mismatch",
			value: "archived",
			context: map[string]any{
				"refs": map[string]any{
					"owner": map[string]any{
						"slug": "alice",
					},
				},
			},
			expectedCode: []string{"meta.required_enum_mismatch"},
		},
		{
			name:  "interpolation_failure",
			value: "draft",
			context: map[string]any{
				"refs": map[string]any{
					"owner": nil,
				},
			},
			expectedCode: []string{"meta.required_enum_interpolation_failed"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			issues := validateMetaFieldValue(fieldSpec, testCase.value, candidate, testCase.context)
			if got := issueCodes(issues); !reflect.DeepEqual(got, testCase.expectedCode) {
				t.Fatalf("unexpected issue codes: got=%v want=%v", got, testCase.expectedCode)
			}
		})
	}
}

func TestValidateUsesCompilerOwnedRequiredPaths(t *testing.T) {
	requiredExpr := compileExpression(t, "contains(meta.trigger, 'x')")
	typeSpec := model.EntityTypeSpec{
		IDPrefix:       "FEAT",
		MetaFieldOrder: []string{"owner"},
		MetaFields: map[string]model.MetaField{
			"owner": {
				Name:         "owner",
				Type:         "string",
				RequiredExpr: requiredExpr,
				RequiredPath: "entity.feature.meta.fields.owner.required",
			},
		},
		SectionOrder: []string{"summary"},
		Sections: map[string]model.SectionSpec{
			"summary": {
				Name:         "summary",
				Titles:       []string{"Summary"},
				RequiredExpr: requiredExpr,
				RequiredPath: "entity.feature.content.sections.summary.required",
			},
		},
	}
	candidate := &model.Candidate{
		Type:        "feature",
		ID:          "FEAT-1",
		Slug:        "retry-window",
		CreatedDate: "2026-03-10",
		UpdatedDate: "2026-03-10",
		Frontmatter: map[string]any{
			"type":        "feature",
			"id":          "FEAT-1",
			"slug":        "retry-window",
			"createdDate": "2026-03-10",
			"updatedDate": "2026-03-10",
			"owner":       "team-core",
		},
		Meta: map[string]any{
			"owner": "team-core",
		},
		Body: "## Summary {#summary}\nBody",
	}

	issues := Validate(
		typeSpec,
		candidate,
		model.Snapshot{},
		nil,
		nil,
		map[string]any{
			"meta": map[string]any{
				"trigger": 1,
			},
		},
	)

	fieldsByCode := map[string]string{}
	for _, issue := range issues {
		fieldsByCode[issue.Code] = issue.Field
	}

	if got := fieldsByCode["meta.required_expression_evaluation_failed"]; got != "entity.feature.meta.fields.owner.required" {
		t.Fatalf("unexpected meta required path: %q", got)
	}
	if got := fieldsByCode["content.required_expression_evaluation_failed"]; got != "entity.feature.content.sections.summary.required" {
		t.Fatalf("unexpected content required path: %q", got)
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

func issueCodes(issues []domainvalidation.Issue) []string {
	if len(issues) == 0 {
		return nil
	}
	codes := make([]string, 0, len(issues))
	for _, issue := range issues {
		codes = append(codes, issue.Code)
	}
	return codes
}
