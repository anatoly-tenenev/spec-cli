package schema

import "github.com/anatoly-tenenev/spec-cli/internal/application/help/helpmodel"

func HelpSpec() helpmodel.CommandSpec {
	return helpmodel.CommandSpec{
		Name:    "schema",
		Summary: "inspect schema compile diagnostics",
		Syntaxes: []string{
			"spec-cli schema check",
		},
		OperationModel: []string{
			"Validates the schema language and returns compile diagnostics only.",
			"Does not inspect workspace entities.",
		},
		Positionals: []helpmodel.PositionalSpec{
			{
				Name:        "<subcommand>",
				Required:    true,
				Repeatable:  false,
				Description: "Schema subcommand. Supported value: check.",
			},
		},
		Rules: []string{
			"Currently supports only the `check` subcommand.",
			"Runs shared schema compiler without workspace scan.",
			"Returns top-level schema diagnostics in schema.valid/schema.summary/schema.issues.",
		},
		Examples: []string{
			"spec-cli schema check",
		},
	}
}
