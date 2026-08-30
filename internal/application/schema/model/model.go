// Package model is the compiled schema IR: one in-memory shape that every
// consumer of a schema reads. Nothing here parses or validates - the compiler
// produces these types and the capability builders project them - so the IR
// stays the only agreed vocabulary between schema authoring and the commands.
//
// Declaration order is kept next to every map (EntityOrder, MetaFieldOrder,
// SectionOrder) because responses must not depend on map iteration order.
package model

import schemaexpressions "github.com/anatoly-tenenev/spec-cli/internal/application/schema/expressions"

type CompiledSchema struct {
	Version     string
	Description string
	Entities    map[string]EntityType
	EntityOrder []string
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

// TypeName is the name a kind is reported under. The write and validate
// capabilities both describe the same field to a caller - one as what may be
// written, the other as what is demanded - so a field the two name differently
// would let add accept a value validate then rejects for its type. A kind the
// IR does not know is named once, here, rather than in each projection.
func (kind ValueKind) TypeName() string {
	switch kind {
	case ValueKindString, ValueKindNumber, ValueKindInteger, ValueKindBoolean, ValueKindArray, ValueKindEntityRef:
		return string(kind)
	default:
		return string(ValueKindUnknown)
	}
}

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
