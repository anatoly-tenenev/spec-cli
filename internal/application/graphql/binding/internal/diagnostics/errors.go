package diagnostics

import (
	readmodel "github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func GraphQLDetails(phase string, pos *ast.Position) map[string]any {
	graphql := map[string]any{"phase": phase}
	if pos != nil {
		graphql["locations"] = []map[string]any{{"line": pos.Line, "column": pos.Column}}
	}
	return map[string]any{"graphql": graphql}
}

func InvalidResult(message string, field string, entity readmodel.EntityView) *domainerrors.AppError {
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

func InvalidQuery(message string, phase string, errs gqlerror.List) *domainerrors.AppError {
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
