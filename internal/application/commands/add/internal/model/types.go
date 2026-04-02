package model

import domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
import commandexpressions "github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/expressions"

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

type AddSchema struct {
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
	AllowWritePaths   map[string]WritePathSpec
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
	Name             string
	Type             string
	Format           string
	Required         bool
	RequiredExpr     *commandexpressions.CompiledExpression
	Enum             []any
	HasConst         bool
	Const            any
	IsEntityRef      bool
	IsEntityRefArray bool
	RefTypes         []string
	HasItems         bool
	ItemType         string
	ItemRefTypes     []string
	UniqueItems      bool
	HasMinItems      bool
	MinItems         int
	HasMaxItems      bool
	MaxItems         int
}

type SectionSpec struct {
	Name         string
	Titles       []string
	Required     bool
	RequiredExpr *commandexpressions.CompiledExpression
}

type PathPattern struct {
	Cases []PathPatternCase
}

type PathPatternCase struct {
	Use         string
	UseTemplate *commandexpressions.CompiledTemplate
	HasWhen     bool
	When        bool
	WhenExpr    *commandexpressions.CompiledExpression
}

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
