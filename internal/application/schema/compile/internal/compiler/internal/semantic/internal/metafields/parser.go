package metafields

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/compile/internal/compiler/internal/shared"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/diagnostics"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/expressioncontext"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
	"gopkg.in/yaml.v3"
)

var schemaKeyNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

func Parse(
	node *yaml.Node,
	path string,
	typeSet map[string]struct{},
	issues *[]diagnostics.Issue,
) (map[string]model.MetaField, []string) {
	values, ok := shared.MappingValues(node, path, issues)
	if !ok {
		return map[string]model.MetaField{}, []string{}
	}
	shared.AppendUnsupportedKeys(values, path, shared.SetOf("fields"), issues)

	fieldsNode, exists := values["fields"]
	if !exists {
		return map[string]model.MetaField{}, []string{}
	}
	fieldValues, fieldsOK := shared.MappingValues(fieldsNode, path+".fields", issues)
	if !fieldsOK {
		return map[string]model.MetaField{}, []string{}
	}

	result := make(map[string]model.MetaField, len(fieldValues))
	order := shared.OrderedKeys(fieldValues, fieldsNode)
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

	filteredOrder := make([]string, 0, len(order))
	for _, fieldName := range order {
		if _, exists := result[fieldName]; !exists {
			continue
		}
		filteredOrder = append(filteredOrder, fieldName)
	}

	return result, filteredOrder
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
