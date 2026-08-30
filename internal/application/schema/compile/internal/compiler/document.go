// Package compiler is the seam between loading a schema document and giving it
// meaning. Today it holds a single semantic pass; it exists as its own level
// because the planned raw/semantic split (doc/SCHEMA_COMPILER_RAW_SEMANTIC_SPLIT.md)
// adds a normalization stage in front of that pass without changing what
// compile calls.
//
// This is a deliberate exception to "a directory level must earn its
// existence": today the entrypoint only forwards, and a sweep for redundant
// levels will find it. Collapsing it and expanding it again when the split
// lands costs more than the one extra hop. If the split is abandoned, this
// level goes with it.
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
