package get

import "github.com/anatoly-tenenev/spec-cli/internal/application/help/helpmodel"

func HelpSpec() helpmodel.CommandSpec {
	return helpmodel.CommandSpec{
		Name:    "get",
		Summary: "read one entity by exact id",
		Syntaxes: []string{
			"spec-cli get [options] --id <entity_id>",
		},
		Options: []helpmodel.OptionSpec{
			{
				Name:             "--id",
				ValueSyntax:      "<entity_id>",
				TakesValue:       true,
				Required:         true,
				Repeatable:       false,
				SchemaDerived:    false,
				SchemaDerivation: "none",
				Description:      "Exact target id.",
			},
			{
				Name:             "--select",
				ValueSyntax:      "<field>",
				TakesValue:       true,
				Required:         false,
				Repeatable:       true,
				SchemaDerived:    true,
				SchemaDerivation: "read selectors from CLI read-namespace",
				Description:      "Read-namespace projection selector.",
			},
		},
		Rules: []string{
			"Read-namespace leaf paths: type, id, slug, revision, created_date, updated_date, meta.<name>, refs.<field>.type, refs.<field>.id, refs.<field>.slug, content.raw, content.sections.<name>.",
			"--select uses read-namespace leaf paths.",
			"--id addresses one entity by exact id and returns a single-entity read result.",
			"If --select is omitted, default projection is type, id, slug, revision and meta.",
			"If selected content.sections.<name> is absent, the field must be present with null value.",
			"Existing but invalid target data must not degrade into false ENTITY_NOT_FOUND when deterministic read is still possible.",
		},
		Examples: []string{
			"spec-cli get --id FEAT-8",
			"spec-cli get --id FEAT-8 --select id --select meta.status",
		},
	}
}
