// Package writemodel holds the entity types shared by the add and update
// write pipelines, and builds the fields of those types that both commands
// fill the same way. Command-specific shapes (Options, Snapshot) stay in each
// command's own model package, because they genuinely differ.
package writemodel

import (
	schemacapwrite "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/write"
	"github.com/anatoly-tenenev/spec-cli/internal/application/values"
)

type WriteOperationKind string

const (
	WriteOperationSet     WriteOperationKind = "set"
	WriteOperationSetFile WriteOperationKind = "set-file"
	WriteOperationUnset   WriteOperationKind = "unset"
)

// WriteOperation is one --set/--set-file/--unset the caller asked for, still
// carrying the raw text: what a value means depends on the write path it
// targets, which is only known once the schema is consulted.
type WriteOperation struct {
	Kind     WriteOperationKind
	Path     string
	RawValue string
}

type EntityTypeSpec = schemacapwrite.EntityWriteModel

type MetaField = schemacapwrite.MetaField

type RuleValue = schemacapwrite.RuleValue

type WritePathKind = schemacapwrite.WritePathKind

const (
	WritePathMeta    WritePathKind = schemacapwrite.WritePathMeta
	WritePathRef     WritePathKind = schemacapwrite.WritePathRef
	WritePathSection WritePathKind = schemacapwrite.WritePathSection
)

type WritePathSpec = schemacapwrite.WritePathSpec

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

// BuildMeta keeps every frontmatter key that is not a built-in field. Both
// write pipelines report the same Meta for the same document, so the rule for
// what counts as metadata lives with the type that carries it.
func BuildMeta(frontmatter map[string]any) map[string]any {
	meta := map[string]any{}
	for key, value := range frontmatter {
		switch key {
		case "type", "id", "slug", "createdDate", "updatedDate":
			continue
		default:
			meta[key] = values.NormalizeValue(value)
		}
	}
	return meta
}
