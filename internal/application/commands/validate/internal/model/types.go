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
