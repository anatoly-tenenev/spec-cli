package workspace

import "github.com/anatoly-tenenev/spec-cli/internal/application/entitydoc"

type (
	SectionContent = entitydoc.SectionContent
	SectionRange   = entitydoc.SectionRange
	SectionLayout  = entitydoc.SectionLayout
)

// BuildSectionLayout exposes the raw section layout, which update needs for
// surgical edits: it rewrites one section in place using the line ranges.
func BuildSectionLayout(body string) SectionLayout {
	return entitydoc.BuildSectionLayout(body)
}

func ExtractSections(body string) (map[string]SectionContent, []string) {
	return entitydoc.ExtractSections(body)
}
