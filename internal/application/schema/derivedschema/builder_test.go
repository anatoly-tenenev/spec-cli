package derivedschema

import (
	"testing"

	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
)

func TestProjectValueSpecDropsIncompatibleConst(t *testing.T) {
	value := model.ValueSpec{
		Kind:  model.ValueKindString,
		Const: &model.Literal{Value: 1},
	}

	projected := ProjectValueSpec(value)

	if projected.Const != nil {
		t.Fatalf("expected incompatible const to be dropped, got %#v", projected.Const)
	}
}

func TestProjectValueSpecFiltersIncompatibleStringEnumItems(t *testing.T) {
	value := model.ValueSpec{
		Kind: model.ValueKindString,
		Enum: []model.Literal{
			{Value: "active"},
			{Value: 1},
		},
	}

	projected := ProjectValueSpec(value)

	if len(projected.Enum) != 1 {
		t.Fatalf("expected one enum value after projection, got %d", len(projected.Enum))
	}
	if projected.Enum[0].Value != "active" {
		t.Fatalf("expected compatible enum item to stay, got %#v", projected.Enum[0].Value)
	}
}

func TestProjectValueSpecFiltersIncompatibleNumberEnumItems(t *testing.T) {
	value := model.ValueSpec{
		Kind: model.ValueKindNumber,
		Enum: []model.Literal{
			{Value: 1},
			{Value: "x"},
		},
	}

	projected := ProjectValueSpec(value)

	if len(projected.Enum) != 1 {
		t.Fatalf("expected one enum value after projection, got %d", len(projected.Enum))
	}
	if projected.Enum[0].Value != 1 {
		t.Fatalf("expected numeric enum item to stay, got %#v", projected.Enum[0].Value)
	}
}

func TestProjectValueSpecRecursivelyProjectsArrayItems(t *testing.T) {
	value := model.ValueSpec{
		Kind: model.ValueKindArray,
		Items: &model.ValueSpec{
			Kind:  model.ValueKindString,
			Const: &model.Literal{Value: 1},
			Enum: []model.Literal{
				{Value: "ok"},
				{Value: 1},
			},
		},
	}

	projected := ProjectValueSpec(value)

	if projected.Items == nil {
		t.Fatalf("expected projected items")
	}
	if projected.Items.Const != nil {
		t.Fatalf("expected incompatible nested const to be dropped, got %#v", projected.Items.Const)
	}
	if len(projected.Items.Enum) != 1 {
		t.Fatalf("expected one nested enum item after projection, got %d", len(projected.Items.Enum))
	}
	if projected.Items.Enum[0].Value != "ok" {
		t.Fatalf("expected compatible nested enum item to stay, got %#v", projected.Items.Enum[0].Value)
	}
}
