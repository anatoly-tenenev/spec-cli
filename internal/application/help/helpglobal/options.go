// Package helpglobal declares the global CLI options as help data. The
// descriptions here are what a caller is told the options mean, so they are
// kept next to each other rather than spread across the parser that reads
// them.
package helpglobal

import "github.com/anatoly-tenenev/spec-cli/internal/application/help/helpmodel"

func Options() []helpmodel.GlobalOptionSpec {
	return []helpmodel.GlobalOptionSpec{
		{
			Name:        "--workspace",
			ValueSyntax: "<path>",
			Description: `optional; default "."; root workspace path`,
		},
		{
			Name:        "--schema",
			ValueSyntax: "<path>",
			Description: `optional; default "spec.schema.yaml"; effective schema path`,
		},
		{
			Name:        "--config",
			ValueSyntax: "<path>",
			Description: `optional; JSON config with "schema"/"workspace"; if omitted, auto-loads "./spec-cli.json" when present`,
		},
		{
			Name:        "--format",
			ValueSyntax: "<json|text>",
			Description: `optional; default "json"; help and graphql-help support text output; explicit --format json for help/graphql-help returns CAPABILITY_UNSUPPORTED`,
		},
		{
			Name:        "--require-absolute-paths",
			Description: "optional; reject explicit relative filesystem paths",
		},
	}
}
