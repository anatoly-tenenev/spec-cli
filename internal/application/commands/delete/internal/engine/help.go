package engine

import (
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/contracts/responses"
)

func HelpPayload() map[string]any {
	return map[string]any{
		"result_state": responses.ResultStateValid,
		"command":      "delete",
		"syntax":       "spec-cli delete --id <entity_id> [options]",
		"help":         buildHelpText(),
	}
}

func buildHelpText() string {
	sections := []string{
		"Command\n  delete - remove one entity by exact id.",
		"Syntax\n  spec-cli delete --id <entity_id> [options]",
		"Options\n  --id <entity_id>             Required exact target id.\n  --expect-revision <token>    Optional optimistic concurrency guard.\n  --dry-run                     Run full checks without filesystem mutation.\n  --help, -h                    Print command help.",
		"Rules\n  Target lookup uses exact id match only.\n  Deletion is blocked when incoming entity_ref references exist.\n  Blocking checks include scalar entity_ref and array items with entity_ref type.",
		"Examples\n  spec-cli delete --id FEAT-42\n  spec-cli delete --id FEAT-42 --expect-revision sha256:...\n  spec-cli delete --id FEAT-42 --dry-run",
	}

	return strings.Join(sections, "\n\n") + "\n"
}
