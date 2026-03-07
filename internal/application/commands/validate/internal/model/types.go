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
	Name             string
	Type             string
	RefTypes         []string
	Enum             []any
	HasValue         bool
	Value            any
	Required         bool
	RequiredWhen     bool
	RequiredWhenExpr *expressions.Expression
	RequiredWhenPath string
}

type RequiredSectionRule struct {
	Name             string
	Required         bool
	RequiredWhen     bool
	RequiredWhenExpr *expressions.Expression
	RequiredWhenPath string
}

type PathPatternRule struct {
	Cases []PathPatternCase
}

type PathPatternCase struct {
	Use      string
	HasWhen  bool
	When     bool
	WhenExpr *expressions.Expression
	WhenPath string
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
