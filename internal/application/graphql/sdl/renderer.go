package sdl

import (
	"fmt"
	"sort"
	"strings"

	gqlmodel "github.com/anatoly-tenenev/spec-cli/internal/application/graphql/model"
)

func Render(proj *gqlmodel.Projection, selected []string) string {
	if proj == nil {
		return ""
	}
	entityOrder := selectedEntities(proj, selected)
	var b strings.Builder
	writeCommon(&b)
	writeEntityTypeEnum(&b, proj)
	writeQuery(&b, proj, entityOrder)
	writePageInfo(&b)
	for _, name := range entityOrder {
		entity := proj.Entities[name]
		writeEntity(&b, entity)
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func selectedEntities(proj *gqlmodel.Projection, selected []string) []string {
	if len(selected) == 0 {
		return append([]string(nil), proj.EntityOrder...)
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(selected))
	for _, name := range selected {
		if _, ok := proj.Entities[name]; !ok {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func writeCommon(b *strings.Builder) {
	lines := []string{
		`"""Date string in YYYY-MM-DD format."""`,
		"scalar Date",
		"",
		`directive @requiredWhen(expr: String!) on FIELD_DEFINITION`,
		`directive @arrayConstraints(minItems: Int, maxItems: Int, uniqueItems: Boolean) on FIELD_DEFINITION`,
		"",
		"enum SortDirection {",
		"  asc",
		"  desc",
		"}",
		"",
		"input StringFilter { eq: String neq: String in: [String!] notIn: [String!] contains: String exists: Boolean notExists: Boolean }",
		"input IDFilter { eq: ID neq: ID in: [ID!] notIn: [ID!] exists: Boolean notExists: Boolean }",
		"input DateFilter { eq: Date neq: Date in: [Date!] notIn: [Date!] gt: Date gte: Date lt: Date lte: Date exists: Boolean notExists: Boolean }",
		"input IntFilter { eq: Int neq: Int in: [Int!] notIn: [Int!] gt: Int gte: Int lt: Int lte: Int exists: Boolean notExists: Boolean }",
		"input FloatFilter { eq: Float neq: Float in: [Float!] notIn: [Float!] gt: Float gte: Float lt: Float lte: Float exists: Boolean notExists: Boolean }",
		"input BooleanFilter { eq: Boolean neq: Boolean exists: Boolean notExists: Boolean }",
		"",
		"input StringArrayFilter { exists: Boolean notExists: Boolean contains: String any: StringFilter all: StringFilter none: StringFilter }",
		"input IDArrayFilter { exists: Boolean notExists: Boolean contains: ID any: IDFilter all: IDFilter none: IDFilter }",
		"input DateArrayFilter { exists: Boolean notExists: Boolean contains: Date any: DateFilter all: DateFilter none: DateFilter }",
		"input IntArrayFilter { exists: Boolean notExists: Boolean contains: Int any: IntFilter all: IntFilter none: IntFilter }",
		"input FloatArrayFilter { exists: Boolean notExists: Boolean contains: Float any: FloatFilter all: FloatFilter none: FloatFilter }",
		"input BooleanArrayFilter { exists: Boolean notExists: Boolean contains: Boolean any: BooleanFilter all: BooleanFilter none: BooleanFilter }",
		"",
	}
	b.WriteString(strings.Join(lines, "\n"))
}

func writeEntityTypeEnum(b *strings.Builder, proj *gqlmodel.Projection) {
	b.WriteString("enum EntityType {\n")
	for _, name := range proj.EntityOrder {
		b.WriteString("  " + name + "\n")
	}
	b.WriteString("}\n\n")
}

func writeQuery(b *strings.Builder, proj *gqlmodel.Projection, order []string) {
	b.WriteString("type Query {\n")
	for _, name := range order {
		entity := proj.Entities[name]
		writeDescription(b, entity.Description, "  ")
		b.WriteString(fmt.Sprintf("  %s(where: %s, sort: [%s!], limit: Int = 100, offset: Int = 0): %s!\n",
			entity.Name, entity.WhereName, entity.SortName, entity.ListName))
	}
	b.WriteString("}\n\n")
}

func writePageInfo(b *strings.Builder) {
	lines := []string{
		"type PageInfo {",
		"  limit: Int!",
		"  offset: Int!",
		"  returned: Int!",
		"  hasMore: Boolean!",
		"  nextOffset: Int",
		"}",
		"",
	}
	b.WriteString(strings.Join(lines, "\n"))
}

func writeEntity(b *strings.Builder, entity gqlmodel.Entity) {
	writeEnumDefinitions(b, entity)
	writeRefDefinitions(b, entity)
	writeObjectTypes(b, entity)
	writeWhereInputs(b, entity)
	writeSortInputs(b, entity)
}

func writeEnumDefinitions(b *strings.Builder, entity gqlmodel.Entity) {
	for _, field := range entity.MetaFields {
		if field.EnumName == "" {
			continue
		}
		b.WriteString("enum " + field.EnumName + " {\n")
		for _, value := range field.EnumValues {
			b.WriteString("  " + value + "\n")
		}
		b.WriteString("}\n\n")
		b.WriteString("input " + field.EnumName + "Filter { eq: " + field.EnumName + " neq: " + field.EnumName + " in: [" + field.EnumName + "!] notIn: [" + field.EnumName + "!] exists: Boolean notExists: Boolean }\n")
		if field.IsArray {
			b.WriteString("input " + field.EnumName + "ArrayFilter { exists: Boolean notExists: Boolean contains: " + field.EnumName + " any: " + field.EnumName + "Filter all: " + field.EnumName + "Filter none: " + field.EnumName + "Filter }\n")
		}
		b.WriteString("\n")
	}
	for _, ref := range entity.RefFields {
		b.WriteString("enum " + ref.EnumName + " {\n")
		for _, value := range ref.AllowedTypes {
			b.WriteString("  " + value + "\n")
		}
		b.WriteString("}\n\n")
		b.WriteString("input " + ref.EnumName + "Filter { eq: " + ref.EnumName + " neq: " + ref.EnumName + " in: [" + ref.EnumName + "!] notIn: [" + ref.EnumName + "!] exists: Boolean notExists: Boolean }\n\n")
	}
}

func writeRefDefinitions(b *strings.Builder, entity gqlmodel.Entity) {
	for _, ref := range entity.RefFields {
		writeDescription(b, refDescription(ref), "")
		b.WriteString("type " + ref.TypeName + " {\n")
		b.WriteString("  id: ID!\n")
		b.WriteString("  resolved: Boolean!\n")
		b.WriteString("  type: " + ref.EnumName + "!\n")
		b.WriteString("  slug: String!\n")
		b.WriteString("}\n\n")
		b.WriteString("input " + ref.TypeName + "Match { id: ID type: " + ref.EnumName + " slug: String }\n")
		b.WriteString("input " + ref.TypeName + "Where { id: IDFilter resolved: BooleanFilter type: " + ref.EnumName + "Filter slug: StringFilter }\n")
		if ref.Cardinality == "array" {
			b.WriteString("input " + ref.ArrayFilterName + " { exists: Boolean notExists: Boolean contains: " + ref.TypeName + "Match any: " + ref.TypeName + "Where all: " + ref.TypeName + "Where none: " + ref.TypeName + "Where }\n")
		}
		b.WriteString("\n")
	}
}

func writeObjectTypes(b *strings.Builder, entity gqlmodel.Entity) {
	writeDescription(b, entity.Description, "")
	b.WriteString("type " + entity.TypeName + " {\n")
	b.WriteString("  type: EntityType!\n")
	b.WriteString("  id: ID!\n")
	b.WriteString("  slug: String!\n")
	b.WriteString("  revision: String!\n")
	b.WriteString("  createdDate: Date!\n")
	b.WriteString("  updatedDate: Date!\n")
	b.WriteString("  meta: " + entity.MetaName + "\n")
	b.WriteString("  refs: " + entity.RefsName + "\n")
	b.WriteString("  content: " + entity.ContentName + "\n")
	b.WriteString("}\n\n")
	b.WriteString("type " + entity.ListName + " { items: [" + entity.TypeName + "!]! totalCount: Int! pageInfo: PageInfo! }\n\n")

	b.WriteString("type " + entity.MetaName + " {\n")
	if len(entity.MetaFields) == 0 {
		b.WriteString("  _empty: Boolean\n")
	}
	for _, field := range entity.MetaFields {
		writeDescription(b, field.Description, "  ")
		b.WriteString("  " + field.Name + ": " + outputType(field.TypeName, field.IsArray, field.Required))
		writeRequiredWhenDirective(b, field.RequiredWhen)
		writeArrayDirective(b, field.MinItems, field.MaxItems, field.UniqueItems)
		b.WriteString("\n")
	}
	b.WriteString("}\n\n")

	b.WriteString("type " + entity.RefsName + " {\n")
	if len(entity.RefFields) == 0 {
		b.WriteString("  _empty: Boolean\n")
	}
	for _, ref := range entity.RefFields {
		writeDescription(b, ref.Description, "  ")
		b.WriteString("  " + ref.Name + ": " + outputType(ref.TypeName, ref.Cardinality == "array", ref.Required))
		writeRequiredWhenDirective(b, ref.RequiredWhen)
		writeArrayDirective(b, ref.MinItems, ref.MaxItems, ref.UniqueItems)
		b.WriteString("\n")
	}
	b.WriteString("}\n\n")

	b.WriteString("type " + entity.ContentName + " { raw: String sections: " + entity.SectionsName + " }\n\n")
	b.WriteString("type " + entity.SectionsName + " {\n")
	if len(entity.Sections) == 0 {
		b.WriteString("  _empty: Boolean\n")
	}
	for _, section := range entity.Sections {
		writeDescription(b, section.Description, "  ")
		b.WriteString("  " + section.Name + ": String")
		if section.Required {
			b.WriteString("!")
		}
		writeRequiredWhenDirective(b, section.RequiredWhen)
		b.WriteString("\n")
	}
	b.WriteString("}\n\n")
}

func writeWhereInputs(b *strings.Builder, entity gqlmodel.Entity) {
	b.WriteString("input " + entity.WhereName + " {\n")
	b.WriteString("  and: [" + entity.WhereName + "!]\n")
	b.WriteString("  or: [" + entity.WhereName + "!]\n")
	b.WriteString("  not: " + entity.WhereName + "\n")
	b.WriteString("  id: IDFilter\n")
	b.WriteString("  slug: StringFilter\n")
	b.WriteString("  revision: StringFilter\n")
	b.WriteString("  createdDate: DateFilter\n")
	b.WriteString("  updatedDate: DateFilter\n")
	b.WriteString("  meta: " + entity.TypeName + "MetaWhere\n")
	b.WriteString("  refs: " + entity.TypeName + "RefsWhere\n")
	b.WriteString("  content: " + entity.TypeName + "ContentWhere\n")
	b.WriteString("}\n\n")

	b.WriteString("input " + entity.TypeName + "MetaWhere {\n")
	if len(entity.MetaFields) == 0 {
		b.WriteString("  _empty: Boolean\n")
	}
	for _, field := range entity.MetaFields {
		b.WriteString("  " + field.Name + ": " + filterType(field) + "\n")
	}
	b.WriteString("}\n\n")

	b.WriteString("input " + entity.TypeName + "RefsWhere {\n")
	if len(entity.RefFields) == 0 {
		b.WriteString("  _empty: Boolean\n")
	}
	for _, ref := range entity.RefFields {
		filter := ref.TypeName + "Where"
		if ref.Cardinality == "array" {
			filter = ref.ArrayFilterName
		}
		b.WriteString("  " + ref.Name + ": " + filter + "\n")
	}
	b.WriteString("}\n\n")

	b.WriteString("input " + entity.TypeName + "ContentWhere { raw: StringFilter sections: " + entity.TypeName + "SectionsWhere }\n\n")
	b.WriteString("input " + entity.TypeName + "SectionsWhere {\n")
	if len(entity.Sections) == 0 {
		b.WriteString("  _empty: Boolean\n")
	}
	for _, section := range entity.Sections {
		b.WriteString("  " + section.Name + ": StringFilter\n")
	}
	b.WriteString("}\n\n")
}

func writeSortInputs(b *strings.Builder, entity gqlmodel.Entity) {
	b.WriteString("enum " + entity.SortFieldName + " {\n")
	for _, field := range entity.SortFields {
		b.WriteString("  " + field.Name + "\n")
	}
	b.WriteString("}\n\n")
	b.WriteString("input " + entity.SortName + " { field: " + entity.SortFieldName + "! direction: SortDirection = asc }\n\n")
}

func outputType(typeName string, isArray bool, required bool) string {
	result := typeName
	if isArray {
		result = "[" + result + "!]"
	}
	if required {
		result += "!"
	}
	return result
}

func filterType(field gqlmodel.MetaField) string {
	if field.EnumName != "" {
		if field.IsArray {
			return field.EnumName + "ArrayFilter"
		}
		return field.EnumName + "Filter"
	}
	name := field.TypeName
	switch name {
	case "Date":
		name = "Date"
	case "Float":
		name = "Float"
	case "Boolean":
		name = "Boolean"
	case "Int":
		name = "Int"
	default:
		name = "String"
	}
	if field.IsArray {
		return name + "ArrayFilter"
	}
	return name + "Filter"
}

func writeArrayDirective(b *strings.Builder, minItems *int, maxItems *int, uniqueItems bool) {
	parts := []string{}
	if minItems != nil {
		parts = append(parts, fmt.Sprintf("minItems: %d", *minItems))
	}
	if maxItems != nil {
		parts = append(parts, fmt.Sprintf("maxItems: %d", *maxItems))
	}
	if uniqueItems {
		parts = append(parts, "uniqueItems: true")
	}
	if len(parts) > 0 {
		b.WriteString(" @arrayConstraints(" + strings.Join(parts, ", ") + ")")
	}
}

func writeRequiredWhenDirective(b *strings.Builder, expr string) {
	if strings.TrimSpace(expr) == "" {
		return
	}
	b.WriteString(" @requiredWhen(expr: \"" + escapeGraphQLString(expr) + "\")")
}

func escapeGraphQLString(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
		"\t", `\t`,
	)
	return replacer.Replace(value)
}

func writeDescription(b *strings.Builder, description string, indent string) {
	if strings.TrimSpace(description) == "" {
		return
	}
	escaped := strings.ReplaceAll(description, `"""`, `\"\"\"`)
	b.WriteString(indent + `"""` + escaped + `"""` + "\n")
}

func refDescription(ref gqlmodel.RefField) string {
	targets := append([]string(nil), ref.AllowedTypes...)
	sort.Strings(targets)
	return strings.TrimSpace(strings.Join([]string{ref.Description, "Targets: " + strings.Join(targets, ", ")}, " "))
}
