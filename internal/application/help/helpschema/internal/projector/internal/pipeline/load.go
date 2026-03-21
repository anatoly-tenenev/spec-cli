package pipeline

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/help/helpschema/internal/projector/internal/pipeline/internal/yamlnodes"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	"gopkg.in/yaml.v3"
)

type Projection struct {
	EffectivePath  string
	ProjectionYAML string
}

type mappingEntry = yamlnodes.MappingEntry

var (
	readMapping       = yamlnodes.ReadMapping
	ensureAllowedKeys = yamlnodes.EnsureAllowedKeys
	requiredScalar    = yamlnodes.RequiredScalar
	parseBoolScalar   = yamlnodes.ParseBoolScalar
	scalarSequence    = yamlnodes.ScalarSequence
	mappingNode       = yamlnodes.MappingNode
	sequenceNode      = yamlnodes.SequenceNode
	boolScalar        = yamlnodes.BoolScalar
	cloneScalar       = yamlnodes.CloneScalar
	appendMapping     = yamlnodes.AppendMapping
	projectionError   = yamlnodes.ProjectionError
)

func LoadProjection(schemaPath string, displayPath string) (Projection, *domainerrors.AppError) {
	raw, readErr := os.ReadFile(schemaPath)
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			return Projection{}, domainerrors.New(
				domainerrors.CodeSchemaNotFound,
				"schema file does not exist",
				map[string]any{"path": displayPath},
			)
		}
		return Projection{}, domainerrors.New(
			domainerrors.CodeSchemaReadError,
			"schema file is not readable",
			map[string]any{"path": displayPath},
		)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return Projection{}, domainerrors.New(
			domainerrors.CodeSchemaParseError,
			"failed to parse schema yaml/json",
			map[string]any{"path": displayPath},
		)
	}

	rootNode, rootErr := rootMappingNode(&root)
	if rootErr != nil {
		return Projection{}, projectionError(rootErr)
	}

	projectedRoot, projectionErr := projectRoot(rootNode)
	if projectionErr != nil {
		return Projection{}, projectionError(projectionErr)
	}

	rendered, renderErr := renderProjection(projectedRoot)
	if renderErr != nil {
		return Projection{}, projectionError(renderErr)
	}

	return Projection{
		EffectivePath:  displayPath,
		ProjectionYAML: rendered,
	}, nil
}

func rootMappingNode(root *yaml.Node) (*yaml.Node, error) {
	node := root
	for node != nil && node.Kind == yaml.DocumentNode {
		if len(node.Content) == 0 {
			return nil, fmt.Errorf("schema document is empty")
		}
		node = node.Content[0]
	}
	if node == nil || node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("schema root must be a mapping object")
	}
	_, _, err := readMapping(node, "schema")
	if err != nil {
		return nil, err
	}
	return node, nil
}

func projectRoot(root *yaml.Node) (*yaml.Node, error) {
	entries, byKey, err := readMapping(root, "schema")
	if err != nil {
		return nil, err
	}
	if err := ensureAllowedKeys(entries, map[string]struct{}{
		"version":     {},
		"description": {},
		"entity":      {},
	}, "schema"); err != nil {
		return nil, err
	}

	versionNode, exists := byKey["version"]
	if !exists {
		return nil, fmt.Errorf("schema.version is required")
	}
	if versionNode.Kind != yaml.ScalarNode {
		return nil, fmt.Errorf("schema.version must be scalar")
	}

	entityNode, exists := byKey["entity"]
	if !exists {
		return nil, fmt.Errorf("schema.entity is required")
	}

	projectedEntity, err := projectEntityMap(entityNode, "schema.entity")
	if err != nil {
		return nil, err
	}

	projected := mappingNode()
	versionValue := cloneScalar(versionNode)
	versionValue.Style = yaml.DoubleQuotedStyle
	appendMapping(projected, "version", versionValue)

	if descriptionNode, hasDescription := byKey["description"]; hasDescription {
		descriptionScalar, descErr := requiredScalar(descriptionNode, "schema.description")
		if descErr != nil {
			return nil, descErr
		}
		appendMapping(projected, "description", descriptionScalar)
	}

	appendMapping(projected, "entity", projectedEntity)

	return projected, nil
}

