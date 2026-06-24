package engine

import (
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/engine/internal/execution"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/engine/internal/planning"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/model"
	schemacapread "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/read"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func BuildPlan(opts model.Options, capability schemacapread.Capability) (model.QueryPlan, *domainerrors.AppError) {
	return planning.BuildPlan(opts, capability)
}

func Execute(plan model.QueryPlan, entities []model.EntityView) (model.QueryResponse, *domainerrors.AppError) {
	return execution.Execute(plan, entities)
}
