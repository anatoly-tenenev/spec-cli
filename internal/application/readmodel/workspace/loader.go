// Package workspace loads a whole workspace into the entity views the read
// engine runs on: scan, parse, resolve references, project. It is the read
// side's only door to the filesystem, so what a query can see is decided in
// one place.
package workspace

import (
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/workspace/internal/loading"
	schemacapread "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/read"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func LoadEntities(
	workspacePath string,
	capability schemacapread.Capability,
	typeFilters []string,
) ([]model.EntityView, *domainerrors.AppError) {
	return loading.LoadEntities(workspacePath, capability, typeFilters)
}
