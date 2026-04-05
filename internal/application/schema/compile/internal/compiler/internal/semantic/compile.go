package semantic

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/compile/internal/compiler/internal/shared"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/diagnostics"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/expressioncontext"
	schemaexpressions "github.com/anatoly-tenenev/spec-cli/internal/application/schema/expressions"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/source"
	"gopkg.in/yaml.v3"
)

var (
	idPrefixPattern      = regexp.MustCompile(`^[A-Za-z0-9_]+(?:-[A-Za-z0-9_]+)*$`)
	schemaKeyNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)
)

func CompileDocument(doc source.Document) (model.CompiledSchema, []diagnostics.Issue) {
	issues := make([]diagnostics.Issue, 0)

	rootValues, ok := shared.MappingValues(doc.Root, "schema", &issues)
	if !ok {
		return model.CompiledSchema{}, issues
	}
	shared.AppendUnsupportedKeys(rootValues, "schema", shared.SetOf("version", "description", "entity"), &issues)

	compiled := model.CompiledSchema{Entities: map[string]model.EntityType{}}

	if versionNode, exists := rootValues["version"]; exists {
		if version, isValid := shared.ScalarString(versionNode, "schema.version", false, &issues); isValid {
			compiled.Version = version
		}
	} else {
		shared.AddWarning(&issues, "schema.top_level.version_missing", "schema.version is recommended", "schema.version")
	}

	if descriptionNode, exists := rootValues["description"]; exists {
		if description, isValid := shared.ScalarString(descriptionNode, "schema.description", false, &issues); isValid {
			compiled.Description = description
		}
	}

	entityNode, hasEntity := rootValues["entity"]
	if !hasEntity {
		shared.AddError(&issues, "schema.top_level.entity_required", "schema.entity is required", "schema.entity")
		return compiled, issues
	}

	entityValues, entityMapping := shared.MappingValues(entityNode, "schema.entity", &issues)
	if !entityMapping {
		return compiled, issues
	}
	if len(entityValues) == 0 {
		shared.AddError(&issues, "schema.top_level.entity_empty", "schema.entity must be a non-empty mapping", "schema.entity")
		return compiled, issues
	}

	typeNames := shared.SortedKeys(entityValues)
	typeSet := make(map[string]struct{}, len(typeNames))
	for _, typeName := range typeNames {
		typeSet[typeName] = struct{}{}
	}

	usedPrefixes := map[string]string{}
	for _, typeName := range typeNames {
		node := entityValues[typeName]
		path := "schema.entity." + typeName
		entity, entityOK := parseEntityType(typeName, node, path, typeSet, usedPrefixes, &issues)
		if entityOK {
			compiled.Entities[typeName] = entity
		}
	}

	return compiled, issues
}

func parseEntityType(
	typeName string,
	node *yaml.Node,
	path string,
	typeSet map[string]struct{},
	usedPrefixes map[string]string,
	issues *[]diagnostics.Issue,
) (model.EntityType, bool) {
	values, ok := shared.MappingValues(node, path, issues)
	if !ok {
		return model.EntityType{}, false
	}
	shared.AppendUnsupportedKeys(values, path, shared.SetOf("idPrefix", "pathTemplate", "meta", "content", "description"), issues)

	entity := model.EntityType{
		Name:       typeName,
		MetaFields: map[string]model.MetaField{},
		Sections:   map[string]model.Section{},
	}

	idPrefixNode, hasIDPrefix := values["idPrefix"]
	if !hasIDPrefix {
		shared.AddError(issues, "schema.entity.id_prefix_required", "idPrefix is required", path+".idPrefix")
	} else if idPrefix, isValid := shared.ScalarString(idPrefixNode, path+".idPrefix", true, issues); isValid {
		entity.IDPrefix = idPrefix
		if strings.Contains(idPrefix, "${") {
			shared.AddError(issues, "schema.entity.id_prefix_interpolation_forbidden", "idPrefix must not contain interpolation", path+".idPrefix")
		}
		if !idPrefixPattern.MatchString(idPrefix) {
			shared.AddError(issues, "schema.entity.id_prefix_format_invalid", "idPrefix has invalid format", path+".idPrefix")
		}
		if previousType, exists := usedPrefixes[idPrefix]; exists {
			shared.AddError(
				issues,
				"schema.entity.id_prefix_duplicate",
				fmt.Sprintf("idPrefix '%s' is already used by entity type '%s'", idPrefix, previousType),
				path+".idPrefix",
			)
		} else {
			usedPrefixes[idPrefix] = typeName
		}
	}

	if descriptionNode, exists := values["description"]; exists {
		if description, isValid := shared.ScalarString(descriptionNode, path+".description", false, issues); isValid {
			entity.Description = description
		}
	}

	if metaNode, exists := values["meta"]; exists {
		entity.MetaFields = parseMetaFields(metaNode, path+".meta", typeSet, issues)
	}
	if contentNode, exists := values["content"]; exists {
		entity.Sections = parseSections(contentNode, path+".content", issues)
	}

	expressionEngine := buildExpressionEngine(entity, path, issues)
	compileEntityExpressions(&entity, expressionEngine, issues)

	pathTemplateNode, hasPathTemplate := values["pathTemplate"]
	if !hasPathTemplate {
		shared.AddError(issues, "schema.entity.path_template_required", "pathTemplate is required", path+".pathTemplate")
	} else {
		entity.PathTemplate = parsePathTemplate(pathTemplateNode, path+".pathTemplate", expressionEngine, entity.MetaFields, issues)
	}

	return entity, true
}

