// Package model is the vocabulary the read layer is built from: the request
// options, the plan compiled out of them, the per-entity view the workspace
// loader produces, and the response shape. Keeping these types in one package
// is what lets planning, execution and workspace loading stay independent of
// each other.
//
// EntityView carries two projections of the same document - View is what may
// be returned, WhereContext is what a filter may see - because the read
// contract exposes references as resolved objects while filtering needs the
// raw shape behind them.
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
	// DuplicateSectionLabels lists section labels the document repeats. Such a
	// section carries no value; reading it fails where it is requested.
	DuplicateSectionLabels []string
}

// SectionAccess records how a query reaches content.sections, so that an
// ambiguous section only breaks the queries that actually touch it.
type SectionAccess struct {
	All   bool
	Names map[string]struct{}
}

// Touches reports whether the access set covers a label.
func (access SectionAccess) Touches(label string) bool {
	if access.All {
		return true
	}
	_, named := access.Names[label]
	return named
}

type WherePlan struct {
	Source   string
	Query    *jmespath.JMESPath
	Sections SectionAccess
}

type QueryPlan struct {
	SelectTree        *SelectNode
	Where             *WherePlan
	ActiveTypeSet     []string
	RootPlans         []RootPlan
	OriginalSelects   []string
	OriginalSortTerms []SortTerm
	SelectSections    SectionAccess
	WhereSections     SectionAccess
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
