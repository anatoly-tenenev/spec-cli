package compile

import (
	"strings"
	"sync"

	internalcompiler "github.com/anatoly-tenenev/spec-cli/internal/application/schema/compile/internal/compiler"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/diagnostics"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/source"
)

type Result struct {
	Schema  model.CompiledSchema
	Issues  []diagnostics.Issue
	Summary diagnostics.Summary
	Valid   bool
}

type Compiler struct {
	mu    sync.Mutex
	cache map[cacheKey]Result
}

type cacheKey struct {
	path        string
	displayPath string
}

func NewCompiler() *Compiler {
	return &Compiler{cache: make(map[cacheKey]Result)}
}

func Compile(path string, displayPath string) Result {
	compiler := NewCompiler()
	return compiler.Compile(path, displayPath)
}

func (c *Compiler) Compile(path string, displayPath string) Result {
	if c == nil {
		c = NewCompiler()
	}

	key := cacheKey{
		path:        strings.TrimSpace(path),
		displayPath: strings.TrimSpace(displayPath),
	}

	c.mu.Lock()
	if cached, ok := c.cache[key]; ok {
		c.mu.Unlock()
		return cloneResult(cached)
	}
	c.mu.Unlock()

	doc, sourceIssues := source.Load(path, displayPath)
	issues := make([]diagnostics.Issue, 0, len(sourceIssues)+8)
	issues = append(issues, sourceIssues...)

	compiled := model.CompiledSchema{}
	if len(sourceIssues) == 0 {
		schema, compileIssues := internalcompiler.CompileDocument(doc)
		compiled = schema
		issues = append(issues, compileIssues...)
	}

	summary := diagnostics.Summarize(issues)
	result := Result{
		Schema:  compiled,
		Issues:  append([]diagnostics.Issue(nil), issues...),
		Summary: summary,
		Valid:   summary.Errors == 0,
	}

	c.mu.Lock()
	c.cache[key] = cloneResult(result)
	c.mu.Unlock()

	return result
}

func cloneResult(value Result) Result {
	copyResult := Result{
		Schema:  cloneSchema(value.Schema),
		Issues:  append([]diagnostics.Issue(nil), value.Issues...),
		Summary: value.Summary,
		Valid:   value.Valid,
	}
	return copyResult
}

func cloneSchema(schema model.CompiledSchema) model.CompiledSchema {
	entities := make(map[string]model.EntityType, len(schema.Entities))
	for name, entity := range schema.Entities {
		entities[name] = cloneEntity(entity)
	}
	return model.CompiledSchema{
		Version:     schema.Version,
		Description: schema.Description,
		Entities:    entities,
	}
}

func cloneEntity(entity model.EntityType) model.EntityType {
	metaFields := make(map[string]model.MetaField, len(entity.MetaFields))
	for name, field := range entity.MetaFields {
		metaFields[name] = cloneMetaField(field)
	}

	sections := make(map[string]model.Section, len(entity.Sections))
	for name, section := range entity.Sections {
		sections[name] = cloneSection(section)
	}

	cases := make([]model.PathTemplateCase, len(entity.PathTemplate.Cases))
	for idx, pathCase := range entity.PathTemplate.Cases {
		cases[idx] = clonePathCase(pathCase)
	}

	return model.EntityType{
		Name:         entity.Name,
		IDPrefix:     entity.IDPrefix,
		PathTemplate: model.PathTemplate{Cases: cases},
		MetaFields:   metaFields,
		Sections:     sections,
		Description:  entity.Description,
	}
}

func cloneMetaField(field model.MetaField) model.MetaField {
	return model.MetaField{
		Name:        field.Name,
		Value:       cloneValueSpec(field.Value),
		Required:    cloneRequirement(field.Required),
		Description: field.Description,
	}
}

func cloneSection(section model.Section) model.Section {
	titles := append([]string(nil), section.Titles...)
	return model.Section{
		Name:        section.Name,
		Titles:      titles,
		Required:    cloneRequirement(section.Required),
		Description: section.Description,
	}
}

func clonePathCase(pathCase model.PathTemplateCase) model.PathTemplateCase {
	return model.PathTemplateCase{
		Use:         pathCase.Use,
		UseTemplate: cloneTemplate(pathCase.UseTemplate),
		When:        cloneRequirement(pathCase.When),
	}
}

func cloneRequirement(requirement model.Requirement) model.Requirement {
	return model.Requirement{
		Always: requirement.Always,
		Expr:   cloneExpression(requirement.Expr),
	}
}

func cloneTemplate(template *model.CompiledTemplate) *model.CompiledTemplate {
	if template == nil {
		return nil
	}
	parts := make([]model.TemplatePart, len(template.Parts))
	for idx, part := range template.Parts {
		parts[idx] = model.TemplatePart{
			Literal: part.Literal,
			Expr:    cloneExpression(part.Expr),
		}
	}
	return &model.CompiledTemplate{Parts: parts}
}

func cloneExpression(expr *model.CompiledExpression) *model.CompiledExpression {
	if expr == nil {
		return nil
	}
	copyExpr := *expr
	return &copyExpr
}

func cloneValueSpec(spec model.ValueSpec) model.ValueSpec {
	copySpec := model.ValueSpec{
		Kind:        spec.Kind,
		Format:      spec.Format,
		UniqueItems: spec.UniqueItems,
	}
	if spec.Const != nil {
		copyConst := *spec.Const
		copySpec.Const = &copyConst
	}
	if spec.Ref != nil {
		copyRef := model.RefSpec{
			Cardinality:  spec.Ref.Cardinality,
			AllowedTypes: append([]string(nil), spec.Ref.AllowedTypes...),
		}
		copySpec.Ref = &copyRef
	}
	if spec.Items != nil {
		copyItems := cloneValueSpec(*spec.Items)
		copySpec.Items = &copyItems
	}
	if spec.MinItems != nil {
		min := *spec.MinItems
		copySpec.MinItems = &min
	}
	if spec.MaxItems != nil {
		max := *spec.MaxItems
		copySpec.MaxItems = &max
	}
	if len(spec.Enum) > 0 {
		copySpec.Enum = make([]model.Literal, len(spec.Enum))
		copy(copySpec.Enum, spec.Enum)
	}
	return copySpec
}
