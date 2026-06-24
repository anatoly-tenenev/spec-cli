package graphqlhelp

import "github.com/anatoly-tenenev/spec-cli/internal/application/help/helpmodel"

func HelpSpec() helpmodel.CommandSpec {
	return helpmodel.CommandSpec{
		Name:    "graphql-help",
		Summary: "Show GraphQL semantic catalog and generated SDL.",
		Syntaxes: []string{
			"spec-cli graphql-help",
			"spec-cli graphql-help --schema-only",
			"spec-cli graphql-help --schema-only --entity <entity>",
		},
		OperationModel: []string{
			"Shows the GraphQL discovery catalog by default.",
			"--schema-only renders generated SDL from the same projection used by graphql-query.",
			"--entity restricts SDL output to selected root entity fields.",
		},
		Options: []helpmodel.OptionSpec{
			{Name: "--schema-only", TakesValue: false, Description: "Render generated GraphQL SDL instead of the semantic catalog."},
			{Name: "--entity", ValueSyntax: "<entity>", TakesValue: true, Repeatable: true, SchemaDerived: true, SchemaDerivation: "entity type keys from the effective schema", Description: "Restrict --schema-only SDL to one entity root field."},
		},
		Rules: []string{
			"graphql-help returns text output when --format is omitted.",
			"Explicit --format json returns CAPABILITY_UNSUPPORTED.",
			"--entity is allowed only with --schema-only.",
		},
		Examples: []string{
			"spec-cli graphql-help",
			"spec-cli graphql-help --schema-only --entity <entity>",
		},
	}
}
