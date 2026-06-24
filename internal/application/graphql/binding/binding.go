package binding

import (
	"github.com/anatoly-tenenev/spec-cli/internal/application/graphql/binding/internal/execution"
	"github.com/anatoly-tenenev/spec-cli/internal/application/graphql/binding/internal/plan"
	gqlmodel "github.com/anatoly-tenenev/spec-cli/internal/application/graphql/model"
	readmodel "github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func Build(proj *gqlmodel.Projection, query string, variables map[string]any, operationName string) ([]gqlmodel.RootPlan, *domainerrors.AppError) {
	return plan.Build(proj, query, variables, operationName)
}

func Execute(roots []gqlmodel.RootPlan, entities []readmodel.EntityView) (map[string]any, *domainerrors.AppError) {
	return execution.Execute(roots, entities)
}
