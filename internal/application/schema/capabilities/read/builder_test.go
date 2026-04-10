package read

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	schemacompile "github.com/anatoly-tenenev/spec-cli/internal/application/schema/compile"
)

func TestBuild_ProjectsReadSemanticsAndNormalizesRefs(t *testing.T) {
	schemaText := `version: "0.0.7"
entity:
  service:
    idPrefix: SVC
    pathTemplate: "services/{slug}.md"
    meta:
      fields:
        status:
          required: true
          schema:
            type: string
            enum: [active, deprecated]
        frozen:
          required: false
          schema:
            type: string
            const: active
        releaseDate:
          required: "${meta.status == 'active'}"
          schema:
            type: string
            format: date
        owner:
          schema:
            type: entityRef
            refType: service
        watchers:
          schema:
            type: array
            items:
              type: entityRef
    content:
      sections:
        summary:
          required: true
        optional:
          required: false
        conditional:
          required: "${meta.status == 'active'}"
  feature:
    idPrefix: FEAT
    pathTemplate: "features/{slug}.md"
    meta:
      fields:
        title:
          schema:
            type: string
`

	capability := buildCapabilityFromSchema(t, schemaText)

	service, ok := capability.EntityTypes["service"]
	if !ok {
		t.Fatal("service entity type is missing")
	}

	if _, exists := service.MetaFields["owner"]; exists {
		t.Fatal("scalar entityRef must not appear in MetaFields")
	}
	if _, exists := service.MetaFields["watchers"]; exists {
		t.Fatal("array entityRef must not appear in MetaFields")
	}

	releaseDate, ok := service.MetaFields["releaseDate"]
	if !ok {
		t.Fatal("releaseDate meta field is missing")
	}
	if releaseDate.Kind != FieldKindDate {
		t.Fatalf("expected releaseDate kind %q, got %q", FieldKindDate, releaseDate.Kind)
	}
	if releaseDate.Required {
		t.Fatal("conditional required must be treated as non-required on read side")
	}

	status := service.MetaFields["status"]
	if !status.Required {
		t.Fatal("status must be required")
	}
	if !reflect.DeepEqual(status.EnumValues, []any{"active", "deprecated"}) {
		t.Fatalf("unexpected enum values: %#v", status.EnumValues)
	}

	frozen := service.MetaFields["frozen"]
	if frozen.Required {
		t.Fatal("frozen must be non-required")
	}
	if !frozen.HasConst || frozen.ConstValue != "active" {
		t.Fatalf("unexpected const projection: %#v", frozen)
	}

	ownerRef, ok := service.RefFields["owner"]
	if !ok {
		t.Fatal("owner ref field is missing")
	}
	if ownerRef.Cardinality != RefCardinalityScalar {
		t.Fatalf("expected scalar cardinality, got %q", ownerRef.Cardinality)
	}
	if !reflect.DeepEqual(ownerRef.AllowedTypes, []string{"service"}) {
		t.Fatalf("unexpected owner allowed types: %#v", ownerRef.AllowedTypes)
	}

	watchersRef, ok := service.RefFields["watchers"]
	if !ok {
		t.Fatal("watchers ref field is missing")
	}
	if watchersRef.Cardinality != RefCardinalityArray {
		t.Fatalf("expected array cardinality, got %q", watchersRef.Cardinality)
	}
	if !reflect.DeepEqual(watchersRef.AllowedTypes, []string{"feature", "service"}) {
		t.Fatalf("unexpected watchers allowed types: %#v", watchersRef.AllowedTypes)
	}

	if !service.Sections["summary"].Required {
		t.Fatal("summary section must be required")
	}
	if service.Sections["optional"].Required {
		t.Fatal("optional section must be non-required")
	}
	if service.Sections["conditional"].Required {
		t.Fatal("conditional section must be non-required on read side")
	}
}

func TestBuild_DeterministicProjection(t *testing.T) {
	schemaText := `version: "0.0.7"
entity:
  zeta:
    idPrefix: ZETA
    pathTemplate: "zeta/{slug}.md"
    meta:
      fields:
        target:
          schema:
            type: entityRef
  alpha:
    idPrefix: ALPHA
    pathTemplate: "alpha/{slug}.md"
    meta:
      fields:
        title:
          schema:
            type: string
`

	compiled := compileSchema(t, schemaText)
	first := Build(compiled.Schema)
	second := Build(compiled.Schema)

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("read capability projection must be deterministic:\nfirst=%#v\nsecond=%#v", first, second)
	}
}

func TestBuild_ProjectsOnlyStaticLiteralConstraints(t *testing.T) {
	schemaText := `version: "0.0.7"
entity:
  doc:
    idPrefix: DOC
    pathTemplate: "docs/{slug}.md"
    meta:
      fields:
        dynamicConst:
          schema:
            type: string
            const: "${slug}"
        dynamicEnum:
          schema:
            type: string
            enum: ["${slug}"]
        mixedEnum:
          schema:
            type: string
            enum: ["${slug}", "fixed"]
        staticConst:
          schema:
            type: string
            const: "fixed"
        staticEnum:
          schema:
            type: string
            enum: ["fixed", "stable"]
`

	capability := buildCapabilityFromSchema(t, schemaText)
	docType := capability.EntityTypes["doc"]

	dynamicConst := docType.MetaFields["dynamicConst"]
	if dynamicConst.HasConst {
		t.Fatalf("dynamic string const must not be projected as static const: %#v", dynamicConst)
	}
	if dynamicConst.ConstValue != nil {
		t.Fatalf("dynamic string const value must be nil on read side, got %#v", dynamicConst.ConstValue)
	}

	dynamicEnum := docType.MetaFields["dynamicEnum"]
	if len(dynamicEnum.EnumValues) != 0 {
		t.Fatalf("dynamic string enum must not be projected as static enum: %#v", dynamicEnum.EnumValues)
	}

	mixedEnum := docType.MetaFields["mixedEnum"]
	if len(mixedEnum.EnumValues) != 0 {
		t.Fatalf("mixed enum with interpolation must not be projected as static enum: %#v", mixedEnum.EnumValues)
	}

	staticConst := docType.MetaFields["staticConst"]
	if !staticConst.HasConst || staticConst.ConstValue != "fixed" {
		t.Fatalf("static const must stay projected: %#v", staticConst)
	}

	staticEnum := docType.MetaFields["staticEnum"]
	if !reflect.DeepEqual(staticEnum.EnumValues, []any{"fixed", "stable"}) {
		t.Fatalf("static enum must stay projected: %#v", staticEnum.EnumValues)
	}
}

func buildCapabilityFromSchema(t *testing.T, schemaText string) Capability {
	t.Helper()
	compiled := compileSchema(t, schemaText)
	return Build(compiled.Schema)
}

func compileSchema(t *testing.T, schemaText string) schemacompile.Result {
	t.Helper()

	path := filepath.Join(t.TempDir(), "spec.schema.yaml")
	if err := os.WriteFile(path, []byte(schemaText), 0o644); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	compileResult, compileErr := schemacompile.NewCompiler().Compile(path, path)
	if compileErr != nil {
		t.Fatalf("unexpected compile error: %v", compileErr)
	}
	if !compileResult.Valid {
		t.Fatalf("expected valid schema, got issues: %#v", compileResult.Issues)
	}
	return compileResult
}