func parseMetaFields(
	node *yaml.Node,
	path string,
	typeSet map[string]struct{},
	issues *[]diagnostics.Issue,
) map[string]model.MetaField {
	values, ok := shared.MappingValues(node, path, issues)
	if !ok {
		return map[string]model.MetaField{}
	}
	shared.AppendUnsupportedKeys(values, path, shared.SetOf("fields"), issues)

	fieldsNode, exists := values["fields"]
	if !exists {
		return map[string]model.MetaField{}
	}
	fieldValues, fieldsOK := shared.MappingValues(fieldsNode, path+".fields", issues)
	if !fieldsOK {
		return map[string]model.MetaField{}
	}

	result := make(map[string]model.MetaField, len(fieldValues))
	for _, fieldName := range shared.SortedKeys(fieldValues) {
		if !schemaKeyNamePattern.MatchString(fieldName) {
			shared.AddError(
				issues,
				"schema.meta_field.name_invalid",
				fmt.Sprintf("meta field name '%s' has invalid format", fieldName),
				path+".fields."+fieldName,
			)
			continue
		}
		if expressioncontext.IsBuiltinMetaField(fieldName) {
			shared.AddError(
				issues,
				"schema.meta_field.reserved_name",
				fmt.Sprintf("meta field name '%s' is reserved", fieldName),
				path+".fields."+fieldName,
			)
			continue
		}

		field, fieldOK := parseMetaField(fieldName, fieldValues[fieldName], path+".fields."+fieldName, typeSet, issues)
		if fieldOK {
			result[fieldName] = field
		}
	}

	return result
}

func parseMetaField(
	fieldName string,
	node *yaml.Node,
	path string,
	typeSet map[string]struct{},
	issues *[]diagnostics.Issue,
) (model.MetaField, bool) {
	values, ok := shared.MappingValues(node, path, issues)
	if !ok {
		return model.MetaField{}, false
	}
	shared.AppendUnsupportedKeys(values, path, shared.SetOf("schema", "required", "description"), issues)

	schemaNode, hasSchema := values["schema"]
	if !hasSchema {
		shared.AddError(issues, "schema.meta_field.schema_required", "meta field schema is required", path+".schema")
		return model.MetaField{}, false
	}

	valueSpec := shared.ParseValueSpec(schemaNode, path+".schema", typeSet, true, issues)
	required := shared.ParseRequirement(values["required"], path+".required", true, issues)
	description := ""
	if descriptionNode, exists := values["description"]; exists {
		if parsed, isValid := shared.ScalarString(descriptionNode, path+".description", true, issues); isValid {
			description = parsed
			if strings.Contains(parsed, "${") {
				shared.AddError(issues, "schema.meta_field.description_interpolation_forbidden", "description must not contain interpolation", path+".description")
			}
		}
	}

	return model.MetaField{
		Name:        fieldName,
		Value:       valueSpec,
		Required:    required,
		Description: description,
		SchemaPath:  path,
	}, true
}

