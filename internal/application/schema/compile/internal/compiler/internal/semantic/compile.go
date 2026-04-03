package semantic

import (
	"fmt"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/compile/internal/compiler/internal/shared"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/diagnostics"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/source"
	"gopkg.in/yaml.v3"
)

var builtinMetaFieldNames = map[string]struct{}{
	"type":        {},
	"id":          {},
	"slug":        {},
	"createdDate": {},
	"updatedDate": {},
}

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

	pathTemplateNode, hasPathTemplate := values["pathTemplate"]
	if !hasPathTemplate {
		shared.AddError(issues, "schema.entity.path_template_required", "pathTemplate is required", path+".pathTemplate")
	} else {
		entity.PathTemplate = parsePathTemplate(pathTemplateNode, path+".pathTemplate", issues)
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
	shared.AppendUnsupportedKeys(values, path, shared.SetOf("fields", "description"), issues)

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
		if strings.TrimSpace(fieldName) == "" {
			shared.AddError(issues, "schema.meta_field.name_invalid", "meta field name must be non-empty", path+".fields")
			continue
		}
		if shared.HasWhitespace(fieldName) {
			shared.AddError(
				issues,
				"schema.meta_field.name_invalid",
				fmt.Sprintf("meta field name '%s' must not contain whitespace", fieldName),
				path+".fields."+fieldName,
			)
			continue
		}
		if _, reserved := builtinMetaFieldNames[fieldName]; reserved {
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
		if parsed, isValid := shared.ScalarString(descriptionNode, path+".description", false, issues); isValid {
			description = parsed
		}
	}

	return model.MetaField{
		Name:        fieldName,
		Value:       valueSpec,
		Required:    required,
		Description: description,
	}, true
}

func parseSections(node *yaml.Node, path string, issues *[]diagnostics.Issue) map[string]model.Section {
	values, ok := shared.MappingValues(node, path, issues)
	if !ok {
		return map[string]model.Section{}
	}
	shared.AppendUnsupportedKeys(values, path, shared.SetOf("sections", "description"), issues)

	sectionsNode, exists := values["sections"]
	if !exists {
		return map[string]model.Section{}
	}
	sectionValues, sectionsOK := shared.MappingValues(sectionsNode, path+".sections", issues)
	if !sectionsOK {
		return map[string]model.Section{}
	}

	result := make(map[string]model.Section, len(sectionValues))
	for _, sectionName := range shared.SortedKeys(sectionValues) {
		if strings.TrimSpace(sectionName) == "" {
			shared.AddError(issues, "schema.section.name_invalid", "section name must be non-empty", path+".sections")
			continue
		}
		if shared.HasWhitespace(sectionName) {
			shared.AddError(
				issues,
				"schema.section.name_invalid",
				fmt.Sprintf("section name '%s' must not contain whitespace", sectionName),
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
	if titleNode, exists := values["title"]; exists {
		parsedTitles, isValid := shared.ParseTitles(titleNode, path+".title", issues)
		if isValid {
			titles = parsedTitles
		}
	}

	required := shared.ParseRequirement(values["required"], path+".required", true, issues)
	description := ""
	if descriptionNode, exists := values["description"]; exists {
		if parsed, isValid := shared.ScalarString(descriptionNode, path+".description", false, issues); isValid {
			description = parsed
		}
	}

	return model.Section{
		Name:        sectionName,
		Titles:      titles,
		Required:    required,
		Description: description,
	}, true
}

func parsePathTemplate(node *yaml.Node, path string, issues *[]diagnostics.Issue) model.PathTemplate {
	cases := make([]model.PathTemplateCase, 0)

	switch node.Kind {
	case yaml.ScalarNode:
		if use, isValid := shared.ScalarString(node, path, true, issues); isValid {
			cases = append(cases, model.PathTemplateCase{
				Use:         use,
				UseTemplate: shared.CompileTemplate(use, path, issues),
				When:        model.Requirement{Always: true},
			})
		}
	case yaml.SequenceNode:
		for index, caseNode := range node.Content {
			casePath := fmt.Sprintf("%s[%d]", path, index)
			parsedCase, ok := parsePathCase(caseNode, casePath, issues)
			if ok {
				cases = append(cases, parsedCase)
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
			parsedCase, caseOK := parsePathCase(caseNode, casePath, issues)
			if caseOK {
				cases = append(cases, parsedCase)
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

	hasUnconditional := false
	for _, pathCase := range cases {
		if pathCase.When.Expr == nil && pathCase.When.Always {
			hasUnconditional = true
			break
		}
	}
	if !hasUnconditional {
		shared.AddError(
			issues,
			"schema.path_template.no_unconditional_case",
			"pathTemplate must contain at least one unconditional case",
			path,
		)
	}

	return model.PathTemplate{Cases: cases}
}

func parsePathCase(node *yaml.Node, path string, issues *[]diagnostics.Issue) (model.PathTemplateCase, bool) {
	values, ok := shared.MappingValues(node, path, issues)
	if !ok {
		return model.PathTemplateCase{}, false
	}
	shared.AppendUnsupportedKeys(values, path, shared.SetOf("use", "when"), issues)

	useNode, hasUse := values["use"]
	if !hasUse {
		shared.AddError(issues, "schema.path_template.use_required", "pathTemplate case requires 'use'", path+".use")
		return model.PathTemplateCase{}, false
	}
	use, useOK := shared.ScalarString(useNode, path+".use", true, issues)
	if !useOK {
		return model.PathTemplateCase{}, false
	}

	when := shared.ParseRequirement(values["when"], path+".when", true, issues)
	return model.PathTemplateCase{
		Use:         use,
		UseTemplate: shared.CompileTemplate(use, path+".use", issues),
		When:        when,
	}, true
}
