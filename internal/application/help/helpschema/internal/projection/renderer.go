package projection

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
)

func Render(compiled model.CompiledSchema) (string, error) {
	root := newOrderedJSONObject()
	applyDescription(root, compiled.Description)
	entityTypes := newOrderedJSONObject()
	allEntityTypes := sortedCompiledEntityNames(compiled)
	for _, typeName := range allEntityTypes {
		entityTypes.add(typeName, buildEntityProjection(typeName, compiled.Entities[typeName], allEntityTypes))
	}
	root.add("entityTypes", entityTypes)
	return renderOrderedJSONObject(root)
}

func buildEntityProjection(typeName string, entity model.EntityType, allEntityTypes []string) *orderedJSONObject {
	entityProjection := newOrderedJSONObject()
	entityProjection.add("type", "object")
	applyDescription(entityProjection, entity.Description)
	properties := newOrderedJSONObject()

	typeField := newOrderedJSONObject()
	typeField.add("const", typeName)
	typeField.add("x-presence", "required")
	properties.add("type", typeField)
	properties.add("id", buildRequiredBuiltinStringField(""))
	properties.add("slug", buildRequiredBuiltinStringField(""))
	properties.add("revision", buildRequiredBuiltinStringField(""))
	properties.add("createdDate", buildRequiredBuiltinStringField("date"))
	properties.add("updatedDate", buildRequiredBuiltinStringField("date"))

	metaFields, refFields := splitEntityFields(entity)
	if len(metaFields) > 0 {
		properties.add("meta", buildMetaProjection(metaFields, allEntityTypes))
	}
	if len(refFields) > 0 {
		properties.add("refs", buildRefsProjection(refFields, allEntityTypes))
	}
	if len(entity.Sections) > 0 {
		properties.add("content", buildContentProjection(entity))
	}

	entityProjection.add("properties", properties)
	return entityProjection
}

func buildRequiredBuiltinStringField(format string) *orderedJSONObject {
	field := newOrderedJSONObject()
	field.add("type", "string")
	if strings.TrimSpace(format) != "" {
		field.add("format", format)
	}
	field.add("x-presence", "required")
	return field
}

func buildMetaProjection(fields []fieldProjection, allEntityTypes []string) *orderedJSONObject {
	meta := newOrderedJSONObject()
	meta.add("type", "object")
	properties := newOrderedJSONObject()
	for _, field := range fields {
		fieldProjection := buildValueProjection(field.Field.Value, valueProjectionOptions{
			allEntityTypes: allEntityTypes,
		})
		applyDescription(fieldProjection, field.Field.Description)
		applyPresence(fieldProjection, field.Field.Required)
		properties.add(field.Name, fieldProjection)
	}
	meta.add("properties", properties)
	return meta
}

func buildRefsProjection(fields []fieldProjection, allEntityTypes []string) *orderedJSONObject {
	refs := newOrderedJSONObject()
	refs.add("type", "object")
	properties := newOrderedJSONObject()
	for _, field := range fields {
		fieldProjection := buildValueProjection(field.Field.Value, valueProjectionOptions{
			allEntityTypes: allEntityTypes,
		})
		applyDescription(fieldProjection, field.Field.Description)
		applyPresence(fieldProjection, field.Field.Required)
		properties.add(field.Name, fieldProjection)
	}
	refs.add("properties", properties)
	return refs
}

func buildContentProjection(entity model.EntityType) *orderedJSONObject {
	content := newOrderedJSONObject()
	content.add("type", "object")
	contentProperties := newOrderedJSONObject()
	raw := newOrderedJSONObject()
	raw.add("type", "string")
	contentProperties.add("raw", raw)
	sections := newOrderedJSONObject()
	sections.add("type", "object")
	sectionProperties := newOrderedJSONObject()
	for _, sectionName := range orderedSectionNames(entity) {
		sectionProperties.add(sectionName, buildSectionProjection(entity.Sections[sectionName]))
	}
	sections.add("properties", sectionProperties)
	contentProperties.add("sections", sections)
	content.add("properties", contentProperties)
	return content
}

func buildSectionProjection(section model.Section) *orderedJSONObject {
	result := newOrderedJSONObject()
	result.add("type", "string")
	applyDescription(result, section.Description)
	applyPresence(result, section.Required)
	applyCanonicalSectionTitle(result, section)
	return result
}

type valueProjectionOptions struct {
	allEntityTypes []string
}