func projectEntityMap(entityNode *yaml.Node, path string) (*yaml.Node, error) {
	entries, _, err := readMapping(entityNode, path)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("%s must be a non-empty mapping", path)
	}

	projected := mappingNode()
	for _, entry := range entries {
		typePath := path + "." + entry.Key
		projectedType, projectErr := projectEntityType(entry.Value, typePath)
		if projectErr != nil {
			return nil, projectErr
		}
		appendMapping(projected, entry.Key, projectedType)
	}
	return projected, nil
}

func projectEntityType(entityTypeNode *yaml.Node, path string) (*yaml.Node, error) {
	entries, byKey, err := readMapping(entityTypeNode, path)
	if err != nil {
		return nil, err
	}
	if err := ensureAllowedKeys(entries, map[string]struct{}{
		"description":  {},
		"id_prefix":    {},
		"path_pattern": {},
		"meta":         {},
		"content":      {},
	}, path); err != nil {
		return nil, err
	}

	idPrefixNode, exists := byKey["id_prefix"]
	if !exists {
		return nil, fmt.Errorf("%s.id_prefix is required", path)
	}
	idPrefixScalar, err := requiredScalar(idPrefixNode, path+".id_prefix")
	if err != nil {
		return nil, err
	}

	metaFields, refFields, err := projectMetaFields(byKey["meta"], path+".meta")
	if err != nil {
		return nil, err
	}
	contentNode, err := projectContent(byKey["content"], path+".content")
	if err != nil {
		return nil, err
	}

	projected := mappingNode()
	if descriptionNode, hasDescription := byKey["description"]; hasDescription {
		descriptionScalar, descErr := requiredScalar(descriptionNode, path+".description")
		if descErr != nil {
			return nil, descErr
		}
		appendMapping(projected, "description", descriptionScalar)
	}
	appendMapping(projected, "id_prefix", idPrefixScalar)
	if len(metaFields) > 0 {
		metaNode := mappingNode()
		for _, field := range metaFields {
			appendMapping(metaNode, field.Name, field.Node)
		}
		appendMapping(projected, "meta", metaNode)
	}
	if len(refFields) > 0 {
		refsNode := mappingNode()
		for _, field := range refFields {
			appendMapping(refsNode, field.Name, field.Node)
		}
		appendMapping(projected, "refs", refsNode)
	}
	if contentNode != nil {
		appendMapping(projected, "content", contentNode)
	}

	return projected, nil
}

type projectedField struct {
	Name string
	Node *yaml.Node
}

func projectMetaFields(metaNode *yaml.Node, path string) ([]projectedField, []projectedField, error) {
	if metaNode == nil {
		return nil, nil, nil
	}

	entries, byKey, err := readMapping(metaNode, path)
	if err != nil {
		return nil, nil, err
	}
	if err := ensureAllowedKeys(entries, map[string]struct{}{"fields": {}}, path); err != nil {
		return nil, nil, err
	}

	fieldsNode, hasFields := byKey["fields"]
	if !hasFields {
		return nil, nil, nil
	}

	fieldEntries, _, err := readMapping(fieldsNode, path+".fields")
	if err != nil {
		return nil, nil, err
	}

	metaFields := make([]projectedField, 0, len(fieldEntries))
	refFields := make([]projectedField, 0, len(fieldEntries))
	for _, fieldEntry := range fieldEntries {
		fieldPath := path + ".fields." + fieldEntry.Key
		fieldNode, fieldType, fieldErr := projectMetadataField(fieldEntry.Value, fieldPath)
		if fieldErr != nil {
			return nil, nil, fieldErr
		}
		if fieldType == "entity_ref" {
			refFields = append(refFields, projectedField{Name: fieldEntry.Key, Node: fieldNode})
			continue
		}
		metaFields = append(metaFields, projectedField{Name: fieldEntry.Key, Node: fieldNode})
	}

	return metaFields, refFields, nil
}

