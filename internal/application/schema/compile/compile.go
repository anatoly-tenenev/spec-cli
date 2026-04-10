package compile

import (
	"errors"
	"os"
	"strings"
	"sync"

	internalcompiler "github.com/anatoly-tenenev/spec-cli/internal/application/schema/compile/internal/compiler"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/diagnostics"
	schemaexpressions "github.com/anatoly-tenenev/spec-cli/internal/application/schema/expressions"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/source"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

type Result struct {
	Schema  model.CompiledSchema
	Issues  []diagnostics.Issue
	Summary diagnostics.Summary
	Valid   bool
}

type Compiler struct {
	mu    sync.Mutex
	cache map[cacheKey]cacheEntry
}

type cacheKey struct {
	path        string
	displayPath string
}

type cacheEntry struct {
	Result Result
	Err    *domainerrors.AppError
}

func NewCompiler() *Compiler {
	return &Compiler{cache: make(map[cacheKey]cacheEntry)}
}

func Compile(path string, displayPath string) (Result, *domainerrors.AppError) {
	compiler := NewCompiler()
	return compiler.Compile(path, displayPath)
}

func (c *Compiler) Compile(path string, displayPath string) (Result, *domainerrors.AppError) {
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
		return cloneResult(cached.Result), cloneAppError(cached.Err)
	}
	c.mu.Unlock()

	doc, sourceIssues, sourceErr := source.Load(path, displayPath)
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
		Issues:  cloneIssues(issues),
		Summary: summary,
		Valid:   summary.Errors == 0,
	}
	compileErr := classifyCompileError(result, sourceErr)

	c.mu.Lock()
	c.cache[key] = cacheEntry{
		Result: cloneResult(result),
		Err:    cloneAppError(compileErr),
	}
	c.mu.Unlock()

	return result, compileErr
}

func cloneResult(value Result) Result {
	copyResult := Result{
		Schema:  cloneSchema(value.Schema),
		Issues:  cloneIssues(value.Issues),
		Summary: value.Summary,
		Valid:   value.Valid,
	}
	return copyResult
}

func cloneAppError(appErr *domainerrors.AppError) *domainerrors.AppError {
	if appErr == nil {
		return nil
	}

	details := map[string]any(nil)
	if len(appErr.Details) > 0 {
		details = make(map[string]any, len(appErr.Details))
		for key, value := range appErr.Details {
			details[key] = value
		}
	}

	return &domainerrors.AppError{
		Code:     appErr.Code,
		Message:  appErr.Message,
		Details:  details,
		ExitCode: appErr.ExitCode,
	}
}

func classifyCompileError(result Result, sourceErr error) *domainerrors.AppError {
	if result.Summary.Errors == 0 {
		return nil
	}

	if sourceErr != nil {
		if errors.Is(sourceErr, os.ErrNotExist) {
			return domainerrors.New(
				domainerrors.CodeSchemaNotFound,
				"schema file does not exist",
				nil,
			)
		}
		return domainerrors.New(
			domainerrors.CodeSchemaReadError,
			"schema file is not readable",
			nil,
		)
	}

	if hasParseIssues(result.Issues) {
		return domainerrors.New(
			domainerrors.CodeSchemaParseError,
			parseErrorMessage(result.Issues),
			nil,
		)
	}

	return domainerrors.New(
		domainerrors.CodeSchemaInvalid,
		"schema contains validation errors",
		nil,
	)
}

func hasParseIssues(issues []diagnostics.Issue) bool {
	for _, issue := range issues {
		if issue.Level != diagnostics.LevelError {
			continue
		}
		if isParseIssueCode(issue.Code) {
			return true
		}
	}
	return false
}

func parseErrorMessage(issues []diagnostics.Issue) string {
	for _, issue := range issues {
		if issue.Level != diagnostics.LevelError {
			continue
		}
		switch issue.Code {
		case "schema.source.empty":
			return "schema file is empty"
		case "schema.source.parse_failed":
			return "failed to parse schema yaml/json"
		}
		if isParseIssueCode(issue.Code) {
			return issue.Message
		}
	}
	return "failed to parse schema yaml/json"
}

func isParseIssueCode(code string) bool {
	switch code {
	case "schema.source.empty",
		"schema.source.parse_failed",
		"schema.source.decode_failed",
		"schema.source.bootstrap_failed",
		"schema.source.bootstrap_parse_failed":
		return true
	}
	if strings.Contains(code, ".parse_") || strings.Contains(code, ".decode_") {
		return true
	}
	if strings.Contains(code, ".bootstrap_") {
		return true
	}
	return false
}

func cloneIssues(issues []diagnostics.Issue) []diagnostics.Issue {
	if len(issues) == 0 {
		return []diagnostics.Issue{}
	}
	return append([]diagnostics.Issue(nil), issues...)
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
		Name:           entity.Name,
		IDPrefix:       entity.IDPrefix,
		PathTemplate:   model.PathTemplate{Cases: cases},
		MetaFields:     metaFields,
		MetaFieldOrder: append([]string(nil), entity.MetaFieldOrder...),
		Sections:       sections,
		SectionOrder:   append([]string(nil), entity.SectionOrder...),
		HasContent:     entity.HasContent,
		Description:    entity.Description,
	}
}

func cloneMetaField(field model.MetaField) model.MetaField {
	return model.MetaField{
		Name:        field.Name,
		Value:       cloneValueSpec(field.Value),
		Required:    cloneRequirement(field.Required),
		Description: field.Description,
		SchemaPath:  field.SchemaPath,
	}
}

func cloneSection(section model.Section) model.Section {
	return model.Section{
		Name:        section.Name,
		Title:       section.Title,
		Required:    cloneRequirement(section.Required),
		Description: section.Description,
		SchemaPath:  section.SchemaPath,
		TitlePath:   section.TitlePath,
	}
}

func clonePathCase(pathCase model.PathTemplateCase) model.PathTemplateCase {
	return model.PathTemplateCase{
		Use:         pathCase.Use,
		UseTemplate: cloneTemplate(pathCase.UseTemplate),
		When:        cloneRequirement(pathCase.When),
		UsePath:     pathCase.UsePath,
	}
}

func cloneRequirement(requirement model.Requirement) model.Requirement {
	return model.Requirement{
		Always: requirement.Always,
		Expr:   cloneExpression(requirement.Expr),
		Path:   requirement.Path,
	}
}

func cloneTemplate(template *schemaexpressions.CompiledTemplate) *schemaexpressions.CompiledTemplate {
	if template == nil {
		return nil
	}
	parts := make([]schemaexpressions.TemplatePart, len(template.Parts))
	for idx, part := range template.Parts {
		parts[idx] = schemaexpressions.TemplatePart{
			Literal:    part.Literal,
			Expression: cloneExpression(part.Expression),
		}
	}
	return &schemaexpressions.CompiledTemplate{
		Raw:   template.Raw,
		Parts: parts,
	}
}

func cloneExpression(expr *schemaexpressions.CompiledExpression) *schemaexpressions.CompiledExpression {
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
		copyConst := model.Literal{
			Value:    spec.Const.Value,
			Template: cloneTemplate(spec.Const.Template),
		}
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
		for idx, value := range spec.Enum {
			copySpec.Enum[idx] = model.Literal{
				Value:    value.Value,
				Template: cloneTemplate(value.Template),
			}
		}
	}
	return copySpec
}
