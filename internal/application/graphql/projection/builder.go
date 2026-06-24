package projection

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	gqlmodel "github.com/anatoly-tenenev/spec-cli/internal/application/graphql/model"
	readcap "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/read"
	schemamodel "github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

var gqlNamePattern = regexp.MustCompile(`^[_A-Za-z][_0-9A-Za-z]*$`)
var gqlEntityPattern = regexp.MustCompile(`^[a-z][_0-9A-Za-z]*$`)

var reservedTypeNames = map[string]struct{}{
	"Query": {}, "PageInfo": {}, "Date": {}, "EntityType": {}, "SortDirection": {},
	"StringFilter": {}, "IDFilter": {}, "DateFilter": {}, "IntFilter": {}, "FloatFilter": {}, "BooleanFilter": {},
	"StringArrayFilter": {}, "IDArrayFilter": {}, "DateArrayFilter": {}, "IntArrayFilter": {}, "FloatArrayFilter": {}, "BooleanArrayFilter": {},
}

func Build(compiled schemamodel.CompiledSchema, capability readcap.Capability) (*gqlmodel.Projection, *domainerrors.AppError) {
	proj := &gqlmodel.Projection{
		EntityOrder: append([]string(nil), capability.EntityOrder...),
		Entities:    map[string]gqlmodel.Entity{},
	}
	typeNames := copyReservedTypes()

	for _, entityName := range proj.EntityOrder {
		entitySchema := compiled.Entities[entityName]
		entityCap := capability.EntityTypes[entityName]
		entity, err := buildEntity(entityName, entitySchema, entityCap, typeNames)
		if err != nil {
			return nil, err
		}
		proj.Entities[entityName] = entity
	}
	return proj, nil
}

func buildEntity(
	entityName string,
	entitySchema schemamodel.EntityType,
	entityCap readcap.EntityReadModel,
	typeNames map[string]string,
) (gqlmodel.Entity, *domainerrors.AppError) {
	if err := validateEntityName(entityName); err != nil {
		return gqlmodel.Entity{}, err
	}
	base := pascalCase(entityName)
	entity := gqlmodel.Entity{
		Name:          entityName,
		TypeName:      base,
		ListName:      base + "List",
		MetaName:      base + "Meta",
		RefsName:      base + "Refs",
		ContentName:   base + "Content",
		SectionsName:  base + "Sections",
		WhereName:     base + "Where",
		SortName:      base + "Sort",
		SortFieldName: base + "SortField",
		Description:   entitySchema.Description,
	}
	for _, name := range []string{
		entity.TypeName, entity.ListName, entity.MetaName, entity.RefsName,
		entity.ContentName, entity.SectionsName, entity.WhereName, entity.SortName, entity.SortFieldName,
	} {
		if err := registerTypeName(typeNames, name, "entity "+entityName); err != nil {
			return gqlmodel.Entity{}, err
		}
	}

	metaFields, err := buildMetaFields(base, entitySchema, entityCap, typeNames)
	if err != nil {
		return gqlmodel.Entity{}, err
	}
	refFields, err := buildRefFields(base, entitySchema, entityCap, typeNames)
	if err != nil {
		return gqlmodel.Entity{}, err
	}
	sections, err := buildSections(entitySchema)
	if err != nil {
		return gqlmodel.Entity{}, err
	}
	sortFields, err := buildSortFields(metaFields, refFields, sections)
	if err != nil {
		return gqlmodel.Entity{}, err
	}
	entity.MetaFields = metaFields
	entity.RefFields = refFields
	entity.Sections = sections
	entity.SortFields = sortFields
	return entity, nil
}

