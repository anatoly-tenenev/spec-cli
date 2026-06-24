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
