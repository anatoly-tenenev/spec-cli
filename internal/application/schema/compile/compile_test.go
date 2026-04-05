package compile

import (
	"os"
	"path/filepath"
	"testing"

	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func TestCompilerCachesWithinProcess(t *testing.T) {
	schemaPath := writeSchema(t, "entity:\n  service:\n    idPrefix: SVC\n    pathTemplate: services/index.md\n")

	compiler := NewCompiler()
	first, firstErr := compiler.Compile(schemaPath, "spec.schema.yaml")
	if firstErr != nil {
		t.Fatalf("expected first compile app error to be nil, got %#v", firstErr)
	}
	if !first.Valid {
		t.Fatalf("expected first compile to be valid, got %#v", first.Issues)
	}

	if err := os.WriteFile(schemaPath, []byte("entity: []\n"), 0o644); err != nil {
		t.Fatalf("rewrite schema file: %v", err)
	}

	second, secondErr := compiler.Compile(schemaPath, "spec.schema.yaml")
	if secondErr != nil {
		t.Fatalf("expected second compile app error to be nil, got %#v", secondErr)
	}
	if !second.Valid {
		t.Fatalf("expected cached compile result, got invalid %#v", second.Issues)
	}
}

func TestCompilerAlwaysReturnsNonNilIssues(t *testing.T) {
	schemaPath := writeSchema(t, "version: \"1.0\"\nentity:\n  service:\n    idPrefix: SVC\n    pathTemplate: services/index.md\n")

	compiler := NewCompiler()
	result, compileErr := compiler.Compile(schemaPath, "spec.schema.yaml")
	if compileErr != nil {
		t.Fatalf("expected compile app error to be nil, got %#v", compileErr)
	}
	if !result.Valid {
		t.Fatalf("expected compile to be valid, got %#v", result.Issues)
	}
	if result.Issues == nil {
		t.Fatalf("expected non-nil issues slice for zero diagnostics")
	}
	if len(result.Issues) != 0 {
		t.Fatalf("expected zero issues, got %d", len(result.Issues))
	}
}

func TestCompilerClassifiesMissingSchemaAsNotFound(t *testing.T) {
	root := t.TempDir()
	missingPath := filepath.Join(root, "missing.schema.yaml")

	result, compileErr := NewCompiler().Compile(missingPath, "missing.schema.yaml")
	assertCompileErrorCode(t, compileErr, domainerrors.CodeSchemaNotFound)
	if result.Valid {
		t.Fatalf("expected invalid result for missing schema")
	}
}

func TestCompilerClassifiesUnreadablePathAsReadError(t *testing.T) {
	root := t.TempDir()

	result, compileErr := NewCompiler().Compile(root, "spec.schema.yaml")
	assertCompileErrorCode(t, compileErr, domainerrors.CodeSchemaReadError)
	if result.Valid {
		t.Fatalf("expected invalid result for unreadable schema path")
	}
}

func TestCompilerClassifiesEmptySchemaAsParseError(t *testing.T) {
	schemaPath := writeSchema(t, " \n\t\n")

	result, compileErr := NewCompiler().Compile(schemaPath, "spec.schema.yaml")
	assertCompileErrorCode(t, compileErr, domainerrors.CodeSchemaParseError)
	if result.Valid {
		t.Fatalf("expected invalid result for empty schema")
	}
}

func TestCompilerClassifiesSemanticSchemaErrorsAsSchemaInvalid(t *testing.T) {
	schemaPath := writeSchema(t, "entity:\n  service:\n    idPrefix: SVC\n")

	result, compileErr := NewCompiler().Compile(schemaPath, "spec.schema.yaml")
	assertCompileErrorCode(t, compileErr, domainerrors.CodeSchemaInvalid)
	if result.Valid {
		t.Fatalf("expected invalid result for semantic schema errors")
	}
}

func TestCompilerWarningsOnlyDoesNotReturnAppError(t *testing.T) {
	schemaPath := writeSchema(t, "entity:\n  service:\n    idPrefix: SVC\n    pathTemplate: services/index.md\n")

	result, compileErr := NewCompiler().Compile(schemaPath, "spec.schema.yaml")
	if compileErr != nil {
		t.Fatalf("expected nil compile app error for warning-only schema, got %#v", compileErr)
	}
	if !result.Valid {
		t.Fatalf("expected warning-only schema to be valid, got %#v", result.Issues)
	}
	if result.Summary.Warnings == 0 {
		t.Fatalf("expected warning-only schema to include warnings summary")
	}
}

func TestCompilerAllowsDescriptionAtLegalEntityMetaFieldAndSectionLevels(t *testing.T) {
	schemaPath := writeSchema(t, ""+
		"version: v1\n"+
		"entity:\n"+
		"  doc:\n"+
		"    idPrefix: DOC\n"+
		"    description: Entity description\n"+
		"    pathTemplate: docs/${slug}.md\n"+
		"    meta:\n"+
		"      fields:\n"+
		"        owner:\n"+
		"          schema:\n"+
		"            type: string\n"+
		"          description: Meta description\n"+
		"    content:\n"+
		"      sections:\n"+
		"        goal:\n"+
		"          description: Section description\n",
	)

	result, compileErr := NewCompiler().Compile(schemaPath, "spec.schema.yaml")
	if compileErr != nil {
		t.Fatalf("expected compile app error to be nil, got %#v", compileErr)
	}
	if !result.Valid {
		t.Fatalf("expected schema to be valid, got %#v", result.Issues)
	}

	entity, ok := result.Schema.Entities["doc"]
	if !ok {
		t.Fatalf("expected entity 'doc' to be compiled")
	}
	if entity.Description != "Entity description" {
		t.Fatalf("expected entity description to be preserved, got %q", entity.Description)
	}

	metaField, ok := entity.MetaFields["owner"]
	if !ok {
		t.Fatalf("expected meta field 'owner' to be compiled")
	}
	if metaField.Description != "Meta description" {
		t.Fatalf("expected meta field description to be preserved, got %q", metaField.Description)
	}

	section, ok := entity.Sections["goal"]
	if !ok {
		t.Fatalf("expected section 'goal' to be compiled")
	}
	if section.Description != "Section description" {
		t.Fatalf("expected section description to be preserved, got %q", section.Description)
	}
}

func TestCompilerRejectsIntegerConstFloatLiteral(t *testing.T) {
	schemaPath := writeSchema(t, ""+
		"version: v1\n"+
		"entity:\n"+
		"  service:\n"+
		"    idPrefix: SVC\n"+
		"    pathTemplate: services/index.md\n"+
		"    meta:\n"+
		"      fields:\n"+
		"        count:\n"+
		"          schema:\n"+
		"            type: integer\n"+
		"            const: 1.0\n",
	)

	result, compileErr := NewCompiler().Compile(schemaPath, "spec.schema.yaml")
	assertCompileErrorCode(t, compileErr, domainerrors.CodeSchemaInvalid)
	if result.Valid {
		t.Fatalf("expected invalid result for integer const float literal")
	}
	assertSchemaIssue(t, result, "schema.value.const_type_mismatch", "schema.entity.service.meta.fields.count.schema.const")
}

func TestCompilerRejectsIntegerEnumFloatLiteral(t *testing.T) {
	schemaPath := writeSchema(t, ""+
		"version: v1\n"+
		"entity:\n"+
		"  service:\n"+
		"    idPrefix: SVC\n"+
		"    pathTemplate: services/index.md\n"+
		"    meta:\n"+
		"      fields:\n"+
		"        count:\n"+
		"          schema:\n"+
		"            type: integer\n"+
		"            enum: [1.0]\n",
	)

	result, compileErr := NewCompiler().Compile(schemaPath, "spec.schema.yaml")
	assertCompileErrorCode(t, compileErr, domainerrors.CodeSchemaInvalid)
	if result.Valid {
		t.Fatalf("expected invalid result for integer enum float literal")
	}
	assertSchemaIssue(t, result, "schema.value.enum_type_mismatch", "schema.entity.service.meta.fields.count.schema.enum[0]")
}

func TestCompilerAllowsIntegerConstIntegerLiteral(t *testing.T) {
	schemaPath := writeSchema(t, ""+
		"version: v1\n"+
		"entity:\n"+
		"  service:\n"+
		"    idPrefix: SVC\n"+
		"    pathTemplate: services/index.md\n"+
		"    meta:\n"+
		"      fields:\n"+
		"        count:\n"+
		"          schema:\n"+
		"            type: integer\n"+
		"            const: 1\n",
	)

	result, compileErr := NewCompiler().Compile(schemaPath, "spec.schema.yaml")
	if compileErr != nil {
		t.Fatalf("expected nil compile app error, got %#v", compileErr)
	}
	if !result.Valid {
		t.Fatalf("expected integer const integer literal to be valid, got %#v", result.Issues)
	}
}

func assertCompileErrorCode(t *testing.T, compileErr *domainerrors.AppError, expectedCode domainerrors.Code) {
	t.Helper()
	if compileErr == nil {
		t.Fatalf("expected compile error code %s, got nil", expectedCode)
	}
	if compileErr.Code != expectedCode {
		t.Fatalf("expected compile error code %s, got %s", expectedCode, compileErr.Code)
	}
}

func assertSchemaIssue(t *testing.T, result Result, code string, path string) {
	t.Helper()
	for _, issue := range result.Issues {
		if issue.Code == code && issue.Path == path {
			return
		}
	}
	t.Fatalf("expected issue %s at path %s, got %#v", code, path, result.Issues)
}

func writeSchema(t *testing.T, content string) string {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "spec.schema.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write schema fixture: %v", err)
	}
	return path
}
