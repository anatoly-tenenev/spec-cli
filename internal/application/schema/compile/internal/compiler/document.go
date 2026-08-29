// Package compiler is the seam between loading a schema document and giving it
// meaning. Today it holds a single semantic pass; it exists as its own level
// because the planned raw/semantic split (doc/SCHEMA_COMPILER_RAW_SEMANTIC_SPLIT.md)
// adds a normalization stage in front of that pass without changing what
// compile calls.
package compiler

import (
	semantic "github.com/anatoly-tenenev/spec-cli/internal/application/schema/compile/internal/compiler/internal/semantic"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/diagnostics"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/source"
)

func CompileDocument(doc source.Document) (model.CompiledSchema, []diagnostics.Issue) {
	return semantic.CompileDocument(doc)
}
