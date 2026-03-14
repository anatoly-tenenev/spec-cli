package entity

import (
	"fmt"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	"gopkg.in/yaml.v3"
)

const schemaBlockingStandardRef = "7"

func ParseType(
	typeName string,
	rawType map[string]any,
	typeNode *yaml.Node,
	usedPrefixes map[string]string,
) (model.EntityTypeSpec, *domainerrors.AppError) {
	idPrefix, idPrefixErr := parseIDPrefix(typeName, rawType["id_prefix"], usedPrefixes)
	if idPrefixErr != nil {
		return model.EntityTypeSpec{}, idPrefixErr
	}

	pathPattern, pathPatternErr := parsePathPattern(typeName, rawType["path_pattern"])
	if pathPatternErr != nil {
		return model.EntityTypeSpec{}, pathPatternErr
	}

	metaFields, metaOrder, metaErr := parseMetaFields(typeName, rawType["meta"], mappingValueNode(typeNode, "meta"))
	if metaErr != nil {
		return model.EntityTypeSpec{}, metaErr
	}

	sections, sectionOrder, hasContent, sectionErr := parseSections(typeName, rawType["content"], mappingValueNode(typeNode, "content"))
	if sectionErr != nil {
		return model.EntityTypeSpec{}, sectionErr
	}

	allowWritePaths := map[string]model.WritePathSpec{}
	allowSetFilePaths := map[string]struct{}{}
	for _, fieldName := range metaOrder {
		if metaFields[fieldName].IsEntityRef {
			allowWritePaths["refs."+fieldName] = model.WritePathSpec{Kind: model.WritePathRef, FieldName: fieldName}
			continue
		}
		allowWritePaths["meta."+fieldName] = model.WritePathSpec{Kind: model.WritePathMeta, FieldName: fieldName}
	}
	for _, sectionName := range sectionOrder {
		path := "content.sections." + sectionName
		allowWritePaths[path] = model.WritePathSpec{Kind: model.WritePathSection, FieldName: sectionName}
		allowSetFilePaths[path] = struct{}{}
	}

	return model.EntityTypeSpec{
		Name:              typeName,
		IDPrefix:          idPrefix,
		PathPattern:       pathPattern,
		MetaFields:        metaFields,
		MetaFieldOrder:    metaOrder,
		Sections:          sections,
		SectionOrder:      sectionOrder,
		HasContent:        hasContent,
		AllowWritePaths:   allowWritePaths,
		AllowSetFilePaths: allowSetFilePaths,
	}, nil
}

func parseIDPrefix(typeName string, rawIDPrefix any, usedPrefixes map[string]string) (string, *domainerrors.AppError) {
	idPrefix, ok := rawIDPrefix.(string)
	if !ok || strings.TrimSpace(idPrefix) == "" {
		return "", newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.id_prefix must be a non-empty string", typeName),
			nil,
		)
	}

	idPrefix = strings.TrimSpace(idPrefix)
	if existingType, exists := usedPrefixes[idPrefix]; exists {
		return "", newSchemaError(
			domainerrors.CodeSchemaInvalid,
			"schema contains duplicated id_prefix across entity types",
			map[string]any{"id_prefix": idPrefix, "types": []string{existingType, typeName}},
		)
	}
	usedPrefixes[idPrefix] = typeName
	return idPrefix, nil
}

