package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func TestLoadAndBuildIndex_FromStandardSchema(t *testing.T) {
	schemaText := `version: "0.0.4"
entity:
  service:
    idPrefix: SVC
    pathTemplate: "services/{slug}.md"
    meta:
      fields:
        status:
          schema:
            type: string
            enum: [active, deprecated]
    content:
      sections:
        summary: {}
  feature:
    idPrefix: FEAT
    pathTemplate: "features/{slug}.md"
    meta:
      fields:
        status:
          schema:
            type: string
            enum: [draft, active]
        owner:
          schema:
            type: entityRef
            refTypes: [service]
        score:
          schema:
            type: number
    content:
      sections:
        summary: {}
        implementation: {}
`

	path := filepath.Join(t.TempDir(), "spec.schema.yaml")
	if err := os.WriteFile(path, []byte(schemaText), 0o644); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	loaded, loadErr := Load(path)
	if loadErr != nil {
		t.Fatalf("unexpected load error: %v", loadErr)
	}

	index, indexErr := BuildIndex(loaded)
	if indexErr != nil {
		t.Fatalf("unexpected index error: %v", indexErr)
	}

	feature, ok := index.EntityTypes["feature"]
	if !ok {
		t.Fatal("feature type is missing")
	}
	if _, ok := feature.MetaFields["status"]; !ok {
		t.Fatal("feature.meta.status is missing")
	}
	if _, ok := feature.MetaFields["owner"]; ok {
		t.Fatal("feature.meta.owner must not be in non-ref meta fields")
	}

	ownerRef, ok := feature.RefFields["owner"]
	if !ok {
		t.Fatal("feature.refs.owner is missing")
	}
	if ownerRef.Cardinality != model.RefCardinalityScalar {
		t.Fatalf("unexpected ref cardinality: %s", ownerRef.Cardinality)
	}
	if len(ownerRef.RefTypes) != 1 || ownerRef.RefTypes[0] != "service" {
		t.Fatalf("unexpected refTypes: %#v", ownerRef.RefTypes)
	}

	if _, ok := feature.SectionFields["summary"]; !ok {
		t.Fatal("feature section 'summary' is missing")
	}
	if _, ok := feature.SectionFields["implementation"]; !ok {
		t.Fatal("feature section 'implementation' is missing")
	}
}

func TestLoad_RejectsMissingEntity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spec.schema.yaml")
	if err := os.WriteFile(path, []byte("version: \"0.0.4\"\nmodel: {}\n"), 0o644); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	_, loadErr := Load(path)
	if loadErr == nil {
		t.Fatal("expected load error")
	}
	if loadErr.Code != domainerrors.CodeSchemaInvalid {
		t.Fatalf("unexpected error code: %s", loadErr.Code)
	}
}

func TestLoad_RejectsInvalidRequiredInterpolation(t *testing.T) {
	schemaText := `version: "0.0.4"
entity:
  feature:
    meta:
      fields:
        priority:
          required: "${refs.owner.slug"
          schema:
            type: string
`

	path := filepath.Join(t.TempDir(), "spec.schema.yaml")
	if err := os.WriteFile(path, []byte(schemaText), 0o644); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	_, loadErr := Load(path)
	if loadErr == nil {
		t.Fatal("expected load error")
	}
	if loadErr.Code != domainerrors.CodeSchemaInvalid {
		t.Fatalf("unexpected error code: %s", loadErr.Code)
	}
	if !strings.Contains(loadErr.Message, "has invalid expression in required context") {
		t.Fatalf("unexpected error message: %s", loadErr.Message)
	}
}

func TestLoad_RejectsObjectMetadataFieldType(t *testing.T) {
	schemaText := `version: "0.0.4"
entity:
  feature:
    meta:
      fields:
        payload:
          schema:
            type: object
`

	path := filepath.Join(t.TempDir(), "spec.schema.yaml")
	if err := os.WriteFile(path, []byte(schemaText), 0o644); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	_, loadErr := Load(path)
	if loadErr == nil {
		t.Fatal("expected load error")
	}
	if loadErr.Code != domainerrors.CodeSchemaInvalid {
		t.Fatalf("unexpected error code: %s", loadErr.Code)
	}
}

func TestBuildIndex_AllowsConflictingMetaKindsAcrossTypes(t *testing.T) {
	loaded := LoadedSchema{EntityTypes: map[string]EntityType{
		"a": {
			Name: "a",
			MetadataFields: map[string]Field{
				"rank": {Name: "rank", Kind: model.FieldKindNumber},
			},
		},
		"b": {
			Name: "b",
			MetadataFields: map[string]Field{
				"rank": {Name: "rank", Kind: model.FieldKindString},
			},
		},
	}}

	index, err := BuildIndex(loaded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := index.EntityTypes["a"]; !ok {
		t.Fatal("type a is missing")
	}
	if _, ok := index.EntityTypes["b"]; !ok {
		t.Fatal("type b is missing")
	}
}
