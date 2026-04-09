package help

import "github.com/anatoly-tenenev/spec-cli/internal/application/help/helpmodel"

func HelpSpec() helpmodel.CommandSpec {
	return helpmodel.CommandSpec{
		Name:    "help",
		Summary: "show deterministic text help for the CLI and its document model",
		Syntaxes: []string{
			"spec-cli help",
			"spec-cli help <command>",
			"spec-cli help <command> --show-schema-projection",
		},
		OperationModel: []string{
			"Shows deterministic text help for the CLI, the specification model, and command behavior.",
			"Renders one schema-derived overview for the whole invocation.",
			"For command help, --show-schema-projection adds the same Specification projection block used by general help.",
		},
		Positionals: []helpmodel.PositionalSpec{
			{
				Name:        "<command>",
				Required:    false,
				Repeatable:  false,
				Description: "Optional command name for detailed help.",
			},
		},
		Options: []helpmodel.OptionSpec{
			{
				Name:                    "--show-schema-projection",
				TakesValue:              false,
				Required:                false,
				Repeatable:              false,
				MutuallyExclusiveGroups: nil,
				SchemaDerived:           false,
				SchemaDerivation:        "",
				Description:             "Optional; for help <command>, include the schema-derived Specification projection block.",
			},
		},
		Rules: []string{
			"Returns stable text-first output.",
			"Renders exactly one schema-derived overview per invocation.",
			"Help is tolerant; when schema-derived values are unavailable, it reports that fact and does not infer them heuristically.",
			"help --show-schema-projection does not duplicate the projection block in general help.",
			"help --format json returns CAPABILITY_UNSUPPORTED.",
		},
		Examples: []string{
			"spec-cli help",
			"spec-cli help query",
			"spec-cli help query --show-schema-projection",
		},
	}
}
