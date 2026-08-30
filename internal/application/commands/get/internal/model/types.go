// Package model holds get's internal types: the parsed options, the selector
// plan, and the located document. The selector plan records not just the tree
// to project but what reaching it requires - which reference fields and
// sections must be resolved - so get resolves only what was actually asked
// for.
package model

import (
	"github.com/anatoly-tenenev/spec-cli/internal/application/entityrefs"
	"github.com/anatoly-tenenev/spec-cli/internal/application/selectors"
)

type Options struct {
	ID        string
	Selectors []string
}

// SelectNode is the shared selector tree; see internal/application/selectors.
type SelectNode = selectors.Node

type SelectorPlan struct {
	Tree                 *SelectNode
	EffectiveSelectors   []string
	NullIfMissingPaths   map[string]struct{}
	RequiredRefFields    map[string]struct{}
	RequiresAllRefFields bool
	RequiredSectionNames map[string]struct{}
	RequiresRefs         bool
	RequiresSections     bool
	RequiresAllSections  bool
	RequiresContent      bool
	RequiresContentRaw   bool
}

type EntityIdentity = entityrefs.Identity

type LocateResult struct {
	TargetPath    string
	TargetRaw     []byte
	IdentityIndex map[string][]EntityIdentity
}

type ParsedTarget struct {
	Path                   string
	Type                   string
	ID                     string
	Slug                   string
	CreatedDate            string
	UpdatedDate            string
	Revision               string
	RawBody                string
	Frontmatter            map[string]any
	Sections               map[string]string
	DuplicateSectionLabels map[string]int
}
