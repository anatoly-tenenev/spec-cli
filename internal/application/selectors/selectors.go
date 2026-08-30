// Package selectors holds the selector machinery get and query share: the tree
// a set of dotted selectors builds, and the schema questions a reference
// selector has to answer before it can be accepted.
//
// The two commands read the same workspace through the same schema, so a
// projection one of them returns cannot be shaped differently from the
// other's, and refs.owner.slug cannot be addressable through one and not the
// other. What each command still decides for itself is which selectors exist
// at all and how a refusal is worded, so that stays in the command.
package selectors

import (
	schemacapread "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/read"
)

// Node is one level of the selector tree. Terminal marks a selector the caller
// actually asked for; the nodes above it exist only to hold their children.
type Node struct {
	Terminal bool
	Children map[string]*Node
}

// NewTree returns an empty tree, selecting nothing.
func NewTree() *Node {
	return &Node{Children: map[string]*Node{}}
}

// Insert adds one selector, given as its dot-separated parts.
//
// A broader selector wins over a narrower one: asking for meta and
// meta.status selects all of meta, in either order. Everything under a
// terminal node is already included, so a longer path below one is dropped,
// and a shorter path over an existing subtree replaces it.
func Insert(root *Node, parts []string) {
	current := root
	for idx, part := range parts {
		if current.Terminal {
			return
		}

		child, exists := current.Children[part]
		if !exists {
			child = &Node{Children: map[string]*Node{}}
			current.Children[part] = child
		}

		if idx == len(parts)-1 {
			child.Terminal = true
			child.Children = map[string]*Node{}
			return
		}
		current = child
	}
}

var refLeaves = map[string]struct{}{
	"id":       {},
	"resolved": {},
	"type":     {},
	"slug":     {},
	"reason":   {},
}

// IsRefLeaf reports whether name addresses a part of a resolved reference.
// The set is closed: it is what a reference object contains, not what the
// schema declares.
func IsRefLeaf(name string) bool {
	_, exists := refLeaves[name]
	return exists
}

// RefLeafNames lists those parts in a fixed order, for callers that enumerate
// the selectors a reference field admits rather than checking one.
func RefLeafNames() []string {
	return []string{"id", "resolved", "type", "slug", "reason"}
}

// HasRefField reports whether any type in the active set declares the field as
// a reference. Any one type is enough: with several types in play, requiring
// all of them to declare the field would make a cross-type request impossible.
func HasRefField(refField string, capability schemacapread.Capability, activeTypeSet []string) bool {
	for _, typeName := range activeTypeSet {
		entityType := capability.EntityTypes[typeName]
		if _, exists := entityType.RefFields[refField]; exists {
			return true
		}
	}
	return false
}

// RefLeafCompatibility reports whether a reference field may be addressed leaf
// by leaf, and whether it exists at all in the active set. An array reference
// has no single target, so refs.<field>.<leaf> would have nothing to point at
// and is refused for the whole set as soon as one type declares it as an array.
func RefLeafCompatibility(
	refField string,
	capability schemacapread.Capability,
	activeTypeSet []string,
) (compatible bool, exists bool) {
	hasScalar := false
	for _, typeName := range activeTypeSet {
		entityType := capability.EntityTypes[typeName]
		refSpec, present := entityType.RefFields[refField]
		if !present {
			continue
		}
		exists = true
		if refSpec.Cardinality == schemacapread.RefCardinalityArray {
			return false, true
		}
		hasScalar = true
	}
	return hasScalar, exists
}
