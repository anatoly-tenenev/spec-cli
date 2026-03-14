package names

import (
	"fmt"
	"regexp"

	expressioncontext "github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/schema/internal/entity/internal/expressioncontext"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

var schemaKeyNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

func ValidateMetaFieldName(typeName string, fieldName string) *domainerrors.AppError {
	if !schemaKeyNameRE.MatchString(fieldName) {
		return domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields has invalid field name '%s'", typeName, fieldName),
			nil,
		)
	}

	if expressioncontext.IsBuiltinMetaField(fieldName) {
		return domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.meta.fields cannot redefine built-in field '%s'", typeName, fieldName),
			nil,
		)
	}
	return nil
}

func ValidateSectionName(typeName string, sectionName string) *domainerrors.AppError {
	if !schemaKeyNameRE.MatchString(sectionName) {
		return domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.content.sections has invalid section name '%s'", typeName, sectionName),
			nil,
		)
	}
	return nil
}
