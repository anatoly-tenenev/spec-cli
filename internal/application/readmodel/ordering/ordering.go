// Package ordering is the read layer's entry point for putting entities in
// order. It exists as a level of its own because sorting is used both while
// executing a plan and by callers that already hold entities.
package ordering

import (
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/ordering/internal/entitysort"
)

func SortEntities(entities []model.EntityView, terms []model.SortTerm) {
	entitysort.SortEntities(entities, terms)
}
