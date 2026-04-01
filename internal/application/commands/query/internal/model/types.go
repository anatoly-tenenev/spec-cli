package model

import (
	"encoding/json"

	jmespath "github.com/anatoly-tenenev/go-jmespath"
)

type SortDirection string

const (
	SortDirectionAsc  SortDirection = "asc"
	SortDirectionDesc SortDirection = "desc"
)

type SortTerm struct {
	Path      string
	Direction SortDirection
}

type Options struct {
	TypeFilters []string
	WhereExpr   string
	Selects     []string
	Sorts       []SortTerm
	Limit       int
	Offset      int
}

type SchemaFieldKind string

const (
	FieldKindString  SchemaFieldKind = "string"
	FieldKindDate    SchemaFieldKind = "date"
	FieldKindNumber  SchemaFieldKind = "number"
	FieldKindArray   SchemaFieldKind = "array"
	FieldKindBoolean SchemaFieldKind = "boolean"
	FieldKindObject  SchemaFieldKind = "object"
	FieldKindNull    SchemaFieldKind = "null"
)

type MetadataFieldSpec struct {
	Name      string
	Kind      SchemaFieldKind
	ItemKind  SchemaFieldKind
	EnumValues []any
	HasConst  bool
	ConstValue any
	Required  bool
}

type SectionFieldSpec struct {
	Name     string
	Required bool
}

type RefCardinality string

const (
	RefCardinalityScalar RefCardinality = "scalar"
	RefCardinalityArray  RefCardinality = "array"
)

type RefFieldSpec struct {
	Name        string
	Cardinality RefCardinality
	RefTypes    []string
}

type EntityTypeSpec struct {
	Name          string
	MetaFields    map[string]MetadataFieldSpec
	RefFields     map[string]RefFieldSpec
	SectionFields map[string]SectionFieldSpec
}

type QuerySchemaIndex struct {
	EntityTypes map[string]EntityTypeSpec
}

type EntityView struct {
	Type         string
	ID           string
	View         map[string]any
	WhereContext map[string]any
}

type WherePlan struct {
	Source string
	Query  *jmespath.JMESPath
}

type QueryPlan struct {
	SelectTree        *SelectNode
	EffectiveSort     []SortTerm
	Where             *WherePlan
	ActiveTypeSet     []string
	OriginalSelects   []string
	OriginalSortTerms []SortTerm
	Limit             int
	Offset            int
}

type SelectNode struct {
	Terminal bool
	Children map[string]*SelectNode
}

type PageInfo struct {
	Mode          string
	Limit         int
	Offset        int
	Returned      int
	HasMore       bool
	NextOffset    any
	EffectiveSort []string
}

type QueryResponse struct {
	ResultState string
	Items       []map[string]any
	Matched     int
	Page        PageInfo
}

type JSONValue = json.RawMessage
