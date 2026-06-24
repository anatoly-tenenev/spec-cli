package catalog

import (
	"sort"
	"strings"

	gqlmodel "github.com/anatoly-tenenev/spec-cli/internal/application/graphql/model"
)

func Render(proj *gqlmodel.Projection) string {
	if proj == nil {
		return ""
	}
	lines := []string{
		"GraphQL help",
		"",
		"Use this flow:",
		"  1. Pick the entity from the catalog below.",
		"  2. Run `spec-cli graphql-help --schema-only --entity <entity>` to get the exact GraphQL SDL.",
		"  3. Build the query.",
		"  4. Run it with `spec-cli graphql-query --file <query.graphql>`.",
		"",
		"Catalog is for discovery only. Use SDL from --schema-only to write exact GraphQL queries.",
		"",
		"Entities",
		"",
	}
	for _, name := range proj.EntityOrder {
		entity := proj.Entities[name]
		lines = append(lines, entity.Name)
		if strings.TrimSpace(entity.Description) != "" {
			lines = append(lines, "  description: "+entity.Description)
		}
		lines = append(lines, "  rootField: "+entity.Name)
		lines = append(lines, "  schema: spec-cli graphql-help --schema-only --entity "+entity.Name)
		lines = append(lines, "")
		lines = append(lines, "  meta:")
		if len(entity.MetaFields) == 0 {
			lines = append(lines, "    none")
		}
		for _, field := range entity.MetaFields {
			lines = append(lines, "    "+describe(field.Name, field.Description))
		}
		lines = append(lines, "")
		lines = append(lines, "  refs:")
		if len(entity.RefFields) == 0 {
			lines = append(lines, "    none")
		}
		for _, field := range entity.RefFields {
			targets := append([]string(nil), field.AllowedTypes...)
			sort.Strings(targets)
			targetText := strings.Join(targets, "|")
			if field.Cardinality == "array" {
				targetText += "[]"
			}
			lines = append(lines, "    "+field.Name+" -> "+targetText+descriptionSuffix(field.Description))
		}
		lines = append(lines, "")
		lines = append(lines, "  content:")
		lines = append(lines, "    raw - full body text")
		lines = append(lines, "    sections:")
		if len(entity.Sections) == 0 {
			lines = append(lines, "      none")
		}
		for _, section := range entity.Sections {
			lines = append(lines, "      "+describe(section.Name, section.Description))
		}
		lines = append(lines, "")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
}

func describe(name string, description string) string {
	if strings.TrimSpace(description) == "" {
		return name
	}
	return name + " - " + description
}

func descriptionSuffix(description string) string {
	if strings.TrimSpace(description) == "" {
		return ""
	}
	return " - " + description
}
