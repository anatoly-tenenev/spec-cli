package engine

import (
	"fmt"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func parseWhereJSON(raw string) (model.RawFilterNode, *domainerrors.AppError) {
	parsed, err := ParseJSONPayload(raw)
	if err != nil {
		return model.RawFilterNode{}, domainerrors.New(
			domainerrors.CodeInvalidQuery,
			"--where-json must be valid JSON",
			nil,
		)
	}
	return parseRawFilterNode(parsed)
}

func parseRawFilterNode(raw any) (model.RawFilterNode, *domainerrors.AppError) {
	nodeMap, ok := raw.(map[string]any)
	if !ok {
		return model.RawFilterNode{}, domainerrors.New(
			domainerrors.CodeInvalidQuery,
			"filter node must be a JSON object",
			nil,
		)
	}

	_, hasField := nodeMap["field"]
	_, hasFilters := nodeMap["filters"]
	_, hasFilter := nodeMap["filter"]

	isLeaf := hasField
	isLogical := hasFilters || hasFilter

	if isLeaf && isLogical {
		return model.RawFilterNode{}, domainerrors.New(
			domainerrors.CodeInvalidQuery,
			"filter node cannot mix logical and leaf forms",
			nil,
		)
	}
	if !isLeaf && !isLogical {
		return model.RawFilterNode{}, domainerrors.New(
			domainerrors.CodeInvalidQuery,
			"filter node must be logical or leaf",
			nil,
		)
	}

	rawOp, ok := nodeMap["op"].(string)
	if !ok || rawOp == "" {
		return model.RawFilterNode{}, domainerrors.New(
			domainerrors.CodeInvalidQuery,
			"filter node must include non-empty string field 'op'",
			nil,
		)
	}

	if isLogical {
		return parseRawLogicalNode(rawOp, nodeMap)
	}
	return parseRawLeafNode(rawOp, nodeMap)
}

func parseRawLogicalNode(op string, raw map[string]any) (model.RawFilterNode, *domainerrors.AppError) {
	switch op {
	case "and", "or":
		rawFilters, ok := raw["filters"]
		if !ok {
			return model.RawFilterNode{}, domainerrors.New(
				domainerrors.CodeInvalidQuery,
				fmt.Sprintf("logical op '%s' requires 'filters'", op),
				nil,
			)
		}
		filtersArray, ok := rawFilters.([]any)
		if !ok || len(filtersArray) == 0 {
			return model.RawFilterNode{}, domainerrors.New(
				domainerrors.CodeInvalidQuery,
				fmt.Sprintf("logical op '%s' requires non-empty array 'filters'", op),
				nil,
			)
		}

		parsedFilters := make([]model.RawFilterNode, 0, len(filtersArray))
		for _, rawChild := range filtersArray {
			childNode, err := parseRawFilterNode(rawChild)
			if err != nil {
				return model.RawFilterNode{}, err
			}
			parsedFilters = append(parsedFilters, childNode)
		}

		kind := model.RawFilterNodeAnd
		if op == "or" {
			kind = model.RawFilterNodeOr
		}
		return model.RawFilterNode{Kind: kind, Filters: parsedFilters}, nil
	case "not":
		rawFilter, ok := raw["filter"]
		if !ok {
			return model.RawFilterNode{}, domainerrors.New(
				domainerrors.CodeInvalidQuery,
				"logical op 'not' requires 'filter'",
				nil,
			)
		}
		parsedFilter, err := parseRawFilterNode(rawFilter)
		if err != nil {
			return model.RawFilterNode{}, err
		}
		return model.RawFilterNode{Kind: model.RawFilterNodeNot, Filter: &parsedFilter}, nil
	default:
		return model.RawFilterNode{}, domainerrors.New(
			domainerrors.CodeInvalidQuery,
			fmt.Sprintf("unknown logical operator '%s'", op),
			nil,
		)
	}
}

func parseRawLeafNode(op string, raw map[string]any) (model.RawFilterNode, *domainerrors.AppError) {
	rawField, ok := raw["field"].(string)
	if !ok || rawField == "" {
		return model.RawFilterNode{}, domainerrors.New(
			domainerrors.CodeInvalidQuery,
			"leaf filter must include non-empty string field 'field'",
			nil,
		)
	}

	_, knownOp := whereOperators[op]
	if !knownOp {
		return model.RawFilterNode{}, domainerrors.New(
			domainerrors.CodeInvalidQuery,
			fmt.Sprintf("unknown operator '%s'", op),
			nil,
		)
	}

	rawValue, hasValue := raw["value"]
	if op == "exists" || op == "not_exists" {
		if hasValue {
			return model.RawFilterNode{}, domainerrors.New(
				domainerrors.CodeInvalidQuery,
				fmt.Sprintf("operator '%s' does not accept value", op),
				nil,
			)
		}
	} else {
		if !hasValue {
			return model.RawFilterNode{}, domainerrors.New(
				domainerrors.CodeInvalidQuery,
				fmt.Sprintf("operator '%s' requires value", op),
				nil,
			)
		}
	}

	if op == "in" || op == "not_in" {
		if _, ok := rawValue.([]any); !ok {
			return model.RawFilterNode{}, domainerrors.New(
				domainerrors.CodeInvalidQuery,
				fmt.Sprintf("operator '%s' requires array value", op),
				nil,
			)
		}
	}

	return model.RawFilterNode{
		Kind:     model.RawFilterNodeLeaf,
		Field:    rawField,
		Op:       op,
		Value:    rawValue,
		HasValue: hasValue,
	}, nil
}

var whereOperators = map[string]struct{}{
	"eq":         {},
	"neq":        {},
	"in":         {},
	"not_in":     {},
	"exists":     {},
	"not_exists": {},
	"gt":         {},
	"gte":        {},
	"lt":         {},
	"lte":        {},
	"contains":   {},
}
