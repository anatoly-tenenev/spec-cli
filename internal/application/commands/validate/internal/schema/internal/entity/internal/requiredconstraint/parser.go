package requiredconstraint

import (
	"fmt"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/expressions"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func Parse(
	rawRule map[string]any,
	path string,
	engine *expressions.Engine,
) (bool, *expressions.CompiledExpression, *domainerrors.AppError) {
	required := true
	var requiredExpr *expressions.CompiledExpression

	rawRequired, exists := rawRule["required"]
	if !exists {
		return required, nil, nil
	}

	switch typed := rawRequired.(type) {
	case bool:
		required = typed
		return required, nil, nil
	case string:
		expression, compileErr := expressions.CompileScalarInterpolation(typed, engine)
		if compileErr != nil {
			return false, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s.required has invalid expression in required context: %s", path, compileErr.Message),
				map[string]any{
					"code":         compileErr.Code,
					"field":        path + ".required",
					"offset":       compileErr.Offset,
					"standard_ref": "11.5",
				},
			)
		}
		requiredExpr = expression
		required = false
		return required, requiredExpr, nil
	default:
		return false, nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("%s.required must be boolean or string interpolation ${expr}", path),
			nil,
		)
	}
}
