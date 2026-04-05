package references

import (
	"testing"

	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
)

func TestBuildReferenceCapability(t *testing.T) {
	compiled := model.CompiledSchema{
		Entities: map[string]model.EntityType{
			"feature": {
				MetaFields: map[string]model.MetaField{
					"owner": {
						Value: model.ValueSpec{
							Kind: model.ValueKindEntityRef,
							Ref: &model.RefSpec{
								Cardinality:  model.RefCardinalityScalar,
								AllowedTypes: []string{"service"},
							},
						},
					},
					"watchers": {
						Value: model.ValueSpec{
							Kind: model.ValueKindArray,
							Items: &model.ValueSpec{
								Kind: model.ValueKindEntityRef,
								Ref:  &model.RefSpec{Cardinality: model.RefCardinalityScalar},
							},
						},
					},
				},
			},
			"service": {},
		},
	}

	capability := Build(compiled)

	serviceSlots := capability.InboundByTargetType["service"]
	if len(serviceSlots) != 2 {
		t.Fatalf("expected 2 inbound slots for service, got %#v", serviceSlots)
	}
	if serviceSlots[0].FieldName != "owner" || serviceSlots[0].Cardinality != model.RefCardinalityScalar {
		t.Fatalf("unexpected first service slot: %#v", serviceSlots[0])
	}
	if serviceSlots[1].FieldName != "watchers" || serviceSlots[1].Cardinality != model.RefCardinalityArray {
		t.Fatalf("unexpected second service slot: %#v", serviceSlots[1])
	}

	featureSlots := capability.InboundByTargetType["feature"]
	if len(featureSlots) != 1 || featureSlots[0].FieldName != "watchers" {
		t.Fatalf("expected watchers array inbound slot for feature, got %#v", featureSlots)
	}

	sourceSlots := capability.SlotsBySourceType["feature"]
	if len(sourceSlots) != 2 {
		t.Fatalf("expected 2 source slots for feature, got %#v", sourceSlots)
	}
	if sourceSlots[0].FieldName != "owner" || sourceSlots[0].Cardinality != model.RefCardinalityScalar {
		t.Fatalf("unexpected first source slot: %#v", sourceSlots[0])
	}
	if sourceSlots[1].FieldName != "watchers" || sourceSlots[1].Cardinality != model.RefCardinalityArray {
		t.Fatalf("unexpected second source slot: %#v", sourceSlots[1])
	}
}

func TestBuildReferenceCapabilityKeepsSourceSlotsWithoutTargetExpansion(t *testing.T) {
	compiled := model.CompiledSchema{
		Entities: map[string]model.EntityType{
			"feature": {
				MetaFields: map[string]model.MetaField{
					"container": {
						Value: model.ValueSpec{
							Kind: model.ValueKindEntityRef,
							Ref: &model.RefSpec{
								Cardinality:  model.RefCardinalityScalar,
								AllowedTypes: []string{"feature"},
							},
						},
					},
				},
			},
			"service": {},
		},
	}

	capability := Build(compiled)

	if inbound := capability.InboundByTargetType["service"]; len(inbound) != 0 {
		t.Fatalf("expected no inbound slots for service, got %#v", inbound)
	}

	sourceSlots := capability.SlotsBySourceType["feature"]
	if len(sourceSlots) != 1 {
		t.Fatalf("expected one source slot for feature, got %#v", sourceSlots)
	}
	if sourceSlots[0].FieldName != "container" || sourceSlots[0].Cardinality != model.RefCardinalityScalar {
		t.Fatalf("unexpected source slot: %#v", sourceSlots[0])
	}
}
