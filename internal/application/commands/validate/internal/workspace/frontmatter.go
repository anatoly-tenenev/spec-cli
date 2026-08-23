package workspace

import (
	"github.com/anatoly-tenenev/spec-cli/internal/application/entitydoc"
	"github.com/anatoly-tenenev/spec-cli/internal/application/values"
)

// ParseFrontmatter normalizes decoded values before handing them to the
// validation engine. yaml.v3 decodes a plain YYYY-MM-DD scalar into time.Time,
// and the engine compares frontmatter against schema literals, which are always
// strings; without normalization a valid date fails its own `type: string`.
func ParseFrontmatter(raw []byte) (map[string]any, string, error) {
	frontmatter, body, err := entitydoc.ParseFrontmatter(raw)
	if err != nil {
		return nil, "", err
	}
	normalized, _ := values.NormalizeValue(frontmatter).(map[string]any)
	return normalized, body, nil
}

func ReadStringField(values map[string]any, key string) (string, bool) {
	return entitydoc.ReadStringField(values, key)
}

// ExtractSectionLabels maps every label present in the body to its heading
// title, plus the labels that repeated. A repeated label keeps the title of its
// first occurrence: validate reports on what the document declares, so the
// label stays visible even though the section itself is ambiguous.
func ExtractSectionLabels(body string) (map[string]string, []string) {
	layout := entitydoc.BuildSectionLayout(body)

	labels := make(map[string]string, len(layout.Ranges))
	for _, item := range layout.Ranges {
		if _, exists := labels[item.Label]; exists {
			continue
		}
		labels[item.Label] = item.Title
	}

	return labels, layout.DuplicateLabels()
}
