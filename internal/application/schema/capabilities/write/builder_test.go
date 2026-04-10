package write

import (
	"reflect"
	"testing"

	schemaexpressions "github.com/anatoly-tenenev/spec-cli/internal/application/schema/expressions"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
)

func TestBuildWriteCapability(t *testing.T) {
	minItems := 1
	maxItems := 3

	compiled := model.CompiledSchema{
		Entities: map[string]model.EntityType{
			"feature": {
				Name:           "feature",
				IDPrefix:       "FEAT",
				HasContent:     true,
				MetaFieldOrder: []string{"status", "owner", "watchers"},
				SectionOrder:   []string{"summary", "implementation"},
				MetaFields: map[string]model.MetaField{
					"status": {
						Name:     "status",
						Value:    model.ValueSpec{Kind: model.ValueKindString},
						Required: model.Requirement{Always: true, Path: "entity.feature.meta.fields.status.required"},
					},
					"owner": {
						Name: "owner",
						Value: model.ValueSpec{
							Kind: model.ValueKindEntityRef,
							Ref:  &model.RefSpec{Cardinality: model.RefCardinalityScalar},
						},
						Required: model.Requirement{Always: false, Path: "entity.feature.meta.fields.owner.required"},
					},
					"watchers": {
						Name: "watchers",
						Value: model.ValueSpec{
							Kind: model.ValueKindArray,
							Items: &model.ValueSpec{
								Kind: model.ValueKindEntityRef,
								Ref: &model.RefSpec{
									Cardinality:  model.RefCardinalityArray,
									AllowedTypes: []string{"person"},
								},
							},
							UniqueItems: true,
							MinItems:    &minItems,
							MaxItems:    &maxItems,
						},
						Required: model.Requirement{Always: true, Path: "entity.feature.meta.fields.watchers.required"},
					},
				},
				Sections: map[string]model.Section{
					"summary": {
						Name:     "summary",
						Title:    "Summary",
						Required: model.Requirement{Always: true, Path: "entity.feature.content.sections.summary.required"},
					},
					"implementation": {
						Name:     "implementation",
						Title:    "Implementation",
						Required: model.Requirement{Always: false, Path: "entity.feature.content.sections.implementation.required"},
					},
				},
				PathTemplate: model.PathTemplate{
					Cases: []model.PathTemplateCase{
						{
							Use:     "features/${slug}.md",
							When:    model.Requirement{Always: true, Path: "entity.feature.pathTemplate.cases[0].when"},
							UsePath: "entity.feature.pathTemplate.cases[0].use",
						},
					},
				},
			},
		},
	}

	capability := Build(compiled)
	entity := capability.EntityTypes["feature"]

	expectedSetPaths := []string{
		"content.sections.implementation",
		"content.sections.summary",
		"meta.status",
		"refs.owner",
		"refs.watchers",
	}
	if !reflect.DeepEqual(entity.SetPaths, expectedSetPaths) {
		t.Fatalf("unexpected set paths: %#v", entity.SetPaths)
	}
	if !reflect.DeepEqual(entity.UnsetPaths, expectedSetPaths) {
		t.Fatalf("unexpected unset paths: %#v", entity.UnsetPaths)
	}
	if !reflect.DeepEqual(entity.SetFilePaths, []string{"content.sections.implementation", "content.sections.summary"}) {
		t.Fatalf("unexpected set-file paths: %#v", entity.SetFilePaths)
	}

	if entity.IDPrefix != "FEAT" {
		t.Fatalf("unexpected idPrefix: %q", entity.IDPrefix)
	}
	if !entity.HasContent {
		t.Fatalf("expected hasContent=true")
	}
	if !reflect.DeepEqual(entity.MetaFieldOrder, []string{"status", "owner", "watchers"}) {
		t.Fatalf("unexpected meta field order: %#v", entity.MetaFieldOrder)
	}
	if !reflect.DeepEqual(entity.SectionOrder, []string{"summary", "implementation"}) {
		t.Fatalf("unexpected section order: %#v", entity.SectionOrder)
	}

	ownerPath := entity.AllowWritePaths["refs.owner"]
	if ownerPath.Kind != WritePathRef || ownerPath.FieldName != "owner" {
		t.Fatalf("unexpected refs.owner path spec: %#v", ownerPath)
	}
	watchers := entity.MetaFields["watchers"]
	if !watchers.IsEntityRefArray {
		t.Fatalf("expected watchers as entityRef array")
	}
	if !watchers.HasItems || watchers.ItemType != "entityRef" {
		t.Fatalf("unexpected watchers item type: %#v", watchers)
	}
	if !watchers.UniqueItems || !watchers.HasMinItems || !watchers.HasMaxItems {
		t.Fatalf("unexpected watchers array limits: %#v", watchers)
	}
	if watchers.MinItems != 1 || watchers.MaxItems != 3 {
		t.Fatalf("unexpected watchers min/max: %#v", watchers)
	}

	if len(entity.PathPattern.Cases) != 1 || entity.PathPattern.Cases[0].Use != "features/${slug}.md" {
		t.Fatalf("unexpected path pattern: %#v", entity.PathPattern.Cases)
	}
	if got := entity.MetaFields["status"].RequiredPath; got != "entity.feature.meta.fields.status.required" {
		t.Fatalf("unexpected status required path: %q", got)
	}
	if got := entity.Sections["summary"].RequiredPath; got != "entity.feature.content.sections.summary.required" {
		t.Fatalf("unexpected summary required path: %q", got)
	}
	if got := entity.Sections["summary"].Title; got != "Summary" {
		t.Fatalf("unexpected summary title: %q", got)
	}
	if got := entity.PathPattern.Cases[0].WhenPath; got != "entity.feature.pathTemplate.cases[0].when" {
		t.Fatalf("unexpected when path: %q", got)
	}
	if got := entity.PathPattern.Cases[0].UsePath; got != "entity.feature.pathTemplate.cases[0].use" {
		t.Fatalf("unexpected use path: %q", got)
	}
	if _, exists := entity.AllowSetFilePaths["content.sections.summary"]; !exists {
		t.Fatalf("expected content.sections.summary in set-file allowlist")
	}
}

