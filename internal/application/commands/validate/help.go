package validate

import "github.com/anatoly-tenenev/spec-cli/internal/application/help/helpmodel"

func HelpSpec() helpmodel.CommandSpec {
	return helpmodel.CommandSpec{
		Name:    "validate",
		Summary: "validate schema and workspace documents",
		Syntaxes: []string{
			"spec-cli validate [options]",
		},
		Options: []helpmodel.OptionSpec{
			{
				Name:             "--type",
				ValueSyntax:      "<entity_type>",
				TakesValue:       true,
				Required:         false,
				Repeatable:       true,
				SchemaDerived:    true,
				SchemaDerivation: "entity type keys from Schema.entity",
				Description:      "Restrict validation to selected entity types.",
			},
			{
				Name:             "--fail-fast",
				TakesValue:       false,
				Required:         false,
				Repeatable:       false,
				SchemaDerived:    false,
				SchemaDerivation: "none",
				Description:      "Stop checks after first blocking validation error when supported by rule.",
			},
			{
				Name:             "--warnings-as-errors",
				TakesValue:       false,
				Required:         false,
				Repeatable:       false,
				SchemaDerived:    false,
				SchemaDerivation: "none",
				Description:      "Treat warnings as non-zero exit status.",
			},
		},
		Rules: []string{
			"Validation always includes effective schema and workspace documents in one deterministic run.",
			"Type filters are validated against schema-derived entity type keys.",
		},
		Examples: []string{
			"spec-cli validate",
			"spec-cli validate --type feature --type service",
			"spec-cli validate --fail-fast",
			"spec-cli validate --warnings-as-errors",
		},
	}
}