func parseSections(node *yaml.Node, path string, issues *[]diagnostics.Issue) map[string]model.Section {
	values, ok := shared.MappingValues(node, path, issues)
	if !ok {
		return map[string]model.Section{}
	}
	shared.AppendUnsupportedKeys(values, path, shared.SetOf("sections"), issues)

	sectionsNode, exists := values["sections"]
	if !exists {
		return map[string]model.Section{}
	}
	sectionValues, sectionsOK := shared.MappingValues(sectionsNode, path+".sections", issues)
	if !sectionsOK {
		return map[string]model.Section{}
	}

	if len(sectionValues) == 0 {
		shared.AddError(issues, "schema.section.empty", "content.sections must be a non-empty mapping", path+".sections")
		return map[string]model.Section{}
	}

	result := make(map[string]model.Section, len(sectionValues))
	for _, sectionName := range shared.SortedKeys(sectionValues) {
		if !schemaKeyNamePattern.MatchString(sectionName) {
			shared.AddError(
				issues,
				"schema.section.name_invalid",
				fmt.Sprintf("section name '%s' has invalid format", sectionName),
				path+".sections."+sectionName,
			)
			continue
		}

		section, sectionOK := parseSection(sectionName, sectionValues[sectionName], path+".sections."+sectionName, issues)
		if sectionOK {
			result[sectionName] = section
		}
	}

	return result
}

func parseSection(sectionName string, node *yaml.Node, path string, issues *[]diagnostics.Issue) (model.Section, bool) {
	values, ok := shared.MappingValues(node, path, issues)
	if !ok {
		return model.Section{}, false
	}
	shared.AppendUnsupportedKeys(values, path, shared.SetOf("title", "required", "description"), issues)

	titles := make([]string, 0)
	titlePath := path + ".title"
	if titleNode, exists := values["title"]; exists {
		parsedTitles, isValid := shared.ParseTitles(titleNode, titlePath, issues)
		if isValid {
			for _, title := range parsedTitles {
				if strings.Contains(title, "${") {
					shared.AddError(issues, "schema.section.title_interpolation_forbidden", "title must not contain interpolation", titlePath)
					continue
				}
				titles = append(titles, title)
			}
		}
	}

	required := shared.ParseRequirement(values["required"], path+".required", true, issues)
	description := ""
	if descriptionNode, exists := values["description"]; exists {
		if parsed, isValid := shared.ScalarString(descriptionNode, path+".description", true, issues); isValid {
			description = parsed
			if strings.Contains(parsed, "${") {
				shared.AddError(issues, "schema.section.description_interpolation_forbidden", "description must not contain interpolation", path+".description")
			}
		}
	}

	return model.Section{
		Name:        sectionName,
		Titles:      titles,
		Required:    required,
		Description: description,
		SchemaPath:  path,
		TitlePath:   titlePath,
	}, true
}

func buildExpressionEngine(entity model.EntityType, entityPath string, issues *[]diagnostics.Issue) *schemaexpressions.Engine {
	schema := expressioncontext.BuildEntityExpressionSchema(entity)
	engine, compileErr := schemaexpressions.NewSchemaAwareEngine(entity.Name, schema)
	if compileErr == nil {
		return engine
	}

	shared.AddError(
		issues,
		"schema.expression.context_invalid",
		fmt.Sprintf("failed to compile expression context schema: %s", compileErr.Message),
		entityPath,
	)
	return nil
}

func compileEntityExpressions(entity *model.EntityType, engine *schemaexpressions.Engine, issues *[]diagnostics.Issue) {
	if entity == nil || engine == nil {
		return
	}

	for _, fieldName := range sortedMetaFieldNames(entity.MetaFields) {
		field := entity.MetaFields[fieldName]
		field.Required = compileRequirement(field.Required, engine, issues)
		field.Value = compileValueInterpolations(field.Value, engine, field.SchemaPath+".schema", issues)
		entity.MetaFields[fieldName] = field
	}

	for _, sectionName := range sortedSectionNames(entity.Sections) {
		section := entity.Sections[sectionName]
		section.Required = compileRequirement(section.Required, engine, issues)
		entity.Sections[sectionName] = section
	}
}

func compileRequirement(requirement model.Requirement, engine *schemaexpressions.Engine, issues *[]diagnostics.Issue) model.Requirement {
	if requirement.Expr == nil || engine == nil {
		return requirement
	}

	compiled, compileErr := engine.Compile(requirement.Expr.Source, schemaexpressions.CompileModeScalar)
	if compileErr != nil {
		shared.AddError(
			issues,
			normalizedExpressionCode(compileErr, "schema.requirement.invalid_expression"),
			fmt.Sprintf("invalid required expression: %s", compileErr.Message),
			requirement.Path,
		)
		requirement.Expr = nil
		return requirement
	}

	requirement.Expr = compiled
	return requirement
}

