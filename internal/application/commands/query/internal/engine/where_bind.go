package engine

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

var isoDatePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

const (
	whereDetailsReasonForbiddenField            = "forbidden_field"
	whereDetailsReasonForbiddenOperatorForField = "forbidden_operator_for_field"
)

func bindWhereNode(raw model.RawFilterNode, index model.QuerySchemaIndex) (model.FilterNode, *domainerrors.AppError) {
	switch raw.Kind {
	case model.RawFilterNodeAnd:
		boundFilters := make([]model.FilterNode, 0, len(raw.Filters))
		for _, childRaw := range raw.Filters {
			bound, err := bindWhereNode(childRaw, index)
			if err != nil {
				return model.FilterNode{}, err
			}
			boundFilters = append(boundFilters, bound)
		}
		return model.FilterNode{Kind: model.FilterNodeAnd, Filters: boundFilters}, nil
	case model.RawFilterNodeOr:
		boundFilters := make([]model.FilterNode, 0, len(raw.Filters))
		for _, childRaw := range raw.Filters {
			bound, err := bindWhereNode(childRaw, index)
			if err != nil {
				return model.FilterNode{}, err
			}
			boundFilters = append(boundFilters, bound)
		}
		return model.FilterNode{Kind: model.FilterNodeOr, Filters: boundFilters}, nil
	case model.RawFilterNodeNot:
		if raw.Filter == nil {
			return model.FilterNode{}, domainerrors.New(
				domainerrors.CodeInvalidQuery,
				"not filter must contain nested filter",
				nil,
			)
		}
		bound, err := bindWhereNode(*raw.Filter, index)
		if err != nil {
			return model.FilterNode{}, err
		}
		return model.FilterNode{Kind: model.FilterNodeNot, Filter: &bound}, nil
	case model.RawFilterNodeLeaf:
		if isPolicyForbiddenWhereField(raw.Field) {
			return model.FilterNode{}, domainerrors.New(
				domainerrors.CodeInvalidQuery,
				fmt.Sprintf("field '%s' is not allowed in where-json", raw.Field),
				map[string]any{
					"arg":    "--where-json",
					"reason": whereDetailsReasonForbiddenField,
					"field":  raw.Field,
				},
			)
		}

		spec, exists := index.FilterFields[raw.Field]
		if !exists {
			return model.FilterNode{}, domainerrors.New(
				domainerrors.CodeInvalidQuery,
				fmt.Sprintf("unknown filter field '%s'", raw.Field),
				nil,
			)
		}

		if err := validateLeafOperation(spec, raw.Op, raw.Value, raw.HasValue); err != nil {
			return model.FilterNode{}, err
		}
		return model.FilterNode{
			Kind:     model.FilterNodeLeaf,
			Field:    raw.Field,
			Op:       raw.Op,
			Value:    raw.Value,
			HasValue: raw.HasValue,
			Spec:     spec,
		}, nil
	default:
		return model.FilterNode{}, domainerrors.New(
			domainerrors.CodeInvalidQuery,
			"unsupported filter node kind",
			nil,
		)
	}
}

func validateLeafOperation(spec model.SchemaFieldSpec, op string, value any, hasValue bool) *domainerrors.AppError {
	if _, supported := whereOperators[op]; !supported {
		return domainerrors.New(
			domainerrors.CodeInvalidQuery,
			fmt.Sprintf("unknown operator '%s'", op),
			nil,
		)
	}

	if !hasValue {
		if op == "exists" || op == "not_exists" {
			return nil
		}
		return domainerrors.New(
			domainerrors.CodeInvalidQuery,
			fmt.Sprintf("operator '%s' requires value", op),
			nil,
		)
	}

	if op == "exists" || op == "not_exists" {
		return domainerrors.New(
			domainerrors.CodeInvalidQuery,
			fmt.Sprintf("operator '%s' does not accept value", op),
			nil,
		)
	}

	if !isOpAllowedForField(spec, op) {
		var details map[string]any
		if strings.HasPrefix(spec.Path, "content.sections.") {
			details = map[string]any{
				"arg":      "--where-json",
				"reason":   whereDetailsReasonForbiddenOperatorForField,
				"field":    spec.Path,
				"operator": op,
			}
		}
		return domainerrors.New(
			domainerrors.CodeInvalidQuery,
			fmt.Sprintf("operator '%s' is not allowed for field '%s'", op, spec.Path),
			details,
		)
	}

	switch op {
	case "gt", "gte", "lt", "lte":
		if spec.Kind == model.FieldKindDate {
			dateString, ok := value.(string)
			if !ok || !isoDatePattern.MatchString(strings.TrimSpace(dateString)) {
				return domainerrors.New(
					domainerrors.CodeInvalidQuery,
					fmt.Sprintf("operator '%s' requires YYYY-MM-DD date value for field '%s'", op, spec.Path),
					nil,
				)
			}
			return nil
		}
		if _, ok := support.NumberToFloat64(value); !ok {
			return domainerrors.New(
				domainerrors.CodeInvalidQuery,
				fmt.Sprintf("operator '%s' requires numeric value for field '%s'", op, spec.Path),
				nil,
			)
		}
	case "contains":
		if spec.Kind == model.FieldKindString || spec.Kind == model.FieldKindDate {
			if _, ok := value.(string); !ok {
				return domainerrors.New(
					domainerrors.CodeInvalidQuery,
					fmt.Sprintf("operator '%s' requires string value for field '%s'", op, spec.Path),
					nil,
				)
			}
		}
	case "in", "not_in":
		arrayValue, ok := value.([]any)
		if !ok {
			return domainerrors.New(
				domainerrors.CodeInvalidQuery,
				fmt.Sprintf("operator '%s' requires array value", op),
				nil,
			)
		}
		for _, item := range arrayValue {
			if err := validateScalarCompatibility(spec.Kind, item, spec.Path); err != nil {
				return err
			}
		}
	default:
		if err := validateScalarCompatibility(spec.Kind, value, spec.Path); err != nil {
			return err
		}
	}

	if len(spec.EnumValues) > 0 {
		switch op {
		case "eq", "neq":
			if !containsEnumValue(spec.EnumValues, value) {
				return domainerrors.New(
					domainerrors.CodeInvalidQuery,
					fmt.Sprintf("value is not allowed by enum for field '%s'", spec.Path),
					nil,
				)
			}
		case "in", "not_in":
			for _, item := range value.([]any) {
				if !containsEnumValue(spec.EnumValues, item) {
					return domainerrors.New(
						domainerrors.CodeInvalidQuery,
						fmt.Sprintf("value is not allowed by enum for field '%s'", spec.Path),
						nil,
					)
				}
			}
		}
	}

	return nil
}

