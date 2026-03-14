package expressioncontext

import (
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/expressions"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

var builtinMetaSpecs = map[string]expressions.MetaFieldSpec{
	"type":         {Type: "string", Comparable: true},
	"id":           {Type: "string", Comparable: true},
	"slug":         {Type: "string", Comparable: true},
	"created_date": {Type: "string", Comparable: true},
	"updated_date": {Type: "string", Comparable: true},
}

func Build(fields []model.RequiredFieldRule) expressions.CompileContext {
	metaSpecs := make(map[string]expressions.MetaFieldSpec, len(builtinMetaSpecs)+len(fields))
	for name, spec := range builtinMetaSpecs {
		metaSpecs[name] = spec
	}
	for _, field := range fields {
		metaSpecs[field.Name] = expressions.MetaFieldSpec{
			Type:       field.Type,
			Comparable: isComparableFieldType(field.Type),
			EntityRef:  field.Type == "entity_ref",
		}
	}
	return expressions.CompileContext{MetaFields: metaSpecs}
}

func IsBuiltinMetaField(fieldName string) bool {
	_, exists := builtinMetaSpecs[fieldName]
	return exists
}

func isComparableFieldType(typeName string) bool {
	switch typeName {
	case "string", "integer", "number", "boolean", "null", "entity_ref":
		return true
	default:
		return false
	}
}

func FromCompileIssue(issue expressions.CompileIssue) domainvalidation.Issue {
	standardRef := issue.StandardRef
	if standardRef == "" {
		standardRef = "11.6"
	}

	return domainvalidation.Issue{
		Code:        issue.Code,
		Level:       domainvalidation.LevelError,
		Class:       "SchemaError",
		Message:     issue.Message,
		StandardRef: standardRef,
		Field:       issue.Field,
	}
}