func TestBuildWriteCapabilityKeepsTemplateAwareMetaConstAndEnum(t *testing.T) {
	constTemplate := &schemaexpressions.CompiledTemplate{Raw: "${refs.owner.slug}"}
	enumTemplate := &schemaexpressions.CompiledTemplate{Raw: "${meta.status}"}

	compiled := model.CompiledSchema{
		Entities: map[string]model.EntityType{
			"feature": {
				Name:           "feature",
				IDPrefix:       "FEAT",
				MetaFieldOrder: []string{"ownerSlug", "status"},
				MetaFields: map[string]model.MetaField{
					"ownerSlug": {
						Name: "ownerSlug",
						Value: model.ValueSpec{
							Kind: model.ValueKindString,
							Const: &model.Literal{
								Value:    "${refs.owner.slug}",
								Template: constTemplate,
							},
						},
						Required: model.Requirement{Always: true, Path: "entity.feature.meta.fields.ownerSlug.required"},
					},
					"status": {
						Name: "status",
						Value: model.ValueSpec{
							Kind: model.ValueKindString,
							Enum: []model.Literal{
								{Value: "draft"},
								{Value: "${meta.status}", Template: enumTemplate},
							},
						},
						Required: model.Requirement{Always: true, Path: "entity.feature.meta.fields.status.required"},
					},
				},
				PathTemplate: model.PathTemplate{
					Cases: []model.PathTemplateCase{
						{
							Use:         "features/${slug}.md",
							UseTemplate: &schemaexpressions.CompiledTemplate{Raw: "features/${slug}.md"},
							When:        model.Requirement{Always: true, Path: "entity.feature.pathTemplate.cases[0].when"},
							UsePath:     "entity.feature.pathTemplate.cases[0].use",
						},
					},
				},
			},
		},
	}

	capability := Build(compiled)
	entity := capability.EntityTypes["feature"]

	ownerSlug := entity.MetaFields["ownerSlug"]
	if !ownerSlug.HasConst {
		t.Fatalf("expected ownerSlug const")
	}
	if ownerSlug.Const.Literal != "${refs.owner.slug}" {
		t.Fatalf("unexpected const literal: %#v", ownerSlug.Const.Literal)
	}
	if ownerSlug.Const.Template != constTemplate {
		t.Fatalf("expected const template pointer to be preserved")
	}

	status := entity.MetaFields["status"]
	if len(status.Enum) != 2 {
		t.Fatalf("unexpected enum size: %d", len(status.Enum))
	}
	if status.Enum[0].Literal != "draft" || status.Enum[0].Template != nil {
		t.Fatalf("unexpected first enum value: %#v", status.Enum[0])
	}
	if status.Enum[1].Literal != "${meta.status}" {
		t.Fatalf("unexpected second enum literal: %#v", status.Enum[1].Literal)
	}
	if status.Enum[1].Template != enumTemplate {
		t.Fatalf("expected enum template pointer to be preserved")
	}
}
