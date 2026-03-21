package model

import "encoding/json"

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
	WhereJSON   string
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
)

type SchemaFieldSpec struct {
	Path       string
	Kind       SchemaFieldKind
	EnumValues []any
}

type EntityTypeSpec struct {
	Name          string
	RefFields     map[string]struct{}
	RefTypeHints  map[string]string
	SectionFields map[string]struct{}
}

type QuerySchemaIndex struct {
	EntityTypes   map[string]EntityTypeSpec
	SelectorPaths map[string]struct{}
	SortFields    map[string]SchemaFieldSpec
	FilterFields  map[string]SchemaFieldSpec
}

type EntityView struct {
	Type string
	ID   string
	View map[string]any
}

type RawFilterNodeKind string

const (
	RawFilterNodeLeaf RawFilterNodeKind = "leaf"
	RawFilterNodeAnd  RawFilterNodeKind = "and"
	RawFilterNodeOr   RawFilterNodeKind = "or"
	RawFilterNodeNot  RawFilterNodeKind = "not"
)

type RawFilterNode struct {
	Kind     RawFilterNodeKind
	Field    string
	Op       string
	Value    any
	HasValue bool
	Filters  []RawFilterNode
	Filter   *RawFilterNode
}

type FilterNodeKind string

const (
	FilterNodeLeaf FilterNodeKind = "leaf"
	FilterNodeAnd  FilterNodeKind = "and"
	FilterNodeOr   FilterNodeKind = "or"
	FilterNodeNot  FilterNodeKind = "not"
)

type FilterNode struct {
	Kind     FilterNodeKind
	Field    string
	Op       string
	Value    any
	HasValue bool
	Spec     SchemaFieldSpec
	Filters  []FilterNode
	Filter   *FilterNode
}

type QueryPlan struct {
	SelectTree        *SelectNode
	EffectiveSort     []SortTerm
	TypedFilter       *FilterNode
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
