package model

import schemacapwrite "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/write"
import domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"

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

type ValidationResult struct {
	Issues []domainvalidation.Issue
}
