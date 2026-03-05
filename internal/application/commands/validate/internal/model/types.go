package model

import domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"

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
	RequiredSections []string
}

type RequiredFieldRule struct {
	Name              string
	Type              string
	Enum              []any
	HasValue          bool
	Value             any
	ValueIsExpression bool
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
	CandidateEntities int
	CheckedEntities   int
	EntitiesValid     int
	Issues            []domainvalidation.Issue
}
