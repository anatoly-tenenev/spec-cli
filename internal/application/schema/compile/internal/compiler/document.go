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
