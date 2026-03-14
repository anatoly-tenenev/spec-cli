package requiredconstraint

import (
	"fmt"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/expressions"
	expressioncontext "github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/schema/internal/entity/internal/expressioncontext"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	domainvalidation "github.com/anatoly-tenenev/spec-cli/internal/domain/validation"
)

func Parse(
	rawRule map[string]any,
	path string,
	compileContext expressions.CompileContext,
) (bool, bool, *expressions.Expression, []domainvalidation.Issue, *domainerrors.AppError) {
	required := true
	requiredWhen := false
	var requiredWhenExpr *expressions.Expression
	hasRequired := false
	hasRequiredWhen := false
	issues := make([]domainvalidation.Issue, 0)

	if rawRequired, exists := rawRule["required"]; exists {
		typed, ok := rawRequired.(bool)
		if !ok {
			return false, false, nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s.required must be boolean", path),
				nil,
			)
		}
		required = typed
		hasRequired = true
	}

	if rawRequiredWhen, exists := rawRule["required_when"]; exists {
		hasRequiredWhen = true
		switch typed := rawRequiredWhen.(type) {
		case bool:
			requiredWhen = typed
		case map[string]any:
			expression, compileIssues := expressions.Compile(typed, fmt.Sprintf("%s.required_when", path), compileContext)
			for _, compileIssue := range compileIssues {
				issues = append(issues, expressioncontext.FromCompileIssue(compileIssue))
			}
			if len(compileIssues) == 0 {
				requiredWhenExpr = expression
			}
		default:
			return false, false, nil, nil, domainerrors.New(
				domainerrors.CodeSchemaInvalid,
				fmt.Sprintf("%s.required_when must be boolean or expression object", path),
				nil,
			)
		}
	}

	if hasRequired && hasRequiredWhen && required {
		return false, false, nil, nil, domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("%s cannot set required=true together with required_when", path),
			nil,
		)
	}

	if !hasRequired && hasRequiredWhen {
		required = false
		issues = append(issues, domainvalidation.Issue{
			Code:        "schema.required_when.required_not_explicit",
			Level:       domainvalidation.LevelWarning,
			Class:       "SchemaError",
			Message:     "required_when is set without explicit required; effective required=false is applied",
			StandardRef: "11.5",
			Field:       fmt.Sprintf("%s.required", path),
		})
	}

	return required, requiredWhen, requiredWhenExpr, issues, nil
}
