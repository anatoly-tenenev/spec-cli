// Package model holds validate's internal types: the parsed options, a
// candidate document, and the run summary. The run counts candidates and
// checked entities separately, because a document that could not be parsed far
// enough to identify its type still has to appear in the totals.
package model

import domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"

type Options struct {
	TypeFilters      map[string]struct{}
	FailFast         bool
	WarningsAsErrors bool
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
