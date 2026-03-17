package help

import "github.com/anatoly-tenenev/spec-cli/internal/application/help/helpmodel"

func HelpSpec() helpmodel.CommandSpec {
	return helpmodel.CommandSpec{
		Name:    "help",
		Summary: "show deterministic text help for CLI and commands",
		Syntaxes: []string{
			"spec-cli help",
			"spec-cli help <command>",
		},
		Positionals: []helpmodel.PositionalSpec{
			{
				Name:        "<command>",
				Required:    false,
				Repeatable:  false,
				Description: "Optional command name for detailed help.",
			},
		},
		Rules: []string{
			"Returns stable text-first output.",
			"Renders exactly one Schema section per invocation.",
			"Schema always includes ResolvedPath and Status; when schema is unavailable it includes ReasonCode, Impact, RecoveryClass and RetryCommand.",
			"help --format json returns CAPABILITY_UNSUPPORTED.",
		},
		Examples: []string{
			"spec-cli help",
			"spec-cli help query",
		},
	}
}