func parsePathPattern(typeName string, rawPathPattern any) (model.PathPattern, *domainerrors.AppError) {
	if rawPathPattern == nil {
		return model.PathPattern{}, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.path_pattern is required", typeName),
			nil,
		)
	}

	switch typed := rawPathPattern.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return model.PathPattern{}, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.path_pattern must be non-empty", typeName),
				nil,
			)
		}
		return model.PathPattern{Cases: []model.PathPatternCase{{Use: typed}}}, nil
	case []any:
		if len(typed) == 0 {
			return model.PathPattern{}, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.path_pattern array must be non-empty", typeName),
				nil,
			)
		}
		cases := make([]model.PathPatternCase, 0, len(typed))
		for idx, rawCase := range typed {
			useValue, ok := rawCase.(string)
			if !ok || strings.TrimSpace(useValue) == "" {
				return model.PathPattern{}, newSchemaError(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("schema.entity.%s.path_pattern[%d] must be a non-empty string", typeName, idx),
					nil,
				)
			}
			cases = append(cases, model.PathPatternCase{Use: useValue})
		}
		return model.PathPattern{Cases: cases}, nil
	case map[string]any:
		rawCases, ok := support.ToSlice(typed["cases"])
		if !ok || len(rawCases) == 0 {
			return model.PathPattern{}, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.path_pattern.cases must be a non-empty array", typeName),
				nil,
			)
		}

		cases := make([]model.PathPatternCase, 0, len(rawCases))
		hasFallback := false
		for idx, rawCase := range rawCases {
			caseMap, ok := support.ToStringMap(rawCase)
			if !ok {
				return model.PathPattern{}, newSchemaError(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("schema.entity.%s.path_pattern.cases[%d] must be a mapping", typeName, idx),
					nil,
				)
			}
			useValue, ok := caseMap["use"].(string)
			if !ok || strings.TrimSpace(useValue) == "" {
				return model.PathPattern{}, newSchemaError(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("schema.entity.%s.path_pattern.cases[%d].use must be non-empty string", typeName, idx),
					nil,
				)
			}

			pathCase := model.PathPatternCase{Use: useValue}
			if whenValue, exists := caseMap["when"]; exists {
				pathCase.HasWhen = true
				pathCase.When = whenValue
			} else {
				hasFallback = true
			}
			cases = append(cases, pathCase)
		}

		if !hasFallback {
			return model.PathPattern{}, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.path_pattern must include unconditional fallback case", typeName),
				nil,
			)
		}

		return model.PathPattern{Cases: cases}, nil
	default:
		return model.PathPattern{}, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.path_pattern has unsupported format", typeName),
			nil,
		)
	}
}

func parseMetaFields(typeName string, rawMeta any, metaNode *yaml.Node) (map[string]model.MetaField, []string, *domainerrors.AppError) {
	fields := map[string]model.MetaField{}
	order := []string{}

	if rawMeta == nil {
		return fields, order, nil
	}

	metaMap, ok := support.ToStringMap(rawMeta)
	if !ok {
		return nil, nil, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta must be a mapping", typeName),
			nil,
		)
	}

	rawFields, hasFields := metaMap["fields"]
	if !hasFields {
		return fields, order, nil
	}

	fieldsMap, ok := support.ToStringMap(rawFields)
	if !ok {
		return nil, nil, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields must be a mapping", typeName),
			nil,
		)
	}

	fieldsNode := mappingValueNode(metaNode, "fields")
	fieldNames := orderedKeys(fieldsMap, fieldsNode)
	for _, fieldName := range fieldNames {
		rawField, ok := support.ToStringMap(fieldsMap[fieldName])
		if !ok {
			return nil, nil, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.fields.%s must be a mapping", typeName, fieldName),
				nil,
			)
		}

		parsed, parseErr := parseMetaField(typeName, fieldName, rawField)
		if parseErr != nil {
			return nil, nil, parseErr
		}
		fields[fieldName] = parsed
		order = append(order, fieldName)
	}

	return fields, order, nil
}

