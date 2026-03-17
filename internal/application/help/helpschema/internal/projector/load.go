package projector

import (
	"github.com/anatoly-tenenev/spec-cli/internal/application/help/helpschema/internal/projector/internal/pipeline"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

type Projection struct {
	EffectivePath  string
	ProjectionYAML string
}

func LoadProjection(schemaPath string, displayPath string) (Projection, *domainerrors.AppError) {
	projected, err := pipeline.LoadProjection(schemaPath, displayPath)
	if err != nil {
		return Projection{}, err
	}
	return Projection{
		EffectivePath:  projected.EffectivePath,
		ProjectionYAML: projected.ProjectionYAML,
	}, nil
}
