// Package schemaload compiles the workspace schema for a command and describes
// the outcome in the form the response carries.
//
// Every command that needs a schema asks the same two things - compile it, and
// report what came of it - so the schema block a caller reads is the same block
// whichever command produced it, and a schema that fails to compile fails the
// same way everywhere.
//
// What a command does about a schema that did not compile stays in the command:
// most refuse to go on, `schema` reports the outcome as its answer, and
// `validate` wraps it in its own response shape. That is a difference in what
// the command is for, not in how the schema is read.
package schemaload

import (
	schemacompile "github.com/anatoly-tenenev/spec-cli/internal/application/schema/compile"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	outputpayload "github.com/anatoly-tenenev/spec-cli/internal/output/payload"
)

// Loader holds the seam through which a command reaches the compiler. It is a
// value rather than a package-level function so the compiler can be substituted
// in one place if a test ever needs to.
type Loader struct {
	newCompiler func() *schemacompile.Compiler
}

func NewLoader() Loader {
	return Loader{newCompiler: schemacompile.NewCompiler}
}

// Compile returns the compiled schema, the block describing it, and the failure
// if it did not compile.
//
// The block is returned even on failure, and that is the point: a caller facing
// a broken schema needs the issues, not an empty answer. displayPath is the
// path as the caller asked for it, so diagnostics name the file the caller
// knows rather than the resolved one.
func (l Loader) Compile(
	schemaPath string,
	displayPath string,
) (schemacompile.Result, map[string]any, *domainerrors.AppError) {
	newCompiler := l.newCompiler
	if newCompiler == nil {
		newCompiler = schemacompile.NewCompiler
	}

	result, compileErr := newCompiler().Compile(schemaPath, displayPath)
	return result, outputpayload.BuildSchemaPayload(result), compileErr
}
