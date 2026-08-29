// Package reservedkeys names the schema and frontmatter keys the standard
// reserves. Callers compare against these constants instead of writing the
// literal, so a renamed key cannot be half-applied across the codebase.
package reservedkeys

const (
	SchemaKeyIDPrefix     = "idPrefix"
	SchemaKeyPathTemplate = "pathTemplate"
	SchemaTypeEntityRef   = "entityRef"

	BuiltinType        = "type"
	BuiltinID          = "id"
	BuiltinSlug        = "slug"
	BuiltinCreatedDate = "createdDate"
	BuiltinUpdatedDate = "updatedDate"

	RefPartDirPath = "dirPath"

	ModelTypeName    = "typeName"
	ModelFieldName   = "fieldName"
	ModelSectionName = "sectionName"
)
