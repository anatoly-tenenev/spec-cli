package entity

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	expressioncontext "github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/schema/internal/entity/internal/expressioncontext"
	metafields "github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/schema/internal/entity/internal/metafields"
	pathpattern "github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/schema/internal/entity/internal/pathpattern"
	schemachecks "github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/schema/internal/entity/internal/schemachecks"
	sections "github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/schema/internal/entity/internal/sections"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	"github.com/anatoly-tenenev/spec-cli/internal/domain/reservedkeys"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

var (
	idPrefixPattern = regexp.MustCompile(`^[A-Za-z0-9_]+(?:-[A-Za-z0-9_]+)*$`)
)

func ParseType(
	typeName string,
	typeConfig map[string]any,
	typeSet map[string]struct{},
	usedPrefixes map[string]string,
) (model.SchemaEntityType, []domainvalidation.Issue, *domainerrors.AppError) {
	if keyErr := schemachecks.EnsureOnlyKeys(
		fmt.Sprintf("schema.entity.%s", typeName),
		typeConfig,
		reservedkeys.SchemaKeyIDPrefix,
		reservedkeys.SchemaKeyPathTemplate,
		"meta",
		"content",
		"description",
	); keyErr != nil {
		return model.SchemaEntityType{}, nil, keyErr
	}

	idPrefix, idPrefixErr := parseIDPrefix(typeName, typeConfig[reservedkeys.SchemaKeyIDPrefix], usedPrefixes)
	if idPrefixErr != nil {
		return model.SchemaEntityType{}, nil, idPrefixErr
	}

	requiredFields, fieldIssues, requiredFieldErr := metafields.Parse(typeName, typeConfig["meta"], typeSet)
	if requiredFieldErr != nil {
		return model.SchemaEntityType{}, nil, requiredFieldErr
	}
	fieldByName := make(map[string]model.RequiredFieldRule, len(requiredFields))
	for _, fieldRule := range requiredFields {
		fieldByName[fieldRule.Name] = fieldRule
	}

	expressionContext := expressioncontext.Build(requiredFields)
	requiredSections, sectionIssues, requiredSectionsErr := sections.Parse(typeName, typeConfig["content"], expressionContext, fieldByName)
	if requiredSectionsErr != nil {
		return model.SchemaEntityType{}, nil, requiredSectionsErr
	}

	pathRule, pathIssues, pathErr := pathpattern.Parse(typeName, typeConfig[reservedkeys.SchemaKeyPathTemplate], expressionContext, fieldByName)
	if pathErr != nil {
		return model.SchemaEntityType{}, nil, pathErr
	}

	issues := make([]domainvalidation.Issue, 0, len(fieldIssues)+len(sectionIssues)+len(pathIssues))
	issues = append(issues, fieldIssues...)
	issues = append(issues, sectionIssues...)
	issues = append(issues, pathIssues...)

	return model.SchemaEntityType{
		Name:             typeName,
		IDPrefix:         idPrefix,
		RequiredFields:   requiredFields,
		RequiredSections: requiredSections,
		PathPattern:      pathRule,
	}, issues, nil
}

func parseIDPrefix(typeName string, rawIDPrefix any, usedPrefixes map[string]string) (string, *domainerrors.AppError) {
	idPrefix, ok := rawIDPrefix.(string)
	if !ok || strings.TrimSpace(idPrefix) == "" {
		return "", domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.%s must be a non-empty string", typeName, reservedkeys.SchemaKeyIDPrefix),
			nil,
		)
	}

	if !idPrefixPattern.MatchString(idPrefix) {
		return "", domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema.entity.%s.%s has invalid format", typeName, reservedkeys.SchemaKeyIDPrefix),
			nil,
		)
	}

	if existingType, exists := usedPrefixes[idPrefix]; exists {
		return "", domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("schema contains duplicated %s across entity types", reservedkeys.SchemaKeyIDPrefix),
			map[string]any{reservedkeys.SchemaKeyIDPrefix: idPrefix, "types": []string{existingType, typeName}},
		)
	}

	usedPrefixes[idPrefix] = typeName
	return idPrefix, nil
}
