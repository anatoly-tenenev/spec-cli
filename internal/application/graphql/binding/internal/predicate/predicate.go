package predicate

import (
	"fmt"
	"strings"

	gqlmodel "github.com/anatoly-tenenev/spec-cli/internal/application/graphql/model"
	readmodel "github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/model"
)

func Build(raw any) gqlmodel.Predicate {
	if raw == nil {
		return nil
	}
	where, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	return func(entity readmodel.EntityView) bool {
		return evalObject(entity.WhereContext, where)
	}
}

func evalObject(context map[string]any, where map[string]any) bool {
	for key, cond := range where {
		switch key {
		case "and":
			if !evalList(context, cond, true) {
				return false
			}
		case "or":
			if !evalList(context, cond, false) {
				return false
			}
		case "not":
			if obj, ok := cond.(map[string]any); ok && evalObject(context, obj) {
				return false
			}
		case "meta", "refs", "content":
			nested, _ := resolveChild(context, key)
			if nestedMap, ok := nested.(map[string]any); ok {
				if !evalNamespace(nestedMap, cond) {
					return false
				}
			} else if !evalNamespace(nil, cond) {
				return false
			}
		default:
			value, present := resolveChild(context, key)
			filter, _ := cond.(map[string]any)
			if !evalFilter(value, present, filter) {
				return false
			}
		}
	}
	return true
}

func evalNamespace(source map[string]any, raw any) bool {
	where, ok := raw.(map[string]any)
	if !ok {
		return true
	}
	for key, cond := range where {
		value, present := resolveChild(source, key)
		if nested, ok := cond.(map[string]any); ok && isNestedObject(nested) {
			childMap, _ := value.(map[string]any)
			if !evalNamespace(childMap, nested) {
				return false
			}
			continue
		}
		filter, _ := cond.(map[string]any)
		if !evalFilter(value, present, filter) {
			return false
		}
	}
	return true
}

func evalFilter(value any, present bool, filter map[string]any) bool {
	if filter == nil {
		return true
	}
	if usesArrayFilter(value, filter) {
		return evalArrayFilter(value, present, filter)
	}
	for op, expected := range filter {
		switch op {
		case "exists":
			if boolValue(expected) != present {
				return false
			}
		case "notExists":
			if boolValue(expected) == present {
				return false
			}
		case "eq":
			if !present || compareAny(value, expected) != 0 {
				return false
			}
		case "neq":
			if present && compareAny(value, expected) == 0 {
				return false
			}
		case "in":
			if !present || !containsValue(expected, value) {
				return false
			}
		case "notIn":
			if present && containsValue(expected, value) {
				return false
			}
		case "contains":
			text, ok := value.(string)
			needle, okNeedle := expected.(string)
			if !present || !ok || !okNeedle || !strings.Contains(text, needle) {
				return false
			}
		case "gt", "gte", "lt", "lte":
			compared := compareAny(value, expected)
			if !present || !compareByOp(compared, op) {
				return false
			}
		}
	}
	return true
}

func evalArrayFilter(value any, present bool, filter map[string]any) bool {
	items, _ := value.([]any)
	for op, expected := range filter {
		switch op {
		case "exists":
			if boolValue(expected) != present {
				return false
			}
		case "notExists":
			if boolValue(expected) == present {
				return false
			}
		case "contains":
			if !present || !arrayContains(items, expected) {
				return false
			}
		case "any":
			if !present || !arrayAny(items, expected) {
				return false
			}
		case "all":
			if !present || !arrayAll(items, expected) {
				return false
			}
		case "none":
			if !present || arrayAny(items, expected) {
				return false
			}
		}
	}
	return true
}

func resolveChild(source map[string]any, key string) (any, bool) {
	if source == nil {
		return nil, false
	}
	value, exists := source[key]
	return value, exists
}

func evalList(context map[string]any, raw any, all bool) bool {
	items, ok := raw.([]any)
	if !ok {
		return true
	}
	if all {
		for _, item := range items {
			obj, _ := item.(map[string]any)
			if !evalObject(context, obj) {
				return false
			}
		}
		return true
	}
	for _, item := range items {
		obj, _ := item.(map[string]any)
		if evalObject(context, obj) {
			return true
		}
	}
	return false
}

func isNestedObject(obj map[string]any) bool {
	if len(obj) == 0 {
		return false
	}
	for key := range obj {
		switch key {
		case "eq", "neq", "in", "notIn", "contains", "exists", "notExists", "gt", "gte", "lt", "lte", "any", "all", "none":
			return false
		}
	}
	return true
}

func usesArrayFilter(value any, filter map[string]any) bool {
	for key := range filter {
		switch key {
		case "any", "all", "none":
			return true
		case "contains":
			if _, ok := value.([]any); ok {
				return true
			}
		}
	}
	return false
}

func boolValue(raw any) bool {
	value, _ := raw.(bool)
	return value
}

func containsValue(list any, value any) bool {
	items, ok := list.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if compareAny(item, value) == 0 {
			return true
		}
	}
	return false
}

func arrayContains(items []any, expected any) bool {
	for _, item := range items {
		if expectedObj, ok := expected.(map[string]any); ok {
			itemObj, _ := item.(map[string]any)
			if objectContains(itemObj, expectedObj) {
				return true
			}
			continue
		}
		if compareAny(item, expected) == 0 {
			return true
		}
	}
	return false
}

func arrayAny(items []any, expected any) bool {
	filter, ok := expected.(map[string]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if itemObj, ok := item.(map[string]any); ok {
			if evalNamespace(itemObj, filter) {
				return true
			}
			continue
		}
		if evalFilter(item, true, filter) {
			return true
		}
	}
	return false
}

func arrayAll(items []any, expected any) bool {
	filter, ok := expected.(map[string]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if itemObj, ok := item.(map[string]any); ok {
			if !evalNamespace(itemObj, filter) {
				return false
			}
			continue
		}
		if !evalFilter(item, true, filter) {
			return false
		}
	}
	return true
}

func objectContains(item map[string]any, expected map[string]any) bool {
	for key, value := range expected {
		actual, exists := item[key]
		if !exists || compareAny(actual, value) != 0 {
			return false
		}
	}
	return true
}

func compareByOp(compared int, op string) bool {
	switch op {
	case "gt":
		return compared > 0
	case "gte":
		return compared >= 0
	case "lt":
		return compared < 0
	case "lte":
		return compared <= 0
	default:
		return false
	}
}

func compareAny(left any, right any) int {
	leftNum, leftNumOK := number(left)
	rightNum, rightNumOK := number(right)
	if leftNumOK && rightNumOK {
		switch {
		case leftNum < rightNum:
			return -1
		case leftNum > rightNum:
			return 1
		default:
			return 0
		}
	}
	leftString := fmt.Sprintf("%v", left)
	rightString := fmt.Sprintf("%v", right)
	switch {
	case leftString < rightString:
		return -1
	case leftString > rightString:
		return 1
	default:
		return 0
	}
}

func number(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	default:
		return 0, false
	}
}