func validateScalarCompatibility(kind model.SchemaFieldKind, value any, path string) *domainerrors.AppError {
	switch kind {
	case model.FieldKindString:
		if _, ok := value.(string); !ok {
			return domainerrors.New(
				domainerrors.CodeInvalidQuery,
				fmt.Sprintf("type mismatch for field '%s': expected string", path),
				nil,
			)
		}
	case model.FieldKindDate:
		s, ok := value.(string)
		if !ok || !isoDatePattern.MatchString(strings.TrimSpace(s)) {
			return domainerrors.New(
				domainerrors.CodeInvalidQuery,
				fmt.Sprintf("type mismatch for field '%s': expected YYYY-MM-DD string", path),
				nil,
			)
		}
	case model.FieldKindNumber:
		if _, ok := support.NumberToFloat64(value); !ok {
			return domainerrors.New(
				domainerrors.CodeInvalidQuery,
				fmt.Sprintf("type mismatch for field '%s': expected number", path),
				nil,
			)
		}
	case model.FieldKindBoolean:
		if _, ok := value.(bool); !ok {
			return domainerrors.New(
				domainerrors.CodeInvalidQuery,
				fmt.Sprintf("type mismatch for field '%s': expected boolean", path),
				nil,
			)
		}
	case model.FieldKindArray:
		if _, ok := value.([]any); !ok {
			return domainerrors.New(
				domainerrors.CodeInvalidQuery,
				fmt.Sprintf("type mismatch for field '%s': expected array", path),
				nil,
			)
		}
	}
	return nil
}

func isOpAllowedForKind(kind model.SchemaFieldKind, op string) bool {
	switch kind {
	case model.FieldKindString:
		return containsOp(op, "eq", "neq", "in", "not_in", "contains", "exists", "not_exists")
	case model.FieldKindDate:
		return containsOp(op, "eq", "neq", "in", "not_in", "gt", "gte", "lt", "lte", "contains", "exists", "not_exists")
	case model.FieldKindNumber:
		return containsOp(op, "eq", "neq", "in", "not_in", "gt", "gte", "lt", "lte", "exists", "not_exists")
	case model.FieldKindArray:
		return containsOp(op, "eq", "neq", "in", "not_in", "contains", "exists", "not_exists")
	case model.FieldKindBoolean:
		return containsOp(op, "eq", "neq", "in", "not_in", "exists", "not_exists")
	default:
		return containsOp(op, "eq", "neq", "in", "not_in", "exists", "not_exists")
	}
}

func isOpAllowedForField(spec model.SchemaFieldSpec, op string) bool {
	if strings.HasPrefix(spec.Path, "content.sections.") {
		return containsOp(op, "contains", "exists", "not_exists")
	}
	return isOpAllowedForKind(spec.Kind, op)
}

func isPolicyForbiddenWhereField(field string) bool {
	return field == "content.raw"
}

func containsOp(candidate string, allowed ...string) bool {
	for _, op := range allowed {
		if candidate == op {
			return true
		}
	}
	return false
}

func containsEnumValue(enumValues []any, candidate any) bool {
	for _, enumValue := range enumValues {
		if support.LiteralEqual(enumValue, candidate) {
			return true
		}
	}
	return false
}
