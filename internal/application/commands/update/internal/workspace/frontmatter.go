package workspace

import "github.com/anatoly-tenenev/spec-cli/internal/application/entitydoc"

// ParseWithNormalizedDates parses a document and normalizes the built-in date
// fields to strings, so that rewriting the document does not reformat dates
// that were not touched.
func ParseWithNormalizedDates(raw []byte) (map[string]any, string, error) {
	fields, body, err := entitydoc.ParseFrontmatter(raw)
	if err != nil {
		return nil, "", err
	}

	normalizeBuiltinDate(fields, "createdDate")
	normalizeBuiltinDate(fields, "updatedDate")

	return fields, body, nil
}

func normalizeBuiltinDate(frontmatter map[string]any, key string) {
	if value, ok := entitydoc.ReadStringField(frontmatter, key); ok {
		frontmatter[key] = value
	}
}