func buildMetaFields(
	base string,
	entitySchema schemamodel.EntityType,
	entityCap readcap.EntityReadModel,
	typeNames map[string]string,
) ([]gqlmodel.MetaField, *domainerrors.AppError) {
	fields := make([]gqlmodel.MetaField, 0, len(entityCap.MetaFields))
	for _, fieldName := range orderedMetaFields(entitySchema, entityCap) {
		if err := validateSourceName("meta field", fieldName); err != nil {
			return nil, err
		}
		sourceField := entitySchema.MetaFields[fieldName]
		capField := entityCap.MetaFields[fieldName]
		field := gqlmodel.MetaField{
			Name:         fieldName,
			Kind:         string(capField.Kind),
			ItemKind:     string(capField.ItemKind),
			Required:     capField.Required,
			RequiredWhen: requirementExpr(sourceField.Required),
			Description:  sourceField.Description,
			IsArray:      capField.Kind == readcap.FieldKindArray,
			MinItems:     sourceField.Value.MinItems,
			MaxItems:     sourceField.Value.MaxItems,
			UniqueItems:  sourceField.Value.UniqueItems,
		}
		if len(capField.EnumValues) > 0 && capField.Kind == readcap.FieldKindString {
			values, enumErr := enumValues("enum value", capField.EnumValues)
			if enumErr != nil {
				return nil, enumErr
			}
			field.EnumName = base + pascalCase(fieldName)
			field.TypeName = field.EnumName
			field.EnumValues = values
			if err := registerTypeName(typeNames, field.EnumName, "meta enum "+fieldName); err != nil {
				return nil, err
			}
		} else {
			field.TypeName = scalarType(capField.Kind)
		}
		if field.IsArray {
			field.TypeName = scalarType(capField.ItemKind)
		}
		fields = append(fields, field)
	}
	return fields, nil
}

func buildRefFields(
	base string,
	entitySchema schemamodel.EntityType,
	entityCap readcap.EntityReadModel,
	typeNames map[string]string,
) ([]gqlmodel.RefField, *domainerrors.AppError) {
	fields := make([]gqlmodel.RefField, 0, len(entityCap.RefFields))
	for _, fieldName := range orderedRefFields(entitySchema, entityCap) {
		if err := validateSourceName("ref field", fieldName); err != nil {
			return nil, err
		}
		sourceField := entitySchema.MetaFields[fieldName]
		capField := entityCap.RefFields[fieldName]
		refBase := base + pascalCase(fieldName) + "Ref"
		field := gqlmodel.RefField{
			Name:            fieldName,
			TypeName:        refBase,
			EnumName:        refBase + "Type",
			ArrayFilterName: refBase + "ArrayFilter",
			Required:        sourceField.Required.Always,
			RequiredWhen:    requirementExpr(sourceField.Required),
			Description:     sourceField.Description,
			Cardinality:     string(capField.Cardinality),
			AllowedTypes:    append([]string(nil), capField.AllowedTypes...),
			MinItems:        sourceField.Value.MinItems,
			MaxItems:        sourceField.Value.MaxItems,
			UniqueItems:     sourceField.Value.UniqueItems,
		}
		if sourceField.Value.Kind == schemamodel.ValueKindArray && sourceField.Value.Items != nil {
			field.MinItems = sourceField.Value.MinItems
			field.MaxItems = sourceField.Value.MaxItems
			field.UniqueItems = sourceField.Value.UniqueItems
		}
		if err := registerTypeName(typeNames, field.TypeName, "ref field "+fieldName); err != nil {
			return nil, err
		}
		if err := registerTypeName(typeNames, field.EnumName, "ref enum "+fieldName); err != nil {
			return nil, err
		}
		fields = append(fields, field)
	}
	return fields, nil
}

func buildSections(entitySchema schemamodel.EntityType) ([]gqlmodel.Section, *domainerrors.AppError) {
	sections := make([]gqlmodel.Section, 0, len(entitySchema.Sections))
	for _, sectionName := range orderedSections(entitySchema) {
		if err := validateSourceName("content section", sectionName); err != nil {
			return nil, err
		}
		sourceSection := entitySchema.Sections[sectionName]
		sections = append(sections, gqlmodel.Section{
			Name:         sectionName,
			Required:     sourceSection.Required.Always,
			RequiredWhen: requirementExpr(sourceSection.Required),
			Description:  sourceSection.Description,
		})
	}
	return sections, nil
}

func requirementExpr(requirement schemamodel.Requirement) string {
	if requirement.Expr == nil {
		return ""
	}
	return requirement.Expr.Source
}

