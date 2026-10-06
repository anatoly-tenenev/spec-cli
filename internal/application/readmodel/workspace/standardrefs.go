// standardrefs.go holds the clauses of the standard the workspace loader
// names when a document cannot be read. Keeping the references here is what
// gives one wording per rule; the failure itself is built by
// internal/application/readissues, which get uses too.

package workspace

const (
	frontmatterStandardRef = "11"
	typeStandardRef        = "5.3"
	idStandardRef          = "11.1"
	slugStandardRef        = "11.2"
	refsStandardRef        = "6"
)
