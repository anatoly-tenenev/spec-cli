package model

import (
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/expressions"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

type Options struct {
	TypeFilters      map[string]struct{}
	FailFast         bool
	WarningsAsErrors bool
}

type ValidationSchema struct {
	Entity map[string]SchemaEntityType
}

type SchemaEntityType struct {
	Name             string
	IDPrefix         string
	RequiredFields   []RequiredFieldRule
	RequiredSections []RequiredSectionRule
	PathPattern      PathPatternRule
}

type RequiredFieldRule struct {
	Name         string
	Type         string
	RefTypes     []string
	Enum         []RuleValue
	HasValue     bool
	Value        RuleValue
	HasItemType  bool
	ItemType     string
	ItemRefTypes []string
	UniqueItems  bool
	HasMinItems  bool
	MinItems     int
	HasMaxItems  bool
	MaxItems     int
	Required     bool
	RequiredExpr *expressions.CompiledExpression
	RequiredPath string
}

type RequiredSectionRule struct {
	Name         string
	HasTitle     bool
	Title        string
	Required     bool
	RequiredExpr *expressions.CompiledExpression
	RequiredPath string
}

type RuleValue struct {
	Literal  any
	Template *expressions.CompiledTemplate
}

type PathPatternRule struct {
	Cases []PathPatternCase
}

type PathPatternCase struct {
	Use         string
	UseTemplate *expressions.CompiledTemplate
	HasWhen     bool
	When        bool
	WhenExpr    *expressions.CompiledExpression
	WhenPath    string
}

type WorkspaceCandidate struct {
	Path string
}

type CheckedEntity struct {
	Type      string
	ID        string
	Slug      string
	HasSuffix bool
	IDSuffix  int
	HasError  bool
}

type ValidationRun struct {
	CandidateEntities   int
	CheckedEntities     int
	EntitiesValid       int
	ValidatorConformant bool
	Issues              []domainvalidation.Issue
}