func parseMetaField(typeName string, fieldName string, rawField map[string]any) (model.MetaField, *domainerrors.AppError) {
	required := true
	if rawRequired, exists := rawField["required"]; exists {
		typed, ok := rawRequired.(bool)
		if !ok {
			return model.MetaField{}, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.fields.%s.required must be boolean", typeName, fieldName),
				nil,
			)
		}
		required = typed
	}

	rawSchema, ok := rawField["schema"]
	if !ok {
		return model.MetaField{}, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema is required", typeName, fieldName),
			nil,
		)
	}

	schemaMap, ok := support.ToStringMap(rawSchema)
	if !ok {
		return model.MetaField{}, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema must be a mapping", typeName, fieldName),
			nil,
		)
	}

	typeValue, ok := schemaMap["type"].(string)
	if !ok || strings.TrimSpace(typeValue) == "" {
		return model.MetaField{}, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.type must be a non-empty string", typeName, fieldName),
			nil,
		)
	}
	typeValue = strings.TrimSpace(typeValue)

	field := model.MetaField{
		Name:     fieldName,
		Type:     typeValue,
		Required: required,
	}

	if requiredWhen, exists := rawField["required_when"]; exists {
		field.HasRequiredWhen = true
		field.RequiredWhen = requiredWhen
		if _, isBool := requiredWhen.(bool); !isBool {
			if _, isMap := support.ToStringMap(requiredWhen); !isMap {
				return model.MetaField{}, newSchemaError(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("schema.entity.%s.meta.fields.%s.required_when must be boolean or expression object", typeName, fieldName),
					nil,
				)
			}
		}
	}

	if typeValue == "entity_ref" {
		field.IsEntityRef = true
		if rawRefTypes, exists := schemaMap["refTypes"]; exists {
			refTypes, ok := support.ToSlice(rawRefTypes)
			if !ok {
				return model.MetaField{}, newSchemaError(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.refTypes must be array", typeName, fieldName),
					nil,
				)
			}
			for _, item := range refTypes {
				refType, ok := item.(string)
				if !ok || strings.TrimSpace(refType) == "" {
					return model.MetaField{}, newSchemaError(
						domainerrors.CodeSchemaInvalid,
						fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.refTypes must contain non-empty strings", typeName, fieldName),
						nil,
					)
				}
				field.RefTypes = append(field.RefTypes, strings.TrimSpace(refType))
			}
		}
	}

	switch typeValue {
	case "string", "integer", "number", "boolean", "null", "array", "entity_ref":
	default:
		return model.MetaField{}, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.type uses unsupported type", typeName, fieldName),
			map[string]any{"type": typeValue},
		)
	}

	if format, ok := schemaMap["format"].(string); ok {
		field.Format = strings.TrimSpace(format)
	}

	if rawEnum, exists := schemaMap["enum"]; exists {
		enumValues, ok := support.ToSlice(rawEnum)
		if !ok || len(enumValues) == 0 {
			return model.MetaField{}, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.enum must be non-empty array", typeName, fieldName),
				nil,
			)
		}
		field.Enum = append([]any(nil), enumValues...)
	}

	if rawConst, exists := schemaMap["const"]; exists {
		field.HasConst = true
		field.Const = rawConst
	}

	if typeValue == "array" {
		if rawItems, exists := schemaMap["items"]; exists {
			itemsMap, ok := support.ToStringMap(rawItems)
			if !ok {
				return model.MetaField{}, newSchemaError(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.items must be mapping", typeName, fieldName),
					nil,
				)
			}
			itemType, ok := itemsMap["type"].(string)
			if !ok || strings.TrimSpace(itemType) == "" {
				return model.MetaField{}, newSchemaError(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.items.type must be non-empty string", typeName, fieldName),
					nil,
				)
			}
			field.HasItems = true
			field.ItemType = strings.TrimSpace(itemType)
		}
		if uniqueItems, exists := schemaMap["uniqueItems"]; exists {
			typed, ok := uniqueItems.(bool)
			if !ok {
				return model.MetaField{}, newSchemaError(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.uniqueItems must be boolean", typeName, fieldName),
					nil,
				)
			}
			field.UniqueItems = typed
		}
		if minItems, exists := schemaMap["minItems"]; exists {
			number, ok := support.NumberToFloat64(minItems)
			if !ok || number < 0 || number != float64(int(number)) {
				return model.MetaField{}, newSchemaError(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.minItems must be integer >= 0", typeName, fieldName),
					nil,
				)
			}
			field.HasMinItems = true
			field.MinItems = int(number)
		}
		if maxItems, exists := schemaMap["maxItems"]; exists {
			number, ok := support.NumberToFloat64(maxItems)
			if !ok || number < 0 || number != float64(int(number)) {
				return model.MetaField{}, newSchemaError(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("schema.entity.%s.meta.fields.%s.schema.maxItems must be integer >= 0", typeName, fieldName),
					nil,
				)
			}
			field.HasMaxItems = true
			field.MaxItems = int(number)
		}
	}

	return field, nil
}

