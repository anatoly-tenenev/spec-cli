// Package engine answers a read query in two steps that stay separate on
// purpose: BuildPlan validates the request against the schema and produces a
// plan, Execute runs that plan over already-loaded entities. Every reason to
// reject a query is therefore decided before a single document is touched, and
// query, get and graphql-query share one meaning for --select, --where and
// --sort.
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
