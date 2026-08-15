package model

import (
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/writemodel"
	schemacapwrite "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/write"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

type Options struct {
	ID             string
	Operations     []WriteOperation
	BodyOperation  BodyOperationKind
	BodyFile       string
	ExpectRevision string
	DryRun         bool
}

type WriteOperationKind string

const (
	WriteOperationSet     WriteOperationKind = "set"
	WriteOperationSetFile WriteOperationKind = "set-file"
	WriteOperationUnset   WriteOperationKind = "unset"
)

type WriteOperation struct {
	Kind     WriteOperationKind
	Path     string
	RawValue string
}

type BodyOperationKind string

const (
	BodyOperationNone         BodyOperationKind = "none"
	BodyOperationReplaceFile  BodyOperationKind = "replace_file"
	BodyOperationReplaceSTDIN BodyOperationKind = "replace_stdin"
	BodyOperationClear        BodyOperationKind = "clear"
)

type WriteCapability = schemacapwrite.Capability
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
	WorkspacePath string
	Entities      []WorkspaceEntity
	EntitiesByID  map[string][]WorkspaceEntity
	SlugsByType   map[string]map[string][]WorkspaceEntity
	ExistingPaths map[string]struct{}
	TargetMatches []TargetMatch
}

// Shared with the add pipeline; see commands/internal/writemodel.
type WorkspaceEntity = writemodel.WorkspaceEntity

type TargetMatch struct {
	PathAbs string
	Raw     []byte
}

type Candidate = writemodel.Candidate

type ResolvedRef = writemodel.ResolvedRef

type ValidationResult struct {
	Issues []domainvalidation.Issue
}