func buildSortFields(
	meta []gqlmodel.MetaField,
	refs []gqlmodel.RefField,
	sections []gqlmodel.Section,
) ([]gqlmodel.SortField, *domainerrors.AppError) {
	fields := []gqlmodel.SortField{
		{Name: "id", Path: "id"},
		{Name: "slug", Path: "slug"},
		{Name: "revision", Path: "revision"},
		{Name: "createdDate", Path: "createdDate"},
		{Name: "updatedDate", Path: "updatedDate"},
	}
	seen := map[string]string{}
	for _, field := range fields {
		seen[field.Name] = field.Path
	}
	add := func(name, path string) *domainerrors.AppError {
		if existing, exists := seen[name]; exists && existing != path {
			return projectionError(
				fmt.Sprintf("GraphQL sort field collision: %s", name),
				map[string]any{"sort_field": name, "path": path, "existing_path": existing},
			)
		}
		seen[name] = path
		fields = append(fields, gqlmodel.SortField{Name: name, Path: path})
		return nil
	}
	for _, field := range meta {
		if field.IsArray {
			continue
		}
		if err := add("meta_"+field.Name, "meta."+field.Name); err != nil {
			return nil, err
		}
	}
	for _, ref := range refs {
		if ref.Cardinality != string(readcap.RefCardinalityScalar) {
			continue
		}
		for _, leaf := range []string{"id", "type", "slug", "resolved"} {
			if err := add("refs_"+ref.Name+"_"+leaf, "refs."+ref.Name+"."+leaf); err != nil {
				return nil, err
			}
		}
	}
	for _, section := range sections {
		if err := add("content_sections_"+section.Name, "content.sections."+section.Name); err != nil {
			return nil, err
		}
	}
	return fields, nil
}

func validateEntityName(name string) *domainerrors.AppError {
	if !gqlEntityPattern.MatchString(name) || strings.HasPrefix(name, "__") {
		return projectionError("entity name is not GraphQL-safe", map[string]any{"name": name})
	}
	return nil
}

func validateSourceName(kind, name string) *domainerrors.AppError {
	if !gqlNamePattern.MatchString(name) || strings.HasPrefix(name, "__") {
		return projectionError(kind+" name is not GraphQL-safe", map[string]any{"name": name})
	}
	return nil
}

func enumValues(kind string, raw []any) ([]string, *domainerrors.AppError) {
	values := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, value := range raw {
		text, ok := value.(string)
		if !ok {
			return nil, nil
		}
		if err := validateSourceName(kind, text); err != nil {
			return nil, err
		}
		if _, exists := seen[text]; exists {
			continue
		}
		seen[text] = struct{}{}
		values = append(values, text)
	}
	return values, nil
}

func registerTypeName(seen map[string]string, name string, owner string) *domainerrors.AppError {
	if !gqlNamePattern.MatchString(name) || strings.HasPrefix(name, "__") {
		return projectionError("generated GraphQL type name is not safe", map[string]any{"name": name})
	}
	if existing, exists := seen[name]; exists {
		return projectionError(
			fmt.Sprintf("generated GraphQL type name collision: %s", name),
			map[string]any{"name": name, "owner": owner, "existing_owner": existing},
		)
	}
	seen[name] = owner
	return nil
}

func copyReservedTypes() map[string]string {
	out := map[string]string{}
	for name := range reservedTypeNames {
		out[name] = "reserved"
	}
	return out
}

func scalarType(kind readcap.FieldKind) string {
	switch kind {
	case readcap.FieldKindDate:
		return "Date"
	case readcap.FieldKindNumber:
		return "Float"
	case readcap.FieldKindBoolean:
		return "Boolean"
	default:
		return "String"
	}
}

func pascalCase(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == '_' || r == '-' })
	var b strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]))
		if len(part) > 1 {
			b.WriteString(part[1:])
		}
	}
	if b.Len() == 0 {
		return value
	}
	return b.String()
}

func orderedMetaFields(entity schemamodel.EntityType, cap readcap.EntityReadModel) []string {
	return orderedFieldNames(entity.MetaFieldOrder, cap.MetaFields)
}

func orderedRefFields(entity schemamodel.EntityType, cap readcap.EntityReadModel) []string {
	return orderedFieldNames(entity.MetaFieldOrder, cap.RefFields)
}

func orderedSections(entity schemamodel.EntityType) []string {
	return orderedFieldNames(entity.SectionOrder, entity.Sections)
}

func orderedFieldNames[T any](declared []string, values map[string]T) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, name := range declared {
		if _, exists := values[name]; !exists {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	if len(out) == len(values) {
		return out
	}
	remaining := make([]string, 0, len(values)-len(out))
	for name := range values {
		if _, exists := seen[name]; exists {
			continue
		}
		remaining = append(remaining, name)
	}
	sort.Strings(remaining)
	return append(out, remaining...)
}

func projectionError(message string, details map[string]any) *domainerrors.AppError {
	return domainerrors.New(domainerrors.CodeGraphQLProjectionError, message, details)
}