func parseSections(typeName string, rawContent any, contentNode *yaml.Node) (map[string]model.SectionSpec, []string, bool, *domainerrors.AppError) {
	sections := map[string]model.SectionSpec{}
	order := []string{}

	if rawContent == nil {
		return sections, order, false, nil
	}

	contentMap, ok := support.ToStringMap(rawContent)
	if !ok {
		return nil, nil, false, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.content must be a mapping", typeName),
			nil,
		)
	}

	rawSections, hasSections := contentMap["sections"]
	if !hasSections {
		return sections, order, true, nil
	}

	sectionsMap, ok := support.ToStringMap(rawSections)
	if !ok {
		return nil, nil, false, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.content.sections must be a mapping", typeName),
			nil,
		)
	}

	sectionsNode := mappingValueNode(contentNode, "sections")
	sectionNames := orderedKeys(sectionsMap, sectionsNode)
	for _, sectionName := range sectionNames {
		rawSection, ok := support.ToStringMap(sectionsMap[sectionName])
		if !ok {
			return nil, nil, false, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.content.sections.%s must be a mapping", typeName, sectionName),
				nil,
			)
		}

		required := true
		if rawRequired, exists := rawSection["required"]; exists {
			typed, ok := rawRequired.(bool)
			if !ok {
				return nil, nil, false, newSchemaError(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("schema.entity.%s.content.sections.%s.required must be boolean", typeName, sectionName),
					nil,
				)
			}
			required = typed
		}

		section := model.SectionSpec{Name: sectionName, Required: required}
		if rawRequiredWhen, exists := rawSection["required_when"]; exists {
			section.HasRequiredWhen = true
			section.RequiredWhen = rawRequiredWhen
			if _, isBool := rawRequiredWhen.(bool); !isBool {
				if _, isMap := support.ToStringMap(rawRequiredWhen); !isMap {
					return nil, nil, false, newSchemaError(
						domainerrors.CodeSchemaInvalid,
						fmt.Sprintf("schema.entity.%s.content.sections.%s.required_when must be boolean or expression object", typeName, sectionName),
						nil,
					)
				}
			}
		}

		if rawTitle, exists := rawSection["title"]; exists {
			titles, titleErr := parseSectionTitles(typeName, sectionName, rawTitle)
			if titleErr != nil {
				return nil, nil, false, titleErr
			}
			section.Titles = titles
		}

		sections[sectionName] = section
		order = append(order, sectionName)
	}

	return sections, order, true, nil
}

func parseSectionTitles(typeName string, sectionName string, rawTitle any) ([]string, *domainerrors.AppError) {
	switch typed := rawTitle.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.content.sections.%s.title must be non-empty", typeName, sectionName),
				nil,
			)
		}
		return []string{strings.TrimSpace(typed)}, nil
	case []any:
		if len(typed) == 0 {
			return nil, newSchemaError(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("schema.entity.%s.content.sections.%s.title must be non-empty", typeName, sectionName),
				nil,
			)
		}
		titles := make([]string, 0, len(typed))
		for _, item := range typed {
			title, ok := item.(string)
			if !ok || strings.TrimSpace(title) == "" {
				return nil, newSchemaError(
					domainerrors.CodeSchemaInvalid,
					fmt.Sprintf("schema.entity.%s.content.sections.%s.title must contain non-empty strings", typeName, sectionName),
					nil,
				)
			}
			titles = append(titles, strings.TrimSpace(title))
		}
		return titles, nil
	default:
		return nil, newSchemaError(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.content.sections.%s.title must be string or string[]", typeName, sectionName),
			nil,
		)
	}
}

func orderedKeys(values map[string]any, mappingNode *yaml.Node) []string {
	if mappingNode == nil || mappingNode.Kind != yaml.MappingNode {
		return support.SortedMapKeys(values)
	}
	keys := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for idx := 0; idx+1 < len(mappingNode.Content); idx += 2 {
		key := mappingNode.Content[idx].Value
		if _, exists := values[key]; !exists {
			continue
		}
		keys = append(keys, key)
		seen[key] = struct{}{}
	}
	if len(seen) == len(values) {
		return keys
	}
	for _, key := range support.SortedMapKeys(values) {
		if _, exists := seen[key]; exists {
			continue
		}
		keys = append(keys, key)
	}
	return keys
}

func mappingValueNode(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for idx := 0; idx+1 < len(node.Content); idx += 2 {
		if node.Content[idx].Value == key {
			return node.Content[idx+1]
		}
	}
	return nil
}

func newSchemaError(code domainerrors.Code, message string, details map[string]any) *domainerrors.AppError {
	issue := support.ValidationIssue("schema.invalid", message, schemaBlockingStandardRef, "")
	return domainerrors.New(code, message, support.WithValidationIssues(details, issue))
}
