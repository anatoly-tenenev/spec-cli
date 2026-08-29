// Package model holds delete's internal types: the parsed options, the
// workspace snapshot the decision is made from, and a blocking reference.
// The snapshot keeps every document, not just the target, because deciding
// whether the target may go requires knowing who points at it.
package model

type Options struct {
	ID             string
	ExpectRevision string
	DryRun         bool
}

type Snapshot struct {
	WorkspacePath string
	Documents     []ParsedDocument
	TargetMatches []TargetMatch
}

type ParsedDocument struct {
	PathAbs     string
	Type        string
	ID          string
	Revision    string
	Frontmatter map[string]any
}

type TargetMatch struct {
	PathAbs string
	Raw     []byte
}

type BlockingReference struct {
	SourceID   string
	SourceType string
	Field      string
}
