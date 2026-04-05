package engine

import (
	"testing"

	schemacapvalidate "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/validate"
	"github.com/anatoly-tenenev/spec-cli/internal/domain/reservedkeys"
)

func TestBuildRuntimeEvaluationContextNormalizesMetaAndRefs(t *testing.T) {
	context := buildRuntimeEvaluationContext(
		map[string]any{
			reservedkeys.BuiltinType:        "doc",
			reservedkeys.BuiltinID:          "DOC-1",
			reservedkeys.BuiltinSlug:        "overview",
			reservedkeys.BuiltinCreatedDate: "2026-01-01",
			reservedkeys.BuiltinUpdatedDate: "2026-01-02",
			"title":                         "Overview",
			"owner":                         "SRV-1",
			"extra":                         "must-not-leak",
		},
		map[string]resolvedEntityRef{
			"owner": {
				ID:      "SRV-1",
				Type:    "service",
				Slug:    "catalog",
				DirPath: "services/catalog",
			},
		},
		schemacapvalidate.EntityValidationModel{
			RequiredFields: []schemacapvalidate.RequiredFieldRule{
				{Name: "title", Type: "string"},
				{Name: "owner", Type: reservedkeys.SchemaTypeEntityRef},
			},
		},
	)

	meta := context["meta"].(map[string]any)
	if value := meta["title"]; value != "Overview" {
		t.Fatalf("unexpected meta.title: %#v", value)
	}
	if _, exists := meta["owner"]; exists {
		t.Fatalf("did not expect entityRef field under meta")
	}
	if _, exists := meta["extra"]; exists {
		t.Fatalf("did not expect unknown raw frontmatter field under meta")
	}
	if _, exists := meta[reservedkeys.BuiltinID]; exists {
		t.Fatalf("did not expect built-in id under meta")
	}

	refs := context["refs"].(map[string]any)
	ownerRef, ok := refs["owner"].(map[string]any)
	if !ok {
		t.Fatalf("expected refs.owner object, got %#v", refs["owner"])
	}
	if ownerRef["id"] != "SRV-1" || ownerRef["type"] != "service" || ownerRef["slug"] != "catalog" || ownerRef["dirPath"] != "services/catalog" {
		t.Fatalf("unexpected refs.owner payload: %#v", ownerRef)
	}
}

func TestBuildRuntimeEvaluationContextKeepsUnresolvedRefsAsNull(t *testing.T) {
	context := buildRuntimeEvaluationContext(
		map[string]any{
			reservedkeys.BuiltinType: "doc",
		},
		nil,
		schemacapvalidate.EntityValidationModel{
			RequiredFields: []schemacapvalidate.RequiredFieldRule{
				{Name: "owner", Type: reservedkeys.SchemaTypeEntityRef},
			},
		},
	)

	for _, key := range []string{
		reservedkeys.BuiltinType,
		reservedkeys.BuiltinID,
		reservedkeys.BuiltinSlug,
		reservedkeys.BuiltinCreatedDate,
		reservedkeys.BuiltinUpdatedDate,
		"meta",
		"refs",
	} {
		if _, exists := context[key]; !exists {
			t.Fatalf("expected key %q in runtime context", key)
		}
	}

	refs := context["refs"].(map[string]any)
	if value, exists := refs["owner"]; !exists || value != nil {
		t.Fatalf("expected refs.owner to be null for unresolved ref, got %#v (exists=%v)", value, exists)
	}
}