func compileValueInterpolations(spec model.ValueSpec, engine *schemaexpressions.Engine, path string, issues *[]diagnostics.Issue) model.ValueSpec {
	if engine == nil {
		if spec.Items != nil {
			compiledItems := compileValueInterpolations(*spec.Items, engine, path+".items", issues)
			spec.Items = &compiledItems
		}
		return spec
	}

	if spec.Const != nil {
		compiledConst := compileLiteralInterpolation(*spec.Const, spec.Kind, path+".const", engine, issues)
		spec.Const = &compiledConst
	}
	for idx := range spec.Enum {
		spec.Enum[idx] = compileLiteralInterpolation(spec.Enum[idx], spec.Kind, fmt.Sprintf("%s.enum[%d]", path, idx), engine, issues)
	}
	if spec.Items != nil {
		compiledItems := compileValueInterpolations(*spec.Items, engine, path+".items", issues)
		spec.Items = &compiledItems
	}
	return spec
}

func compileLiteralInterpolation(
	literal model.Literal,
	kind model.ValueKind,
	path string,
	engine *schemaexpressions.Engine,
	issues *[]diagnostics.Issue,
) model.Literal {
	stringValue, isString := literal.Value.(string)
	if !isString || !strings.Contains(stringValue, "${") {
		return literal
	}
	if kind != model.ValueKindString {
		shared.AddError(issues, "schema.value.interpolation_type_invalid", "interpolation is allowed only for schema.type=string", path)
		return literal
	}

	template, compileErr := schemaexpressions.CompileTemplate(stringValue, engine)
	if compileErr != nil {
		shared.AddError(
			issues,
			normalizedExpressionCode(compileErr, "schema.interpolation.invalid"),
			fmt.Sprintf("invalid interpolation: %s", compileErr.Message),
			path,
		)
		return literal
	}

	literal.Template = template
	return literal
}

func parsePathTemplate(
	node *yaml.Node,
	path string,
	engine *schemaexpressions.Engine,
	fieldsByName map[string]model.MetaField,
	issues *[]diagnostics.Issue,
) model.PathTemplate {
	cases := make([]model.PathTemplateCase, 0)

	switch node.Kind {
	case yaml.ScalarNode:
		if use, isValid := shared.ScalarString(node, path, true, issues); isValid {
			pathCase := parsePathCaseFromValues(map[string]*yaml.Node{"use": node}, path, use, engine, issues)
			if pathCase != nil {
				cases = append(cases, *pathCase)
			}
		}
	case yaml.SequenceNode:
		for index, caseNode := range node.Content {
			casePath := fmt.Sprintf("%s[%d]", path, index)
			pathCase := parsePathCase(caseNode, casePath, engine, issues)
			if pathCase != nil {
				cases = append(cases, *pathCase)
			}
		}
	case yaml.MappingNode:
		values, ok := shared.MappingValues(node, path, issues)
		if !ok {
			return model.PathTemplate{Cases: cases}
		}
		shared.AppendUnsupportedKeys(values, path, shared.SetOf("cases"), issues)

		casesNode, exists := values["cases"]
		if !exists {
			shared.AddError(issues, "schema.path_template.cases_required", "pathTemplate.cases is required", path+".cases")
			return model.PathTemplate{Cases: cases}
		}
		if casesNode.Kind != yaml.SequenceNode {
			shared.AddError(issues, "schema.path_template.cases_invalid", "pathTemplate.cases must be an array", path+".cases")
			return model.PathTemplate{Cases: cases}
		}
		for index, caseNode := range casesNode.Content {
			casePath := fmt.Sprintf("%s.cases[%d]", path, index)
			pathCase := parsePathCase(caseNode, casePath, engine, issues)
			if pathCase != nil {
				cases = append(cases, *pathCase)
			}
		}
	default:
		shared.AddError(
			issues,
			"schema.path_template.invalid",
			"pathTemplate must be string, array of cases, or object with cases",
			path,
		)
	}

	if len(cases) == 0 {
		shared.AddError(issues, "schema.path_template.empty", "pathTemplate must define at least one case", path)
		return model.PathTemplate{Cases: cases}
	}

	for idx := range cases {
		validatePathCaseGuardSafety(cases[idx], fieldsByName, issues)
	}

	unconditionalIndexes := unconditionalCaseIndexes(cases)
	switch len(unconditionalIndexes) {
	case 1:
		if unconditionalIndexes[0] != len(cases)-1 {
			shared.AddError(
				issues,
				"schema.path_template.unconditional_case_position",
				"pathTemplate unconditional case must be the last case",
				fmt.Sprintf("%s.cases[%d]", path, unconditionalIndexes[0]),
			)
		}
	default:
		shared.AddError(
			issues,
			"schema.path_template.unconditional_case_count",
			"pathTemplate must contain exactly one unconditional case",
			path+".cases",
		)
	}

	return model.PathTemplate{Cases: cases}
}

