package model

type Options struct {
	ID             string
	ExpectRevision string
	DryRun         bool
	Help           bool
}

type Schema struct {
	ReferenceSlotsByType map[string][]ReferenceSlot
}

type ReferenceSlotKind string

const (
	ReferenceSlotScalar ReferenceSlotKind = "scalar"
	ReferenceSlotArray  ReferenceSlotKind = "array"
)

type ReferenceSlot struct {
	FieldName string
	Kind      ReferenceSlotKind
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
