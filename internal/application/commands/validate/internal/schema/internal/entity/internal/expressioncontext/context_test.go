package expressioncontext

import (
	"testing"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/domain/reservedkeys"
)

func TestBuildEntityExpressionSchemaSeparatesMetaAndRefs(t *testing.T) {
	schema := BuildEntityExpressionSchema(
		[]model.RequiredFieldRule{
			{Name: "status", Type: "string", Required: true},
			{Name: "owner", Type: reservedkeys.SchemaTypeEntityRef, Required: true},
			{Name: "tags", Type: "array", HasItemType: true, ItemType: "string"},
		},
		map[string]MetaFieldConstraints{
			"status": {Enum: []any{"active", "archived"}},
		},
	)

	properties := schema["properties"].(map[string]any)

	metaSchema := properties["meta"].(map[string]any)
	metaProps := metaSchema["properties"].(map[string]any)
	metaRequired := metaSchema["required"].([]any)
	if _, exists := metaProps["status"]; !exists {
		t.Fatalf("expected meta.status in expression schema")
	}
	if _, exists := metaProps["owner"]; exists {
		t.Fatalf("did not expect meta.owner for entityRef field")
	}
	if _, exists := metaProps[reservedkeys.BuiltinID]; exists {
		t.Fatalf("did not expect built-in id under meta")
	}
	if !containsAny(metaRequired, "status") {
		t.Fatalf("expected required meta field 'status'")
	}
	if containsAny(metaRequired, "owner") {
		t.Fatalf("did not expect required meta field 'owner'")
	}

	refsSchema := properties["refs"].(map[string]any)
	refsProps := refsSchema["properties"].(map[string]any)
	if _, exists := refsProps["owner"]; !exists {
		t.Fatalf("expected refs.owner for entityRef field")
	}
	if _, exists := refsProps["status"]; exists {
		t.Fatalf("did not expect refs.status for non-reference field")
	}
}

func TestIsPathGuaranteedBySchemaForMetaContract(t *testing.T) {
	fieldsByName := map[string]model.RequiredFieldRule{
		"title": {
			Name:     "title",
			Type:     "string",
			Required: true,
		},
		"owner": {
			Name:     "owner",
			Type:     reservedkeys.SchemaTypeEntityRef,
			Required: true,
		},
	}

	if !IsPathGuaranteedBySchema("meta.title", fieldsByName) {
		t.Fatalf("expected meta.title to be guaranteed")
	}
	if IsPathGuaranteedBySchema("meta.owner", fieldsByName) {
		t.Fatalf("did not expect meta.owner to be guaranteed for entityRef")
	}
	if IsPathGuaranteedBySchema("meta.id", fieldsByName) {
		t.Fatalf("did not expect meta.id to be guaranteed")
	}
	if !IsPathGuaranteedBySchema("id", fieldsByName) {
		t.Fatalf("expected top-level id to be guaranteed")
	}
}

func containsAny(values []any, candidate string) bool {
	for _, value := range values {
		typed, ok := value.(string)
		if !ok {
			continue
		}
		if typed == candidate {
			return true
		}
	}
	return false
}
