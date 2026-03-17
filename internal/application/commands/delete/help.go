package delete

import "github.com/anatoly-tenenev/spec-cli/internal/application/help/helpmodel"

func HelpSpec() helpmodel.CommandSpec {
	return helpmodel.CommandSpec{
		Name:    "delete",
		Summary: "delete one entity by exact id",
		Syntaxes: []string{
			"spec-cli delete [options] --id <entity_id>",
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
				Name:             "--expect-revision",
				ValueSyntax:      "<token>",
				TakesValue:       true,
				Required:         false,
				Repeatable:       false,
				SchemaDerived:    false,
				SchemaDerivation: "none",
				Description:      "Optimistic concurrency token for exact-match guard.",
			},
			{
				Name:             "--dry-run",
				TakesValue:       false,
				Required:         false,
				Repeatable:       false,
				SchemaDerived:    false,
				SchemaDerivation: "none",
				Description:      "Run full checks without filesystem mutation.",
			},
		},
		Rules: []string{
			"Target lookup uses exact id match only.",
			"Deletion is blocked when incoming entity_ref references exist.",
			"Blocking checks include scalar entity_ref and array items with entity_ref type.",
			"--expect-revision is an optimistic concurrency guard; command fails on revision mismatch.",
		},
		Examples: []string{
			"spec-cli delete --id FEAT-8",
			"spec-cli delete --id FEAT-8 --expect-revision sha256:...",
			"spec-cli delete --id FEAT-8 --dry-run",
		},
	}
}
