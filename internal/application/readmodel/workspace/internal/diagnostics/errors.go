// Package diagnostics holds the clauses of the standard the workspace loader
// names when a document cannot be read. Keeping the references here is what
// gives one wording per rule; the failure itself is built by
// internal/application/readissues, which get uses too.
package diagnostics

const (
	FrontmatterStandardRef = "11"
	TypeStandardRef        = "5.3"
	IDStandardRef          = "11.1"
	SlugStandardRef        = "11.2"
	RefsStandardRef        = "6"
)