func projectMetadataField(fieldNode *yaml.Node, path string) (*yaml.Node, string, error) {
	entries, byKey, err := readMapping(fieldNode, path)
	if err != nil {
		return nil, "", err
	}
	if err := ensureAllowedKeys(entries, map[string]struct{}{
		"description":   {},
		"required":      {},
		"required_when": {},
		"schema":        {},
	}, path); err != nil {
		return nil, "", err
	}

	schemaValue, exists := byKey["schema"]
	if !exists {
		return nil, "", fmt.Errorf("%s.schema is required", path)
	}

	requiredNode, err := normalizeRequired(byKey["required"], byKey["required_when"], path)
	if err != nil {
		return nil, "", err
	}

	projectedSchema, fieldType, err := projectSchemaMapping(schemaValue, path+".schema")
	if err != nil {
		return nil, "", err
	}

	projected := mappingNode()
	if descriptionNode, hasDescription := byKey["description"]; hasDescription {
		descriptionScalar, descErr := requiredScalar(descriptionNode, path+".description")
		if descErr != nil {
			return nil, "", descErr
		}
		appendMapping(projected, "description", descriptionScalar)
	}
	appendMapping(projected, "required", requiredNode)
	appendMapping(projected, "schema", projectedSchema)

	return projected, fieldType, nil
}

func projectContent(contentNode *yaml.Node, path string) (*yaml.Node, error) {
	if contentNode == nil {
		return nil, nil
	}

	entries, byKey, err := readMapping(contentNode, path)
	if err != nil {
		return nil, err
	}
	if err := ensureAllowedKeys(entries, map[string]struct{}{"sections": {}}, path); err != nil {
		return nil, err
	}

	sectionsNode, hasSections := byKey["sections"]
	if !hasSections {
		return nil, nil
	}

	sectionEntries, _, err := readMapping(sectionsNode, path+".sections")
	if err != nil {
		return nil, err
	}
	if len(sectionEntries) == 0 {
		return nil, nil
	}

	projectedSections := mappingNode()
	for _, sectionEntry := range sectionEntries {
		sectionPath := path + ".sections." + sectionEntry.Key
		projectedSection, projectErr := projectSection(sectionEntry.Value, sectionPath)
		if projectErr != nil {
			return nil, projectErr
		}
		appendMapping(projectedSections, sectionEntry.Key, projectedSection)
	}

	content := mappingNode()
	appendMapping(content, "sections", projectedSections)
	return content, nil
}

func projectSection(sectionNode *yaml.Node, path string) (*yaml.Node, error) {
	entries, byKey, err := readMapping(sectionNode, path)
	if err != nil {
		return nil, err
	}
	if err := ensureAllowedKeys(entries, map[string]struct{}{
		"description":   {},
		"required":      {},
		"required_when": {},
		"title":         {},
	}, path); err != nil {
		return nil, err
	}

	requiredNode, err := normalizeRequired(byKey["required"], byKey["required_when"], path)
	if err != nil {
		return nil, err
	}

	projected := mappingNode()
	if descriptionNode, hasDescription := byKey["description"]; hasDescription {
		descriptionScalar, descErr := requiredScalar(descriptionNode, path+".description")
		if descErr != nil {
			return nil, descErr
		}
		appendMapping(projected, "description", descriptionScalar)
	}
	appendMapping(projected, "required", requiredNode)

	if titleNode, hasTitle := byKey["title"]; hasTitle {
		projectedTitle, titleErr := normalizeTitle(titleNode, path+".title")
		if titleErr != nil {
			return nil, titleErr
		}
		appendMapping(projected, "title", projectedTitle)
	}

	return projected, nil
}

var canonicalSchemaKeys = []string{
	"type",
	"const",
	"enum",
	"items",
	"minItems",
	"maxItems",
	"uniqueItems",
	"refTypes",
}

func projectSchemaMapping(schemaNode *yaml.Node, path string) (*yaml.Node, string, error) {
	entries, byKey, err := readMapping(schemaNode, path)
	if err != nil {
		return nil, "", err
	}

	allowedKeys := map[string]struct{}{}
	for _, key := range canonicalSchemaKeys {
		allowedKeys[key] = struct{}{}
	}
	if err := ensureAllowedKeys(entries, allowedKeys, path); err != nil {
		return nil, "", err
	}

	typeNode, exists := byKey["type"]
	if !exists {
		return nil, "", fmt.Errorf("%s.type is required", path)
	}
	typeScalar, err := requiredScalar(typeNode, path+".type")
	if err != nil {
		return nil, "", err
	}
	fieldType := strings.TrimSpace(typeScalar.Value)
	if fieldType == "" {
		return nil, "", fmt.Errorf("%s.type must be non-empty", path)
	}

	projected := mappingNode()
	for _, key := range canonicalSchemaKeys {
		rawValue, exists := byKey[key]
		if !exists {
			continue
		}
		projectedValue, projectErr := projectSchemaValue(key, rawValue, path+"."+key)
		if projectErr != nil {
			return nil, "", projectErr
		}
		appendMapping(projected, key, projectedValue)
	}

	return projected, fieldType, nil
}

