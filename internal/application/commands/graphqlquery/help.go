package graphqlquery

import "github.com/anatoly-tenenev/spec-cli/internal/application/help/helpmodel"

func HelpSpec() helpmodel.CommandSpec {
	return helpmodel.CommandSpec{
		Name:    "graphql-query",
		Summary: "Execute read-only GraphQL queries.",
		Syntaxes: []string{
			"spec-cli graphql-query --query <graphql>",
			"spec-cli graphql-query --file <query.graphql>",
			"spec-cli graphql-query --file -",
		},
		OperationModel: []string{
			"Executes read-only GraphQL query operations against the effective schema read model.",
			"Use graphql-help --schema-only to get the exact generated SDL.",
			"Success output contains result_state and data only; schema is included only for schema compile errors.",
		},
		Options: []helpmodel.OptionSpec{
			{Name: "--query", ValueSyntax: "<graphql>", TakesValue: true, MutuallyExclusiveGroups: []string{"query-input"}, Description: "Inline GraphQL document."},
			{Name: "--file", ValueSyntax: "<path>|-", TakesValue: true, MutuallyExclusiveGroups: []string{"query-input"}, Description: "GraphQL document file path, or - for stdin."},
			{Name: "--variables-json", ValueSyntax: "<json>", TakesValue: true, MutuallyExclusiveGroups: []string{"variables-input"}, Description: "Inline JSON object with GraphQL variables."},
			{Name: "--variables-file", ValueSyntax: "<path>|-", TakesValue: true, MutuallyExclusiveGroups: []string{"variables-input"}, Description: "Variables JSON object file path, or - for stdin."},
			{Name: "--operation-name", ValueSyntax: "<name>", TakesValue: true, Description: "Operation name for documents with multiple operations."},
		},
		Rules: []string{
			"One of --query or --file is required.",
			"--query and --file are mutually exclusive.",
			"--variables-json and --variables-file are mutually exclusive.",
			"--file - and --variables-file - cannot both read stdin.",
			"Only query operations are supported.",
			"GraphQL introspection and custom query directives are not supported.",
		},
		Examples: []string{
			"spec-cli graphql-query --file <query.graphql>",
			"spec-cli graphql-query --file <query.graphql> --variables-json '{\"status\":\"active\"}' --operation-name <OperationName>",
		},
	}
}
