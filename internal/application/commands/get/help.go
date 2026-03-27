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
				SchemaDerivation: "selectors from CLI projection-namespace",
				Description:      "Projection-namespace selector.",
			},
		},
		Rules: []string{
			"Projection-namespace selectors: type, id, slug, revision, createdDate, updatedDate, meta.<name>, refs, refs.<field>, content.raw, content.sections, content.sections.<name>.",
			"--select uses projection-namespace selectors.",
			"--id addresses one entity by exact id and returns a single-entity read result.",
			"If --select is omitted, default projection is type, id, slug, meta and refs.",
			"entityRef scalar fields are exposed through refs.<field> and are not projected under meta.<name>.",
			"If selected content.sections.<name> is absent, the field must be present with null value.",
			"Existing but invalid target data must not degrade into false ENTITY_NOT_FOUND when deterministic read is still possible.",
		},
		Examples: []string{
			"spec-cli get --id FEAT-8",
			"spec-cli get --id FEAT-8 --select id --select meta.status",
		},
	}
}
