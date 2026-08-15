// Package writemodel holds the entity types shared by the add and update
// write pipelines. Command-specific shapes (Options, Snapshot) stay in each
// command's own model package, because they genuinely differ.
package writemodel

import schemacapwrite "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/write"

type EntityTypeSpec = schemacapwrite.EntityWriteModel

type PathPattern = schemacapwrite.PathPattern

type PathPatternCase = schemacapwrite.PathPatternCase

type WorkspaceEntity struct {
	PathAbs      string
	PathRelPOSIX string
	DirPath      string
	Type         string
	ID           string
	Slug         string
	Frontmatter  map[string]any
	Meta         map[string]any
	Body         string
}

type Candidate struct {
	Type         string
	ID           string
	Slug         string
	CreatedDate  string
	UpdatedDate  string
	Frontmatter  map[string]any
	Meta         map[string]any
	RefIDs       map[string]string
	RefIDArrays  map[string][]string
	Refs         map[string]ResolvedRef
	RefArrays    map[string][]ResolvedRef
	Body         string
	Sections     map[string]string
	PathRelPOSIX string
	PathAbs      string
	Serialized   []byte
	Revision     string
}

type ResolvedRef struct {
	Type    string
	ID      string
	Slug    string
	DirPath string
	Meta    map[string]any
}
