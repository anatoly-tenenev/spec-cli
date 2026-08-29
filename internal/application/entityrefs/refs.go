// Package entityrefs decides what an entity reference points at and what it
// looks like in a response. get and query answer that for the same workspace
// and must agree: a reference one of them calls resolved cannot be unresolved
// for the other, and the object they emit has to be the same shape.
//
// A reference that matches nothing, matches a type the field does not allow, or
// matches several entities is reported unresolved with the reason, rather than
// guessed at.
package entityrefs

import (
	"strings"

	schemacapread "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/read"
)

// Identity is the part of an entity a reference can be resolved against.
type Identity struct {
	Type string
	ID   string
	Slug string
}

// Ref is a reference after resolution. Type and Slug are any because an
// unresolved reference reports them as null.
type Ref struct {
	ID       string
	Resolved bool
	Type     any
	Slug     any
	Reason   any
}

func Classify(targetID string, refSpec schemacapread.RefField, idIndex map[string][]Identity) Ref {
	targets := idIndex[targetID]
	compatibleTargets := filterByTypes(targets, refSpec.AllowedTypes)

	if len(compatibleTargets) == 1 {
		target := compatibleTargets[0]
		return Ref{
			ID:       targetID,
			Resolved: true,
			Type:     target.Type,
			Slug:     target.Slug,
			Reason:   nil,
		}
	}

	reason := "ambiguous"
	switch {
	case len(targets) == 0:
		reason = "missing"
	case len(compatibleTargets) == 0:
		reason = "type_mismatch"
	}

	return Ref{
		ID:       targetID,
		Resolved: false,
		Type:     typeHint(compatibleTargets, refSpec.AllowedTypes),
		Slug:     nil,
		Reason:   reason,
	}
}

func ToPublicObject(ref Ref) map[string]any {
	value := map[string]any{
		"resolved": ref.Resolved,
		"id":       ref.ID,
		"type":     ref.Type,
		"slug":     ref.Slug,
	}
	if !ref.Resolved {
		value["reason"] = ref.Reason
	}
	return value
}

func filterByTypes(targets []Identity, refTypes []string) []Identity {
	// A ref field with no declared types accepts any type. The compiled schema
	// always fills them in, so this is intent rather than a reachable branch.
	if len(refTypes) == 0 {
		return targets
	}

	allowed := map[string]struct{}{}
	for _, refType := range refTypes {
		allowed[refType] = struct{}{}
	}
	filtered := make([]Identity, 0, len(targets))
	for _, target := range targets {
		if _, ok := allowed[target.Type]; ok {
			filtered = append(filtered, target)
		}
	}
	return filtered
}

func typeHint(targets []Identity, refTypes []string) any {
	if len(refTypes) == 1 {
		return refTypes[0]
	}
	if len(targets) == 0 {
		return nil
	}
	candidate := targets[0].Type
	for idx := 1; idx < len(targets); idx++ {
		if targets[idx].Type != candidate {
			return nil
		}
	}
	return candidate
}

func ReadID(rawTarget any) (string, bool) {
	switch typed := rawTarget.(type) {
	case string:
		return normalizeID(typed)
	case map[string]any:
		rawID, ok := typed["id"]
		if !ok {
			return "", false
		}
		targetID, ok := rawID.(string)
		if !ok {
			return "", false
		}
		return normalizeID(targetID)
	default:
		return "", false
	}
}

func normalizeID(raw string) (string, bool) {
	targetID := strings.TrimSpace(raw)
	if targetID == "" {
		return "", false
	}
	return targetID, true
}
