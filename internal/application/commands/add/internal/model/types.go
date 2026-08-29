// Package model holds add's internal types: the parsed options, the requested
// write operations, and the workspace snapshot a new entity is placed against.
// The snapshot carries the indexes creation needs - ids, slugs per type, the
// highest id suffix, existing paths - because each of those is a uniqueness
// rule the new document has to satisfy.
//
// The schema-derived types are aliases of the write capability rather than
// copies, so add cannot drift from what the schema declares writable.
package model

import (
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/writemodel"
	schemacapwrite "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/write"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

type Options struct {
	EntityType   string
	Slug         string
	Operations   []WriteOperation
	ContentFile  string
	ContentStdin bool
	DryRun       bool
}

type WriteOperationKind string

const (
	WriteOperationSet     WriteOperationKind = "set"
	WriteOperationSetFile WriteOperationKind = "set-file"
)

type WriteOperation struct {
	Kind     WriteOperationKind
	Path     string
	RawValue string
}

type EntityTypeSpec = schemacapwrite.EntityWriteModel

type WritePathKind = schemacapwrite.WritePathKind

const (
	WritePathMeta    WritePathKind = schemacapwrite.WritePathMeta
	WritePathRef     WritePathKind = schemacapwrite.WritePathRef
	WritePathSection WritePathKind = schemacapwrite.WritePathSection
)

type WritePathSpec = schemacapwrite.WritePathSpec
type MetaField = schemacapwrite.MetaField
type SectionSpec = schemacapwrite.SectionSpec
type RuleValue = schemacapwrite.RuleValue
type PathPattern = schemacapwrite.PathPattern
type PathPatternCase = schemacapwrite.PathPatternCase

type Snapshot struct {
	WorkspacePath   string
	Entities        []WorkspaceEntity
	EntitiesByID    map[string][]WorkspaceEntity
	SlugsByType     map[string]map[string][]WorkspaceEntity
	MaxSuffixByType map[string]int
	ExistingPaths   map[string]struct{}
}

// Shared with the update pipeline; see commands/internal/writemodel.
type WorkspaceEntity = writemodel.WorkspaceEntity

type Candidate = writemodel.Candidate

type ResolvedRef = writemodel.ResolvedRef

type ValidationResult struct {
	Issues []domainvalidation.Issue
}
