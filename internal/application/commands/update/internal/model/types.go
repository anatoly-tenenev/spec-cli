package model

import domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"

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

type Schema struct {
	EntityTypes map[string]EntityTypeSpec
}

type EntityTypeSpec struct {
	Name              string
	IDPrefix          string
	PathPattern       PathPattern
	MetaFields        map[string]MetaField
	MetaFieldOrder    []string
	Sections          map[string]SectionSpec
	SectionOrder      []string
	HasContent        bool
	AllowSetPaths     map[string]WritePathSpec
	AllowUnsetPaths   map[string]WritePathSpec
	AllowSetFilePaths map[string]struct{}
}

type WritePathKind string

const (
	WritePathMeta    WritePathKind = "meta"
	WritePathRef     WritePathKind = "ref"
	WritePathSection WritePathKind = "section"
)

type WritePathSpec struct {
	Kind      WritePathKind
	FieldName string
}

type MetaField struct {
	Name            string
	Type            string
	Format          string
	Required        bool
	HasRequiredWhen bool
	RequiredWhen    any
	Enum            []any
	HasConst        bool
	Const           any
	IsEntityRef     bool
	RefTypes        []string
	HasItems        bool
	ItemType        string
	UniqueItems     bool
	HasMinItems     bool
	MinItems        int
	HasMaxItems     bool
	MaxItems        int
}

type SectionSpec struct {
	Name            string
	Titles          []string
	Required        bool
	HasRequiredWhen bool
	RequiredWhen    any
}

type PathPattern struct {
	Cases []PathPatternCase
}

type PathPatternCase struct {
	Use     string
	HasWhen bool
	When    any
}

type Snapshot struct {
	WorkspacePath string
	Entities      []WorkspaceEntity
	EntitiesByID  map[string][]WorkspaceEntity
	SlugsByType   map[string]map[string][]WorkspaceEntity
	ExistingPaths map[string]struct{}
	TargetMatches []TargetMatch
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

type TargetMatch struct {
	PathAbs string
	Raw     []byte
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
	Refs         map[string]ResolvedRef
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