func parsePathCase(node *yaml.Node, path string, engine *schemaexpressions.Engine, issues *[]diagnostics.Issue) *model.PathTemplateCase {
	values, ok := shared.MappingValues(node, path, issues)
	if !ok {
		return nil
	}
	shared.AppendUnsupportedKeys(values, path, shared.SetOf("use", "when"), issues)

	useNode, hasUse := values["use"]
	if !hasUse {
		shared.AddError(issues, "schema.path_template.use_required", "pathTemplate case requires 'use'", path+".use")
		return nil
	}
	use, useOK := shared.ScalarString(useNode, path+".use", true, issues)
	if !useOK {
		return nil
	}

	return parsePathCaseFromValues(values, path, use, engine, issues)
}

func parsePathCaseFromValues(
	values map[string]*yaml.Node,
	path string,
	use string,
	engine *schemaexpressions.Engine,
	issues *[]diagnostics.Issue,
) *model.PathTemplateCase {
	when := shared.ParseRequirement(values["when"], path+".when", true, issues)
	when = compileRequirement(when, engine, issues)

	useTemplate := compileUseTemplate(use, path+".use", engine, issues)
	return &model.PathTemplateCase{
		Use:         use,
		UseTemplate: useTemplate,
		When:        when,
		UsePath:     path + ".use",
	}
}

func compileUseTemplate(
	use string,
	path string,
	engine *schemaexpressions.Engine,
	issues *[]diagnostics.Issue,
) *schemaexpressions.CompiledTemplate {
	if engine == nil {
		return shared.CompileTemplate(use, path, issues)
	}

	template, compileErr := schemaexpressions.CompileTemplate(use, engine)
	if compileErr != nil {
		shared.AddError(
			issues,
			normalizedExpressionCode(compileErr, "schema.path_template.use_invalid"),
			fmt.Sprintf("invalid pathTemplate.use interpolation: %s", compileErr.Message),
			path,
		)
		return nil
	}
	return template
}

func validatePathCaseGuardSafety(pathCase model.PathTemplateCase, fieldsByName map[string]model.MetaField, issues *[]diagnostics.Issue) {
	if pathCase.UseTemplate == nil {
		return
	}
	if pathCase.When.Expr == nil && !pathCase.When.Always {
		return
	}

	for _, part := range pathCase.UseTemplate.Parts {
		if part.Expression == nil {
			continue
		}
		for _, root := range requiredGuardRoots(part.Expression, fieldsByName) {
			if expressioncontext.IsPathGuaranteedBySchema(root, fieldsByName) {
				continue
			}
			if pathCase.When.Expr != nil && pathCase.When.Expr.ProtectsWhenTrue(root) {
				continue
			}
			guardLabel := pathCase.When.Path
			if pathCase.When.Expr == nil && pathCase.When.Always {
				guardLabel = "missing when guard"
			}
			shared.AddError(
				issues,
				"schema.path_template.use_missing_guard",
				fmt.Sprintf("interpolation in pathTemplate.use is not protected by %s for path '%s'", guardLabel, root),
				pathCase.UsePath,
			)
		}
	}
}

func requiredGuardRoots(expression *schemaexpressions.CompiledExpression, fieldsByName map[string]model.MetaField) []string {
	if expression == nil {
		return nil
	}
	paths := expression.GuardedPathsWhenTrue()
	if len(paths) == 0 {
		return nil
	}

	roots := map[string]struct{}{}
	for _, path := range paths {
		root, requiresGuard := expressioncontext.GuardRootForPath(path, fieldsByName)
		if !requiresGuard {
			continue
		}
		roots[root] = struct{}{}
	}
	if len(roots) == 0 {
		return nil
	}

	result := make([]string, 0, len(roots))
	for root := range roots {
		result = append(result, root)
	}
	sort.Strings(result)
	return result
}

func unconditionalCaseIndexes(cases []model.PathTemplateCase) []int {
	indexes := make([]int, 0)
	for idx, pathCase := range cases {
		if pathCase.When.Expr == nil && pathCase.When.Always {
			indexes = append(indexes, idx)
		}
	}
	return indexes
}

func sortedMetaFieldNames(fields map[string]model.MetaField) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedSectionNames(sections map[string]model.Section) []string {
	names := make([]string, 0, len(sections))
	for name := range sections {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func normalizedExpressionCode(compileErr *schemaexpressions.CompileError, fallback string) string {
	if compileErr == nil {
		return fallback
	}
	if code := compileErr.AsStaticCode(); strings.TrimSpace(code) != "" {
		return code
	}
	if code := strings.TrimSpace(compileErr.Code); code != "" {
		return code
	}
	return fallback
}
