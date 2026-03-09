package expressions

import (
	"fmt"
	"sort"
	"strings"
)

type Operator string

const (
	OpEq     Operator = "eq"
	OpEqSafe Operator = "eq?"
	OpIn     Operator = "in"
	OpInSafe Operator = "in?"
	OpAll    Operator = "all"
	OpAny    Operator = "any"
	OpNot    Operator = "not"
	OpExists Operator = "exists"
)

type ReferenceKind string

const (
	ReferenceMeta ReferenceKind = "meta"
	ReferenceRef  ReferenceKind = "ref"
)

type Reference struct {
	Kind  ReferenceKind
	Field string
	Part  string
	Raw   string
}

type Operand struct {
	Literal   any
	Reference *Reference
}

type Expression struct {
	Operator       Operator
	Operands       []Operand
	ListOperands   []Operand
	Subexpressions []*Expression
	Subexpression  *Expression
	ExistsRef      *Reference
}

type MetaFieldSpec struct {
	Type       string
	Comparable bool
	EntityRef  bool
}

type CompileContext struct {
	MetaFields map[string]MetaFieldSpec
}

type CompileIssue struct {
	Code        string
	Message     string
	Field       string
	StandardRef string
}

type StrictReferenceUsage struct {
	Operator  Operator
	Reference Reference
}

func Compile(raw any, path string, ctx CompileContext) (*Expression, []CompileIssue) {
	expression, issues := compileExpression(raw, path, ctx)
	if len(issues) > 0 {
		return nil, issues
	}
	return expression, nil
}

