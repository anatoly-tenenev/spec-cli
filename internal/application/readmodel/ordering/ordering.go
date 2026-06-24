package ordering

import (
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/ordering/internal/entitysort"
)

func SortEntities(entities []model.EntityView, terms []model.SortTerm) {
	entitysort.SortEntities(entities, terms)
}
