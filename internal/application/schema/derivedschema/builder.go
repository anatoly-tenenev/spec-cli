package derivedschema

import "github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"

func LiteralMatchesKind(value any, kind model.ValueKind) bool {
	switch kind {
	case model.ValueKindUnknown:
		return true
	case model.ValueKindString, model.ValueKindEntityRef:
		_, ok := value.(string)
		return ok
	case model.ValueKindBoolean:
		_, ok := value.(bool)
		return ok
	case model.ValueKindNumber:
		return isNumeric(value)
	case model.ValueKindInteger:
		return isInteger(value)
	case model.ValueKindArray:
		_, ok := value.([]any)
		return ok
	default:
		return false
	}
}

func ProjectValueSpec(value model.ValueSpec) model.ValueSpec {
	projected := value
	projected.Const, projected.Enum = projectLiteralConstraints(value.Kind, value.Const, value.Enum)
	if value.Items != nil {
		projectedItems := ProjectValueSpec(*value.Items)
		projected.Items = &projectedItems
	}
	return projected
}

func ProjectMetaField(field model.MetaField) model.MetaField {
	projected := field
	projected.Value = ProjectValueSpec(field.Value)
	return projected
}

func StaticConstValue(constValue *model.Literal) (any, bool) {
	if constValue == nil || constValue.Template != nil {
		return nil, false
	}
	return constValue.Value, true
}

func StaticEnumValues(enumValues []model.Literal) ([]any, bool) {
	if len(enumValues) == 0 {
		return nil, false
	}

	staticValues := make([]any, 0, len(enumValues))
	for _, enumValue := range enumValues {
		if enumValue.Template != nil {
			return nil, false
		}
		staticValues = append(staticValues, enumValue.Value)
	}
	return staticValues, len(staticValues) > 0
}

func projectLiteralConstraints(
	kind model.ValueKind,
	constValue *model.Literal,
	enumValues []model.Literal,
) (*model.Literal, []model.Literal) {
	var projectedConst *model.Literal
	if constValue != nil && LiteralMatchesKind(constValue.Value, kind) {
		copiedConst := *constValue
		projectedConst = &copiedConst
	}

	var projectedEnum []model.Literal
	if len(enumValues) > 0 {
		projectedEnum = make([]model.Literal, 0, len(enumValues))
		for _, enumValue := range enumValues {
			if !LiteralMatchesKind(enumValue.Value, kind) {
				continue
			}
			projectedEnum = append(projectedEnum, enumValue)
		}
	}

	return projectedConst, projectedEnum
}

func isNumeric(value any) bool {
	switch value.(type) {
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

func isInteger(value any) bool {
	switch value.(type) {
	case int, int8, int16, int32, int64:
		return true
	case uint, uint8, uint16, uint32, uint64:
		return true
	default:
		return false
	}
}
