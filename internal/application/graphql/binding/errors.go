// errors.go builds the GraphQL-shaped error details: the phase the
// failure belongs to and the position in the query text it points at. A caller
// writing GraphQL needs to know where in its own document the problem is, and
// that location only exists here, so every binding failure is constructed
// through this file rather than as a bare domain error.

package binding

import (
	readmodel "github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func graphqlDetails(phase string, pos *ast.Position) map[string]any {
	graphql := map[string]any{"phase": phase}
	if pos != nil {
		graphql["locations"] = []map[string]any{{"line": pos.Line, "column": pos.Column}}
	}
	return map[string]any{"graphql": graphql}
}

func invalidResult(message string, field string, entity readmodel.EntityView) *domainerrors.AppError {
	return domainerrors.New(
		domainerrors.CodeInvalidQueryResult,
		message,
		map[string]any{
			"graphql": map[string]any{
				"field": field,
				"entity": map[string]any{
					"type": entity.Type,
					"id":   entity.ID,
				},
			},
		},
	)
}

func invalidQuery(message string, phase string, errs gqlerror.List) *domainerrors.AppError {
	details := map[string]any{"graphql": map[string]any{"phase": phase}}
	if len(errs) > 0 && len(errs[0].Locations) > 0 {
		locations := make([]map[string]any, 0, len(errs[0].Locations))
		for _, loc := range errs[0].Locations {
			locations = append(locations, map[string]any{"line": loc.Line, "column": loc.Column})
		}
		details["graphql"].(map[string]any)["locations"] = locations
	}
	return domainerrors.New(domainerrors.CodeInvalidQuery, message, details)
}