func compileExpression(raw any, path string, ctx CompileContext) (*Expression, []CompileIssue) {
	rawMap, ok := raw.(map[string]any)
	if !ok {
		return nil, []CompileIssue{newIssue(
			"schema.expression.invalid_operand_type",
			"expression must be an object with a single operator",
			path,
		)}
	}

	if len(rawMap) != 1 {
		keys := make([]string, 0, len(rawMap))
		for key := range rawMap {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return nil, []CompileIssue{newIssue(
			"schema.expression.invalid_arity",
			fmt.Sprintf("expression must contain exactly one operator, got: %s", strings.Join(keys, ", ")),
			path,
		)}
	}

	var (
		opName string
		opRaw  any
	)
	for key, value := range rawMap {
		opName = key
		opRaw = value
	}

	switch Operator(opName) {
	case OpEq, OpEqSafe:
		return compileBinaryComparison(Operator(opName), opRaw, path, ctx)
	case OpIn, OpInSafe:
		return compileMembership(Operator(opName), opRaw, path, ctx)
	case OpAll, OpAny:
		return compileLogicalList(Operator(opName), opRaw, path, ctx)
	case OpNot:
		return compileNot(opRaw, path, ctx)
	case OpExists:
		return compileExists(opRaw, path, ctx)
	default:
		return nil, []CompileIssue{newIssue(
			"schema.expression.invalid_operator",
			fmt.Sprintf("unsupported expression operator '%s'", opName),
			path,
		)}
	}
}

func compileBinaryComparison(operator Operator, raw any, path string, ctx CompileContext) (*Expression, []CompileIssue) {
	operandsRaw, ok := raw.([]any)
	if !ok {
		return nil, []CompileIssue{newIssue(
			"schema.expression.invalid_operand_type",
			fmt.Sprintf("operator '%s' expects a list", operator),
			path,
		)}
	}
	if len(operandsRaw) != 2 {
		return nil, []CompileIssue{newIssue(
			"schema.expression.invalid_arity",
			fmt.Sprintf("operator '%s' expects exactly 2 operands", operator),
			path,
		)}
	}

	left, leftIssues := compileComparableOperand(operandsRaw[0], fmt.Sprintf("%s.%s[0]", path, operator), ctx)
	right, rightIssues := compileComparableOperand(operandsRaw[1], fmt.Sprintf("%s.%s[1]", path, operator), ctx)
	issues := mergeIssues(leftIssues, rightIssues)
	if len(issues) > 0 {
		return nil, issues
	}

	return &Expression{Operator: operator, Operands: []Operand{left, right}}, nil
}

func compileMembership(operator Operator, raw any, path string, ctx CompileContext) (*Expression, []CompileIssue) {
	outer, ok := raw.([]any)
	if !ok {
		return nil, []CompileIssue{newIssue(
			"schema.expression.invalid_operand_type",
			fmt.Sprintf("operator '%s' expects a list", operator),
			path,
		)}
	}
	if len(outer) != 2 {
		return nil, []CompileIssue{newIssue(
			"schema.expression.invalid_arity",
			fmt.Sprintf("operator '%s' expects exactly 2 operands", operator),
			path,
		)}
	}

	needle, needleIssues := compileComparableOperand(outer[0], fmt.Sprintf("%s.%s[0]", path, operator), ctx)
	issues := append([]CompileIssue{}, needleIssues...)

	haystackRaw, ok := outer[1].([]any)
	if !ok {
		issues = append(issues, newIssue(
			"schema.expression.invalid_operand_type",
			fmt.Sprintf("operator '%s' expects the second operand to be a list", operator),
			fmt.Sprintf("%s.%s[1]", path, operator),
		))
		return nil, issues
	}
	if len(haystackRaw) == 0 {
		issues = append(issues, newIssue(
			"schema.expression.invalid_arity",
			fmt.Sprintf("operator '%s' expects a non-empty list of values", operator),
			fmt.Sprintf("%s.%s[1]", path, operator),
		))
		return nil, issues
	}

	haystack := make([]Operand, 0, len(haystackRaw))
	for idx, item := range haystackRaw {
		operand, operandIssues := compileComparableOperand(item, fmt.Sprintf("%s.%s[1][%d]", path, operator, idx), ctx)
		if len(operandIssues) > 0 {
			issues = append(issues, operandIssues...)
			continue
		}
		haystack = append(haystack, operand)
	}

	if len(issues) > 0 {
		return nil, issues
	}

	return &Expression{Operator: operator, Operands: []Operand{needle}, ListOperands: haystack}, nil
}

func compileLogicalList(operator Operator, raw any, path string, ctx CompileContext) (*Expression, []CompileIssue) {
	rawList, ok := raw.([]any)
	if !ok {
		return nil, []CompileIssue{newIssue(
			"schema.expression.invalid_operand_type",
			fmt.Sprintf("operator '%s' expects a list of expressions", operator),
			path,
		)}
	}
	if len(rawList) == 0 {
		return nil, []CompileIssue{newIssue(
			"schema.expression.invalid_arity",
			fmt.Sprintf("operator '%s' expects a non-empty list", operator),
			path,
		)}
	}

	subexpressions := make([]*Expression, 0, len(rawList))
	issues := make([]CompileIssue, 0)
	for idx, item := range rawList {
		sub, subIssues := compileExpression(item, fmt.Sprintf("%s.%s[%d]", path, operator, idx), ctx)
		if len(subIssues) > 0 {
			issues = append(issues, subIssues...)
			continue
		}
		subexpressions = append(subexpressions, sub)
	}

	if len(issues) > 0 {
		return nil, issues
	}

	return &Expression{Operator: operator, Subexpressions: subexpressions}, nil
}

func compileNot(raw any, path string, ctx CompileContext) (*Expression, []CompileIssue) {
	sub, issues := compileExpression(raw, fmt.Sprintf("%s.%s", path, OpNot), ctx)
	if len(issues) > 0 {
		return nil, issues
	}
	return &Expression{Operator: OpNot, Subexpression: sub}, nil
}

func compileExists(raw any, path string, ctx CompileContext) (*Expression, []CompileIssue) {
	ref, issues := compileReferenceOperand(raw, fmt.Sprintf("%s.%s", path, OpExists), ctx, false)
	if len(issues) > 0 {
		return nil, issues
	}
	return &Expression{Operator: OpExists, ExistsRef: ref}, nil
}

func compileComparableOperand(raw any, path string, ctx CompileContext) (Operand, []CompileIssue) {
	if text, ok := raw.(string); ok && seemsReference(text) {
		ref, issues := compileReferenceOperand(raw, path, ctx, true)
		if len(issues) > 0 {
			return Operand{}, issues
		}
		return Operand{Reference: ref}, nil
	}

	if !isScalarLiteral(raw) {
		return Operand{}, []CompileIssue{newIssue(
			"schema.expression.invalid_operand_type",
			"operand must be a scalar literal or a context reference",
			path,
		)}
	}

	return Operand{Literal: raw}, nil
}

func compileReferenceOperand(raw any, path string, ctx CompileContext, requireComparable bool) (*Reference, []CompileIssue) {
	text, ok := raw.(string)
	if !ok {
		return nil, []CompileIssue{newIssue(
			"schema.expression.invalid_operand_type",
			"operand must be a context reference string",
			path,
		)}
	}

	if strings.HasPrefix(text, "meta.") {
		field := strings.TrimPrefix(text, "meta.")
		if field == "" {
			return nil, []CompileIssue{newIssue(
				"schema.expression.invalid_reference",
				"meta reference must include a field name",
				path,
			)}
		}

		spec, exists := ctx.MetaFields[field]
		if !exists {
			return nil, []CompileIssue{newIssue(
				"schema.expression.invalid_reference",
				fmt.Sprintf("unknown meta field '%s'", field),
				path,
			)}
		}
		if requireComparable && !spec.Comparable {
			return nil, []CompileIssue{newIssue(
				"schema.expression.invalid_reference",
				fmt.Sprintf("meta field '%s' cannot be used in comparison operators", field),
				path,
			)}
		}

		return &Reference{Kind: ReferenceMeta, Field: field, Raw: text}, nil
	}

	if strings.HasPrefix(text, "ref.") {
		parts := strings.Split(text, ".")
		if len(parts) != 3 {
			return nil, []CompileIssue{newIssue(
				"schema.expression.invalid_reference",
				"ref reference must have format ref.<field>.<part>",
				path,
			)}
		}

		field := parts[1]
		part := parts[2]
		if field == "" {
			return nil, []CompileIssue{newIssue(
				"schema.expression.invalid_reference",
				"ref reference must include a field name",
				path,
			)}
		}

		spec, exists := ctx.MetaFields[field]
		if !exists || !spec.EntityRef {
			return nil, []CompileIssue{newIssue(
				"schema.expression.invalid_reference",
				fmt.Sprintf("ref field '%s' is not declared as entity_ref", field),
				path,
			)}
		}

		switch part {
		case "id", "type", "slug", "dir_path":
		default:
			return nil, []CompileIssue{newIssue(
				"schema.expression.invalid_reference",
				fmt.Sprintf("unsupported ref part '%s'", part),
				path,
			)}
		}

		return &Reference{Kind: ReferenceRef, Field: field, Part: part, Raw: text}, nil
	}

	return nil, []CompileIssue{newIssue(
		"schema.expression.invalid_reference",
		"reference must use meta.<field> or ref.<field>.<part> syntax",
		path,
	)}
}

func seemsReference(value string) bool {
	return strings.HasPrefix(value, "meta.") || strings.HasPrefix(value, "ref.")
}

func isScalarLiteral(value any) bool {
	if value == nil {
		return true
	}

	switch value.(type) {
	case string, bool:
		return true
	case int, int8, int16, int32, int64:
		return true
	case uint, uint8, uint16, uint32, uint64:
		return true
	case float32, float64:
		return true
	default:
		return false
	}
}

func mergeIssues(left []CompileIssue, right []CompileIssue) []CompileIssue {
	issues := make([]CompileIssue, 0, len(left)+len(right))
	issues = append(issues, left...)
	issues = append(issues, right...)
	return issues
}

func CollectStrictReferenceUsages(expression *Expression) []StrictReferenceUsage {
	if expression == nil {
		return nil
	}

	usages := make([]StrictReferenceUsage, 0)
	switch expression.Operator {
	case OpEq, OpIn:
		usages = append(usages, collectStrictReferenceUsagesFromOperands(expression.Operator, expression.Operands)...)
		usages = append(usages, collectStrictReferenceUsagesFromOperands(expression.Operator, expression.ListOperands)...)
	case OpAll, OpAny:
		for _, subexpression := range expression.Subexpressions {
			usages = append(usages, CollectStrictReferenceUsages(subexpression)...)
		}
	case OpNot:
		usages = append(usages, CollectStrictReferenceUsages(expression.Subexpression)...)
	}
	return usages
}

func collectStrictReferenceUsagesFromOperands(operator Operator, operands []Operand) []StrictReferenceUsage {
	usages := make([]StrictReferenceUsage, 0)
	for _, operand := range operands {
		if operand.Reference == nil {
			continue
		}
		usages = append(usages, StrictReferenceUsage{
			Operator:  operator,
			Reference: *operand.Reference,
		})
	}
	return usages
}

func newIssue(code string, message string, field string) CompileIssue {
	return CompileIssue{
		Code:        code,
		Message:     message,
		Field:       field,
		StandardRef: "11.6",
	}
}
