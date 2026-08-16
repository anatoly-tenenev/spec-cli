package workspace

import "github.com/anatoly-tenenev/spec-cli/internal/application/entitydoc"

type SectionContent = entitydoc.SectionContent

func ExtractSections(body string) (map[string]SectionContent, []string) {
	return entitydoc.ExtractSections(body)
}