func buildValueProjection(spec model.ValueSpec, options valueProjectionOptions) *orderedJSONObject {
	result := newOrderedJSONObject()
	if valueType, hasType := projectionValueType(spec.Kind); hasType {
		result.add("type", valueType)
	}
	if strings.TrimSpace(spec.Format) != "" {
		result.add("format", spec.Format)
	}
	applyConstProjection(result, spec.Const)
	applyEnumProjection(result, spec.Enum)
	if spec.Items != nil {
		result.add("items", buildValueProjection(*spec.Items, valueProjectionOptions{
			allEntityTypes: options.allEntityTypes,
		}))
	}
	if spec.UniqueItems {
		result.add("uniqueItems", true)
	}
	if spec.MinItems != nil {
		result.add("minItems", *spec.MinItems)
	}
	if spec.MaxItems != nil {
		result.add("maxItems", *spec.MaxItems)
	}
	if spec.Ref != nil {
		result.add("x-kind", "entityRef")
		result.add("x-refTypes", normalizedProjectionRefTypes(spec.Ref.AllowedTypes, options.allEntityTypes))
	}
	return result
}

func normalizedProjectionRefTypes(allowedTypes []string, allEntityTypes []string) []string {
	if len(allowedTypes) == 0 {
		return append([]string(nil), allEntityTypes...)
	}
	return append([]string(nil), allowedTypes...)
}

func applyPresence(result *orderedJSONObject, requirement model.Requirement) {
	result.add("x-presence", presenceLabel(requirement))
	if requirement.Expr != nil {
		result.add("x-requiredWhen", requirement.Expr.Source)
	}
}

func applyDescription(result *orderedJSONObject, description string) {
	if strings.TrimSpace(description) == "" {
		return
	}
	result.add("description", description)
}

func applyCanonicalSectionTitle(result *orderedJSONObject, section model.Section) {
	if len(section.Titles) == 0 {
		return
	}
	canonicalTitle := section.Titles[0]
	if strings.TrimSpace(canonicalTitle) == "" {
		return
	}
	result.add("title", canonicalTitle)
}

func applyConstProjection(result *orderedJSONObject, constValue *model.Literal) {
	if constValue == nil {
		return
	}
	if isDynamicLiteral(*constValue) {
		result.add("x-const", interpolationProjection(*constValue))
		return
	}
	result.add("const", constValue.Value)
}

func applyEnumProjection(result *orderedJSONObject, enumValues []model.Literal) {
	if len(enumValues) == 0 {
		return
	}

	allStatic := true
	for _, enumValue := range enumValues {
		if isDynamicLiteral(enumValue) {
			allStatic = false
			break
		}
	}

	if allStatic {
		staticValues := make([]any, 0, len(enumValues))
		for _, enumValue := range enumValues {
			staticValues = append(staticValues, enumValue.Value)
		}
		result.add("enum", staticValues)
		return
	}

	projectedValues := make([]any, 0, len(enumValues))
	for _, enumValue := range enumValues {
		projectedValues = append(projectedValues, literalProjectionValue(enumValue))
	}
	result.add("x-enum", projectedValues)
}

func literalProjectionValue(literal model.Literal) map[string]any {
	if isDynamicLiteral(literal) {
		return interpolationProjection(literal)
	}
	return map[string]any{
		"kind":  "literal",
		"value": literal.Value,
	}
}

func interpolationProjection(literal model.Literal) map[string]any {
	return map[string]any{
		"kind":   "interpolation",
		"source": interpolationSource(literal),
	}
}

func interpolationSource(literal model.Literal) string {
	if literal.Template != nil && literal.Template.Raw != "" {
		return literal.Template.Raw
	}
	source, _ := literal.Value.(string)
	return source
}

func isDynamicLiteral(literal model.Literal) bool {
	return literal.Template != nil
}

func projectionValueType(kind model.ValueKind) (string, bool) {
	switch kind {
	case model.ValueKindString:
		return "string", true
	case model.ValueKindNumber:
		return "number", true
	case model.ValueKindInteger:
		return "integer", true
	case model.ValueKindBoolean:
		return "boolean", true
	case model.ValueKindArray:
		return "array", true
	case model.ValueKindEntityRef:
		return "", false
	default:
		return "string", true
	}
}

type orderedJSONField struct {
	Key   string
	Value any
}

type orderedJSONObject struct {
	Fields []orderedJSONField
}

func newOrderedJSONObject() *orderedJSONObject {
	return &orderedJSONObject{Fields: []orderedJSONField{}}
}

func (o *orderedJSONObject) add(key string, value any) {
	o.Fields = append(o.Fields, orderedJSONField{
		Key:   key,
		Value: value,
	})
}

func renderOrderedJSONObject(value *orderedJSONObject) (string, error) {
	if value == nil {
		return "{}", nil
	}
	var builder strings.Builder
	if err := writeOrderedJSONObject(&builder, value, 0); err != nil {
		return "", err
	}
	return builder.String(), nil
}

