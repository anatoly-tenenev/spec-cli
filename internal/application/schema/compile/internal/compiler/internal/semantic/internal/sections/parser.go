package sections

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/compile/internal/compiler/internal/shared"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/diagnostics"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
	"gopkg.in/yaml.v3"
)

var schemaKeyNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

func Parse(node *yaml.Node, path string, issues *[]diagnostics.Issue) (map[string]model.Section, []string) {
	values, ok := shared.MappingValues(node, path, issues)
	if !ok {
		return map[string]model.Section{}, []string{}
	}
	shared.AppendUnsupportedKeys(values, path, shared.SetOf("sections"), issues)

	sectionsNode, exists := values["sections"]
	if !exists {
		return map[string]model.Section{}, []string{}
	}
	sectionValues, sectionsOK := shared.MappingValues(sectionsNode, path+".sections", issues)
	if !sectionsOK {
		return map[string]model.Section{}, []string{}
	}

	if len(sectionValues) == 0 {
		shared.AddError(issues, "schema.section.empty", "content.sections must be a non-empty mapping", path+".sections")
		return map[string]model.Section{}, []string{}
	}

	result := make(map[string]model.Section, len(sectionValues))
	order := shared.OrderedKeys(sectionValues, sectionsNode)
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

	filteredOrder := make([]string, 0, len(order))
	for _, sectionName := range order {
		if _, exists := result[sectionName]; !exists {
			continue
		}
		filteredOrder = append(filteredOrder, sectionName)
	}

	return result, filteredOrder
}

func parseSection(sectionName string, node *yaml.Node, path string, issues *[]diagnostics.Issue) (model.Section, bool) {
	values, ok := shared.MappingValues(node, path, issues)
	if !ok {
		return model.Section{}, false
	}
	shared.AppendUnsupportedKeys(values, path, shared.SetOf("title", "required", "description"), issues)

	title := ""
	titlePath := path + ".title"
	if titleNode, exists := values["title"]; exists {
		parsedTitle, isValid := shared.ScalarString(titleNode, titlePath, true, issues)
		if isValid {
			if strings.Contains(parsedTitle, "${") {
				shared.AddError(issues, "schema.section.title_interpolation_forbidden", "title must not contain interpolation", titlePath)
			} else {
				title = parsedTitle
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
		Title:       title,
		Required:    required,
		Description: description,
		SchemaPath:  path,
		TitlePath:   titlePath,
	}, true
}
