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
	TypeFilters   []string
	WhereExpr     string
	Selects       []string
	Sorts         []SortTerm
	ScopedSorts   map[string][]SortTerm
	Limit         int
	ScopedLimits  map[string]int
	Offset        int
	ScopedOffsets map[string]int
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
	Where             *WherePlan
	ActiveTypeSet     []string
	RootPlans         []RootPlan
	OriginalSelects   []string
	OriginalSortTerms []SortTerm
}

type RootPlan struct {
	EntityType    string
	Limit         int
	Offset        int
	EffectiveSort []SortTerm
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
	RootFields  []QueryRootField
}

type QueryRootField struct {
	EntityType string
	Items      []map[string]any
	TotalCount int
	PageInfo   PageInfo
}

type JSONValue = json.RawMessage
