package schemachecks

import (
	"fmt"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/expressions"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

type StrictMissingUsage struct {
	Operator expressions.Operator
	Operand  expressions.Reference
}

func EnsureOnlyKeys(path string, values map[string]any, allowed ...string) *domainerrors.AppError {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		allowedSet[key] = struct{}{}
	}

	for key := range values {
		if _, ok := allowedSet[key]; ok {
			continue
		}
		return domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("%s has unsupported key '%s'", path, key),
			nil,
		)
	}
	return nil
}

func StrictMissingUsageInRequiredWhen(
	expression *expressions.Expression,
	fieldsByName map[string]model.RequiredFieldRule,
) (StrictMissingUsage, bool) {
	if expression == nil {
		return StrictMissingUsage{}, false
	}

	for _, usage := range expressions.CollectStrictReferenceUsages(expression) {
		if !referencePotentiallyMissing(usage.Reference, fieldsByName) {
			continue
		}
		return StrictMissingUsage{Operator: usage.Operator, Operand: usage.Reference}, true
	}

	return StrictMissingUsage{}, false
}

func referencePotentiallyMissing(reference expressions.Reference, fieldsByName map[string]model.RequiredFieldRule) bool {
	switch reference.Kind {
	case expressions.ReferenceMeta:
		if isBuiltinMetaField(reference.Field) {
			return false
		}

		rule, exists := fieldsByName[reference.Field]
		if !exists {
			return false
		}
		if rule.Type == "entity_ref" {
			return true
		}
		if !rule.Required {
			return true
		}
		return rule.RequiredWhen || rule.RequiredWhenExpr != nil
	case expressions.ReferenceRef:
		_, exists := fieldsByName[reference.Field]
		return exists
	default:
		return true
	}
}

func isBuiltinMetaField(name string) bool {
	switch name {
	case "type", "id", "slug", "created_date", "updated_date":
		return true
	default:
		return false
	}
}
