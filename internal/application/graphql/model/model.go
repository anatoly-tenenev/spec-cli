package model

import readmodel "github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/model"

type Projection struct {
	EntityOrder []string
	Entities    map[string]Entity
}

type Entity struct {
	Name          string
	TypeName      string
	ListName      string
	MetaName      string
	RefsName      string
	ContentName   string
	SectionsName  string
	WhereName     string
	SortName      string
	SortFieldName string
	Description   string
	MetaFields    []MetaField
	RefFields     []RefField
	Sections      []Section
	SortFields    []SortField
}

type MetaField struct {
	Name         string
	TypeName     string
	Kind         string
	ItemKind     string
	Required     bool
	RequiredWhen string
	Description  string
	EnumName     string
	EnumValues   []string
	IsArray      bool
	MinItems     *int
	MaxItems     *int
	UniqueItems  bool
}

type RefField struct {
	Name            string
	TypeName        string
	EnumName        string
	ArrayFilterName string
	Required        bool
	RequiredWhen    string
	Description     string
	Cardinality     string
	AllowedTypes    []string
	MinItems        *int
	MaxItems        *int
	UniqueItems     bool
}

type Section struct {
	Name         string
	Required     bool
	RequiredWhen string
	Description  string
}

type SortField struct {
	Name string
	Path string
}

type RootPlan struct {
	ResponseKey  string
	EntityType   string
	Limit        int
	Offset       int
	Sort         []readmodel.SortTerm
	Where        Predicate
	Selection    ResultSelection
	NonNullPaths []string
}

type ResultSelection struct {
	Items      *SelectionNode
	TotalCount bool
	PageInfo   *SelectionNode
}

type SelectionNode struct {
	Terminal   bool
	SourceName string
	Children   map[string]*SelectionNode
}

type Predicate func(entity readmodel.EntityView) bool
