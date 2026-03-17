package helpglobal

import "github.com/anatoly-tenenev/spec-cli/internal/application/help/helpmodel"

func Options() []helpmodel.GlobalOptionSpec {
	return []helpmodel.GlobalOptionSpec{
		{
			Name:        "--workspace",
			ValueSyntax: "<path>",
			Description: `optional; default "."; root workspace filesystem path`,
		},
		{
			Name:        "--schema",
			ValueSyntax: "<path>",
			Description: `optional; default "spec.schema.yaml"; effective schema filesystem path`,
		},
		{
			Name:        "--format",
			ValueSyntax: "<json|text>",
			Description: `optional; default "json"; help supports only text output; explicit --format json for help returns CAPABILITY_UNSUPPORTED`,
		},
		{
			Name:        "--require-absolute-paths",
			Description: "optional; reject explicit relative filesystem paths",
		},
	}
}
