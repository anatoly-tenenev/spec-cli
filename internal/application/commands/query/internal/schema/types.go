package schema

import "github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"

type LoadedSchema struct {
	RawText     string
	EntityTypes map[string]EntityType
}

type EntityType struct {
	Name            string
	MetadataFields  map[string]Field
	EntityRefFields map[string]Field
	ContentSections map[string]struct{}
}

type Field struct {
	Name        string
	Kind        model.SchemaFieldKind
	EnumValues  []any
	IsEntityRef bool
}
