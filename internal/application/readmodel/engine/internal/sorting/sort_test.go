package sorting

import (
	"testing"

	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/internal/testsupport"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/model"
	schemacapread "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/read"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func TestBuildEffectiveSort_DefaultAndTail(t *testing.T) {
	index := testsupport.NewCapability()

	terms, err := BuildEffective(nil, index, []string{"feature", "service"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(terms) != 1 || terms[0].Path != "id" {
		t.Fatalf("unexpected default sort: %#v", terms)
	}

	custom := []model.SortTerm{{Path: "updatedDate", Direction: model.SortDirectionDesc}}
	effective, err := BuildEffective(custom, index, []string{"feature", "service"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(effective) != 2 || effective[1].Path != "id" {
		t.Fatalf("tail not appended: %#v", effective)
	}
}

func TestBuildEffectiveSort_InvalidField(t *testing.T) {
	index := testsupport.NewCapability()
	_, err := BuildEffective([]model.SortTerm{{Path: "meta", Direction: model.SortDirectionAsc}}, index, []string{"feature", "service"})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != domainerrors.CodeInvalidArgs {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func TestBuildEffectiveSort_RejectsMetaEntityRefAcrossActiveSet(t *testing.T) {
	index := schemacapread.Capability{
		EntityTypes: map[string]schemacapread.EntityReadModel{
			"feature": {
				MetaFields: map[string]schemacapread.MetaField{},
				RefFields: map[string]schemacapread.RefField{
					"owner": {Cardinality: schemacapread.RefCardinalityScalar, AllowedTypes: []string{"service"}},
				},
			},
			"service": {
				MetaFields: map[string]schemacapread.MetaField{
					"owner": {Kind: schemacapread.FieldKindString, Required: true},
				},
				RefFields: map[string]schemacapread.RefField{},
			},
		},
	}

	_, err := BuildEffective([]model.SortTerm{{Path: "meta.owner", Direction: model.SortDirectionAsc}}, index, []string{"feature", "service"})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != domainerrors.CodeInvalidArgs {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}

func TestBuildEffectiveSort_SingletonValidationAllowsTypeLocalRefSort(t *testing.T) {
	index := testsupport.NewCapability()

	effective, err := BuildEffective([]model.SortTerm{{Path: "refs.owner.id", Direction: model.SortDirectionAsc}}, index, []string{"feature"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(effective) != 2 || effective[0].Path != "refs.owner.id" || effective[1].Path != "id" {
		t.Fatalf("unexpected effective sort: %#v", effective)
	}
}

func TestBuildEffectiveSort_SingletonValidationRejectsInvalidTypeLocalPath(t *testing.T) {
	index := testsupport.NewCapability()

	_, err := BuildEffective([]model.SortTerm{{Path: "refs.owner.id", Direction: model.SortDirectionAsc}}, index, []string{"service"})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != domainerrors.CodeInvalidArgs {
		t.Fatalf("unexpected error code: %s", err.Code)
	}
}