func projectSchemaValue(key string, valueNode *yaml.Node, path string) (*yaml.Node, error) {
	switch key {
	case "type", "const", "minItems", "maxItems", "uniqueItems":
		return requiredScalar(valueNode, path)
	case "enum", "refTypes":
		return scalarSequence(valueNode, path, false)
	case "items":
		projectedItems, _, err := projectSchemaMapping(valueNode, path)
		if err != nil {
			return nil, err
		}
		return projectedItems, nil
	default:
		return nil, fmt.Errorf("unsupported schema key: %s", key)
	}
}

func normalizeRequired(requiredNode *yaml.Node, requiredWhenNode *yaml.Node, path string) (*yaml.Node, error) {
	if requiredWhenNode != nil {
		whenNode, err := canonicalExpression(requiredWhenNode, path+".required_when")
		if err != nil {
			return nil, err
		}
		whenWrapper := mappingNode()
		appendMapping(whenWrapper, "when", whenNode)
		return whenWrapper, nil
	}

	if requiredNode == nil {
		return boolScalar(true), nil
	}
	parsed, err := parseBoolScalar(requiredNode, path+".required")
	if err != nil {
		return nil, err
	}
	return boolScalar(parsed), nil
}

func normalizeTitle(titleNode *yaml.Node, path string) (*yaml.Node, error) {
	switch titleNode.Kind {
	case yaml.ScalarNode:
		title, err := requiredScalar(titleNode, path)
		if err != nil {
			return nil, err
		}
		out := sequenceNode(false)
		out.Content = append(out.Content, title)
		return out, nil
	case yaml.SequenceNode:
		return scalarSequence(titleNode, path, false)
	default:
		return nil, fmt.Errorf("%s must be string or string[]", path)
	}
}

func canonicalExpression(node *yaml.Node, path string) (*yaml.Node, error) {
	switch node.Kind {
	case yaml.ScalarNode:
		return cloneScalar(node), nil
	case yaml.SequenceNode:
		projected := sequenceNode(true)
		for idx, child := range node.Content {
			projectedChild, err := canonicalExpression(child, fmt.Sprintf("%s[%d]", path, idx))
			if err != nil {
				return nil, err
			}
			projected.Content = append(projected.Content, projectedChild)
		}
		return projected, nil
	case yaml.MappingNode:
		entries, _, err := readMapping(node, path)
		if err != nil {
			return nil, err
		}
		sort.SliceStable(entries, func(i, j int) bool {
			leftKey := entries[i].Key
			rightKey := entries[j].Key
			leftRank := expressionKeyRank(leftKey)
			rightRank := expressionKeyRank(rightKey)
			if leftRank != rightRank {
				return leftRank < rightRank
			}
			return leftKey < rightKey
		})

		projected := mappingNode()
		for _, entry := range entries {
			projectedValue, projectErr := canonicalExpression(entry.Value, path+"."+entry.Key)
			if projectErr != nil {
				return nil, projectErr
			}
			appendMapping(projected, entry.Key, projectedValue)
		}
		return projected, nil
	default:
		return nil, fmt.Errorf("%s uses unsupported yaml node kind", path)
	}
}

func expressionKeyRank(key string) int {
	switch key {
	case "op":
		return 1
	case "filters":
		return 2
	case "filter":
		return 3
	case "field":
		return 4
	case "value":
		return 5
	default:
		return 10
	}
}

func renderProjection(projected *yaml.Node) (string, error) {
	buffer := bytes.Buffer{}
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(projected); err != nil {
		_ = encoder.Close()
		return "", err
	}
	if err := encoder.Close(); err != nil {
		return "", err
	}
	return strings.TrimRight(buffer.String(), "\n"), nil
}
