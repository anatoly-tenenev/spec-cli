package model

import schemaexpressions "github.com/anatoly-tenenev/spec-cli/internal/application/schema/expressions"

type CompiledSchema struct {
	Version     string
	Description string
	Entities    map[string]EntityType
}

type EntityType struct {
	Name           string
	IDPrefix       string
	PathTemplate   PathTemplate
	MetaFields     map[string]MetaField
	MetaFieldOrder []string
	Sections       map[string]Section
	SectionOrder   []string
	HasContent     bool
	Description    string
}

type MetaField struct {
	Name        string
	Value       ValueSpec
	Required    Requirement
	Description string
	SchemaPath  string
}

type Section struct {
	Name        string
	Title       string
	Required    Requirement
	Description string
	SchemaPath  string
	TitlePath   string
}

type Requirement struct {
	Always bool
	Expr   *schemaexpressions.CompiledExpression
	Path   string
}

type PathTemplate struct {
	Cases []PathTemplateCase
}

type PathTemplateCase struct {
	Use         string
	UseTemplate *schemaexpressions.CompiledTemplate
	When        Requirement
	UsePath     string
}

type ValueKind string

const (
	ValueKindUnknown   ValueKind = "unknown"
	ValueKindString    ValueKind = "string"
	ValueKindNumber    ValueKind = "number"
	ValueKindInteger   ValueKind = "integer"
	ValueKindBoolean   ValueKind = "boolean"
	ValueKindArray     ValueKind = "array"
	ValueKindEntityRef ValueKind = "entityRef"
)

type ValueSpec struct {
	Kind        ValueKind
	Format      string
	Enum        []Literal
	Const       *Literal
	Ref         *RefSpec
	Items       *ValueSpec
	UniqueItems bool
	MinItems    *int
	MaxItems    *int
}

type Literal struct {
	Value    any
	Template *schemaexpressions.CompiledTemplate
}

type RefCardinality string

const (
	RefCardinalityScalar RefCardinality = "scalar"
	RefCardinalityArray  RefCardinality = "array"
)

type RefSpec struct {
	Cardinality  RefCardinality
	AllowedTypes []string
}
