package validate_test

import (
	"reflect"
	"strings"
	"testing"

	metafields "github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/schema/internal/entity/internal/metafields"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func TestMetafieldsParseAcceptsItemsRefTypesForEntityRefItems(t *testing.T) {
	rules, _, parseErr := metafields.Parse(
		"feature",
		map[string]any{
			"fields": map[string]any{
				"owners": map[string]any{
					"required": false,
					"schema": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":     "entityRef",
							"refTypes": []any{"service", "domain"},
						},
					},
				},
			},
		},
		map[string]struct{}{
			"feature": {},
			"service": {},
			"domain":  {},
		},
	)
	if parseErr != nil {
		t.Fatalf("unexpected parse error: %v", parseErr)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	rule := rules[0]
	if !rule.HasItemType {
		t.Fatalf("expected HasItemType=true")
	}
	if rule.ItemType != "entityRef" {
		t.Fatalf("expected ItemType=entityRef, got %s", rule.ItemType)
	}

	expectedRefTypes := []string{"domain", "service"}
	if !reflect.DeepEqual(rule.ItemRefTypes, expectedRefTypes) {
		t.Fatalf("expected sorted item refTypes %v, got %v", expectedRefTypes, rule.ItemRefTypes)
	}
}

func TestMetafieldsParseRejectsItemsRefTypesForNonEntityRefItems(t *testing.T) {
	_, _, parseErr := metafields.Parse(
		"feature",
		map[string]any{
			"fields": map[string]any{
				"tags": map[string]any{
					"required": false,
					"schema": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":     "string",
							"refTypes": []any{"service"},
						},
					},
				},
			},
		},
		map[string]struct{}{
			"feature": {},
			"service": {},
		},
	)
	if parseErr == nil {
		t.Fatalf("expected parse error")
	}
	if parseErr.Code != domainerrors.CodeSchemaInvalid {
		t.Fatalf("expected code %s, got %s", domainerrors.CodeSchemaInvalid, parseErr.Code)
	}
	if !strings.Contains(parseErr.Message, "schema.entity.feature.meta.fields.tags.schema.items.refTypes is allowed only for items.type entityRef") {
		t.Fatalf("unexpected error message: %s", parseErr.Message)
	}
}

func TestMetafieldsParseRejectsEntityRefAccessThroughMetaInRequiredExpression(t *testing.T) {
	_, _, parseErr := metafields.Parse(
		"feature",
		map[string]any{
			"fields": map[string]any{
				"owner": map[string]any{
					"required": false,
					"schema": map[string]any{
						"type": "entityRef",
					},
				},
				"title": map[string]any{
					"required": "${meta.owner != `null`}",
					"schema": map[string]any{
						"type": "string",
					},
				},
			},
		},
		map[string]struct{}{
			"feature": {},
			"service": {},
		},
	)
	if parseErr == nil {
		t.Fatalf("expected parse error")
	}
	if parseErr.Code != domainerrors.CodeSchemaInvalid {
		t.Fatalf("expected code %s, got %s", domainerrors.CodeSchemaInvalid, parseErr.Code)
	}
	if !strings.Contains(parseErr.Message, "schema.entity.feature.meta.fields.title.required has invalid expression in required context") {
		t.Fatalf("unexpected error message: %s", parseErr.Message)
	}
}

func TestMetafieldsParseRejectsEntityRefAccessThroughMetaInSchemaConst(t *testing.T) {
	_, _, parseErr := metafields.Parse(
		"feature",
		map[string]any{
			"fields": map[string]any{
				"owner": map[string]any{
					"required": false,
					"schema": map[string]any{
						"type": "entityRef",
					},
				},
				"ownerSnapshot": map[string]any{
					"required": false,
					"schema": map[string]any{
						"type":  "string",
						"const": "${meta.owner}",
					},
				},
			},
		},
		map[string]struct{}{
			"feature": {},
			"service": {},
		},
	)
	if parseErr == nil {
		t.Fatalf("expected parse error")
	}
	if parseErr.Code != domainerrors.CodeSchemaInvalid {
		t.Fatalf("expected code %s, got %s", domainerrors.CodeSchemaInvalid, parseErr.Code)
	}
	if !strings.Contains(parseErr.Message, "schema.entity.feature.meta.fields.ownerSnapshot.schema.const has invalid interpolation in schema.const") {
		t.Fatalf("unexpected error message: %s", parseErr.Message)
	}
}

func TestMetafieldsParseRejectsEntityRefAccessThroughMetaInSchemaEnum(t *testing.T) {
	_, _, parseErr := metafields.Parse(
		"feature",
		map[string]any{
			"fields": map[string]any{
				"owner": map[string]any{
					"required": false,
					"schema": map[string]any{
						"type": "entityRef",
					},
				},
				"ownerSnapshot": map[string]any{
					"required": false,
					"schema": map[string]any{
						"type": "string",
						"enum": []any{
							"${meta.owner}",
							"fallback",
						},
					},
				},
			},
		},
		map[string]struct{}{
			"feature": {},
			"service": {},
		},
	)
	if parseErr == nil {
		t.Fatalf("expected parse error")
	}
	if parseErr.Code != domainerrors.CodeSchemaInvalid {
		t.Fatalf("expected code %s, got %s", domainerrors.CodeSchemaInvalid, parseErr.Code)
	}
	if !strings.Contains(parseErr.Message, "schema.entity.feature.meta.fields.ownerSnapshot.schema.enum[0] has invalid interpolation in schema.enum") {
		t.Fatalf("unexpected error message: %s", parseErr.Message)
	}
}
