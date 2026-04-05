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