func writeOrderedJSONObject(builder *strings.Builder, value *orderedJSONObject, indent int) error {
	builder.WriteString("{")
	if len(value.Fields) == 0 {
		builder.WriteString("}")
		return nil
	}

	builder.WriteString("\n")
	for idx, field := range value.Fields {
		builder.WriteString(strings.Repeat(" ", indent+2))
		keyJSON, err := json.Marshal(field.Key)
		if err != nil {
			return err
		}
		builder.WriteString(string(keyJSON))
		builder.WriteString(": ")
		if err := writeOrderedJSONValue(builder, field.Value, indent+2); err != nil {
			return err
		}
		if idx < len(value.Fields)-1 {
			builder.WriteString(",")
		}
		builder.WriteString("\n")
	}

	builder.WriteString(strings.Repeat(" ", indent))
	builder.WriteString("}")
	return nil
}

func writeOrderedJSONValue(builder *strings.Builder, value any, indent int) error {
	if objectValue, ok := value.(*orderedJSONObject); ok {
		return writeOrderedJSONObject(builder, objectValue, indent)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	builder.WriteString(string(raw))
	return nil
}

type fieldProjection struct {
	Name  string
	Field model.MetaField
}

func splitEntityFields(entity model.EntityType) ([]fieldProjection, []fieldProjection) {
	metaFields := []fieldProjection{}
	scalarRefFields := []fieldProjection{}
	arrayRefFields := []fieldProjection{}

	for _, fieldName := range orderedMetaFieldNames(entity) {
		field := entity.MetaFields[fieldName]
		entry := fieldProjection{Name: fieldName, Field: field}
		if isReferenceField(field) {
			if isArrayReferenceField(field) {
				arrayRefFields = append(arrayRefFields, entry)
			} else {
				scalarRefFields = append(scalarRefFields, entry)
			}
			continue
		}
		metaFields = append(metaFields, entry)
	}

	refFields := make([]fieldProjection, 0, len(scalarRefFields)+len(arrayRefFields))
	refFields = append(refFields, scalarRefFields...)
	refFields = append(refFields, arrayRefFields...)
	return metaFields, refFields
}

func isReferenceField(field model.MetaField) bool {
	if field.Value.Ref != nil {
		return true
	}
	return field.Value.Kind == model.ValueKindArray &&
		field.Value.Items != nil &&
		field.Value.Items.Ref != nil
}

func isArrayReferenceField(field model.MetaField) bool {
	return field.Value.Kind == model.ValueKindArray &&
		field.Value.Items != nil &&
		field.Value.Items.Ref != nil
}

func presenceLabel(requirement model.Requirement) string {
	if requirement.Expr != nil {
		return "conditional"
	}
	if requirement.Always {
		return "required"
	}
	return "optional"
}

func sortedCompiledEntityNames(compiled model.CompiledSchema) []string {
	names := make([]string, 0, len(compiled.Entities))
	for name := range compiled.Entities {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func orderedMetaFieldNames(entity model.EntityType) []string {
	order := make([]string, 0, len(entity.MetaFields))
	seen := make(map[string]struct{}, len(entity.MetaFields))
	for _, fieldName := range entity.MetaFieldOrder {
		if _, exists := entity.MetaFields[fieldName]; !exists {
			continue
		}
		if _, exists := seen[fieldName]; exists {
			continue
		}
		order = append(order, fieldName)
		seen[fieldName] = struct{}{}
	}
	remaining := make([]string, 0, len(entity.MetaFields)-len(order))
	for fieldName := range entity.MetaFields {
		if _, exists := seen[fieldName]; exists {
			continue
		}
		remaining = append(remaining, fieldName)
	}
	sort.Strings(remaining)
	order = append(order, remaining...)
	return order
}

func orderedSectionNames(entity model.EntityType) []string {
	order := make([]string, 0, len(entity.Sections))
	seen := make(map[string]struct{}, len(entity.Sections))
	for _, sectionName := range entity.SectionOrder {
		if _, exists := entity.Sections[sectionName]; !exists {
			continue
		}
		if _, exists := seen[sectionName]; exists {
			continue
		}
		order = append(order, sectionName)
		seen[sectionName] = struct{}{}
	}
	remaining := make([]string, 0, len(entity.Sections)-len(order))
	for sectionName := range entity.Sections {
		if _, exists := seen[sectionName]; exists {
			continue
		}
		remaining = append(remaining, sectionName)
	}
	sort.Strings(remaining)
	order = append(order, remaining...)
	return order
}
