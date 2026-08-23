package workspace

import (
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/values"
	"github.com/anatoly-tenenev/spec-cli/internal/application/entitydoc"
)

func ParseFrontmatter(raw []byte) (map[string]any, string, error) {
	return entitydoc.ParseFrontmatter(raw)
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
			meta[key] = values.NormalizeValue(value)
		}
	}
	return meta
}
