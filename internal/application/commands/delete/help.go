package delete

import "github.com/anatoly-tenenev/spec-cli/internal/application/help/helpmodel"

func HelpSpec() helpmodel.CommandSpec {
	return helpmodel.CommandSpec{
		Name:    "delete",
		Summary: "delete one entity subject to reference integrity",
		Syntaxes: []string{
			"spec-cli delete [options] --id <entity_id>",
		},
		OperationModel: []string{
			"delete removes one entity by exact id, subject to inbound reference integrity.",
		},
		DetailSections: []helpmodel.DetailSectionSpec{
			{
				Title: "Reference integrity model",
				Lines: []string{
					"delete is blocked when other entities still reference the target through entityRef fields.",
					"both scalar and array entityRef links participate in blocking checks.",
				},
			},
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
			"Deletion is blocked when incoming entityRef references exist.",
			"Blocking checks include scalar entityRef and array items with entityRef type.",
			"--expect-revision is an optimistic concurrency guard; command fails on revision mismatch.",
		},
		Examples: []string{
			"spec-cli delete --id <entity_id>",
			"spec-cli delete --id <entity_id> --expect-revision <token>",
			"spec-cli delete --id <entity_id> --dry-run",
		},
	}
}
