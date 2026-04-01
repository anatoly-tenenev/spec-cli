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
	ContentSections map[string]SectionField
}

type Field struct {
	Name        string
	Kind        model.SchemaFieldKind
	ItemKind    model.SchemaFieldKind
	EnumValues  []any
	HasConst    bool
	ConstValue  any
	Required    bool
	IsEntityRef bool
	IsArrayRef  bool
	RefTypes    []string
}

type SectionField struct {
	Name     string
	Required bool
}
