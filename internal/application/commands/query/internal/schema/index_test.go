package schema

import (
	"os"
	"path/filepath"
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

	if _, ok := index.EntityTypes["feature"]; !ok {
		t.Fatal("feature type is missing")
	}
	if _, ok := index.SelectorPaths["revision"]; !ok {
		t.Fatal("revision selector is missing")
	}
	if _, ok := index.SelectorPaths["refs.owner"]; !ok {
		t.Fatal("refs.owner selector is missing")
	}
	if _, ok := index.SelectorPaths["refs.owner.id"]; ok {
		t.Fatal("refs.owner.id selector must not be available in projection namespace")
	}
	if _, ok := index.SortFields["meta.score"]; !ok {
		t.Fatal("meta.score sort field is missing")
	}
	if _, ok := index.FilterFields["refs.owner.resolved"]; !ok {
		t.Fatal("refs.owner.resolved filter field is missing")
	}
	if _, ok := index.SortFields["refs.owner.resolved"]; !ok {
		t.Fatal("refs.owner.resolved sort field is missing")
	}
	if _, ok := index.SelectorPaths["meta.owner"]; ok {
		t.Fatal("meta.owner selector must not be available for entityRef")
	}
	if _, ok := index.FilterFields["content.sections.summary"]; !ok {
		t.Fatal("content.sections.summary filter field is missing")
	}
	if _, ok := index.FilterFields["content.raw"]; ok {
		t.Fatal("content.raw must not be available as where-json filter field")
	}
	if _, ok := index.SortFields["content.raw"]; !ok {
		t.Fatal("content.raw sort field is missing")
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

func TestBuildIndex_ConflictingFieldTypeAcrossEntities(t *testing.T) {
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

	_, err := BuildIndex(loaded)
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if err.Code != domainerrors.CodeSchemaInvalid {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}
