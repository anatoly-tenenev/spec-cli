package validate

import "github.com/anatoly-tenenev/spec-cli/internal/application/help/helpmodel"

func HelpSpec() helpmodel.CommandSpec {
	return helpmodel.CommandSpec{
		Name:    "validate",
		Summary: "validate schema and workspace entities in one deterministic run",
		Syntaxes: []string{
			"spec-cli validate [options]",
		},
		OperationModel: []string{
			"validate runs deterministic checks over the effective schema and entity documents.",
			"--type restricts the active entity-type set for validation.",
		},
		DetailSections: []helpmodel.DetailSectionSpec{
			{
				Title: "Validation model",
				Lines: []string{
					"built-ins, meta fields, refs, content sections, and schema constraints are validated against the effective schema.",
				},
			},
		},
		Options: []helpmodel.OptionSpec{
			{
				Name:             "--type",
				ValueSyntax:      "<entity_type>",
				TakesValue:       true,
				Required:         false,
				Repeatable:       true,
				SchemaDerived:    true,
				SchemaDerivation: "entity type keys from the specification projection",
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
			"spec-cli validate --type <entity_type_1> --type <entity_type_2>",
			"spec-cli validate --fail-fast",
			"spec-cli validate --warnings-as-errors",
		},
	}
}
