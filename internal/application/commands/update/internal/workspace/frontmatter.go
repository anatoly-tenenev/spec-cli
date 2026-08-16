package workspace

import (
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/support"
	"github.com/anatoly-tenenev/spec-cli/internal/application/entitydoc"
)

// ParseFrontmatter parses a document and normalizes the built-in date fields to
// strings, so that rewriting the document does not reformat dates that were not
// touched.
func ParseFrontmatter(raw []byte) (map[string]any, string, error) {
	fields, body, err := entitydoc.ParseFrontmatter(raw)
	if err != nil {
		return nil, "", err
	}

	normalizeBuiltinDate(fields, "createdDate")
	normalizeBuiltinDate(fields, "updatedDate")

	return fields, body, nil
}

func ReadStringField(values map[string]any, key string) (string, bool) {
	return entitydoc.ReadStringField(values, key)
}

// BuildMeta keeps every frontmatter key that is not a built-in field.
func BuildMeta(frontmatter map[string]any) map[string]any {
	meta := map[string]any{}
	for key, value := range frontmatter {
		switch key {
		case "type", "id", "slug", "createdDate", "updatedDate":
			continue
		default:
			meta[key] = support.NormalizeValue(value)
		}
	}
	return meta
}

func normalizeBuiltinDate(frontmatter map[string]any, key string) {
	if value, ok := ReadStringField(frontmatter, key); ok {
		frontmatter[key] = value
	}
}
