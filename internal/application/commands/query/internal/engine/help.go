package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/query/internal/model"
)

func BuildHelpText(index model.QuerySchemaIndex, schemaPath string, rawSchema string) string {
	selectors := sortedKeys(index.SelectorPaths)
	sortFields := sortedFieldKeys(index.SortFields)
	filterFields := sortedFieldKeys(index.FilterFields)

	sections := []string{
		"Command\n  query - read entities with structural filters and deterministic pagination.",
		"Syntax\n  spec-cli query [options]",
		"Options\n  --type <entity_type>          Repeatable early type filter.\n  --where-json <json>          Structural filter JSON.\n  --select <field>             Repeatable read-namespace projection selector.\n  --sort <field[:asc|desc]>    Repeatable sort term.\n  --limit <n>                  Page size, default 100, integer >= 0.\n  --offset <n>                 Page offset, default 0, integer >= 0.\n  --help, -h                   Print command help.",
		"Rules\n  Read namespace paths are shared by --select, --sort and where-json.field.\n  where-json logical nodes: {\"op\":\"and\",\"filters\":[...]}, {\"op\":\"or\",\"filters\":[...]}, {\"op\":\"not\",\"filter\":{...}}.\n  where-json leaf node: {\"field\":\"meta.status\",\"op\":\"eq\",\"value\":\"active\"}.\n  Leaf operators: eq, neq, in, not_in, exists, not_exists, gt, gte, lt, lte, contains.\n  exists/not_exists forbid value; all other operators require value; in/not_in require array value.\n  content.sections.<name> in where-json allows only contains, exists, not_exists and is lexical discovery only.\n  content.raw is not allowed in where-json.field (it remains available for --select and --sort).\n  Missing field semantics: exists=false, not_exists=true, all other operators=false.\n  Range operators (gt/gte/lt/lte) are allowed only for numbers and YYYY-MM-DD dates.",
		"Examples\n  spec-cli query --format json\n  spec-cli query --type feature --where-json '{\"field\":\"meta.status\",\"op\":\"eq\",\"value\":\"active\"}'\n  spec-cli query --type feature --where-json '{\"field\":\"content.sections.summary\",\"op\":\"contains\",\"value\":\"retry\"}'\n  spec-cli query --select type --select id --select meta.status --sort updated_date:desc --limit 50 --offset 0",
		"Schema\n  Effective path: " + schemaPath + "\n  Allowed selectors:\n    - " + strings.Join(selectors, "\n    - ") + "\n  Allowed sort fields:\n    - " + strings.Join(sortFields, "\n    - ") + "\n  Allowed filter fields:\n    - " + strings.Join(filterFields, "\n    - ") + "\n  Verbatim loaded schema:\n" + indentSchema(rawSchema),
	}

	return strings.Join(sections, "\n\n") + "\n"
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedFieldKeys(values map[string]model.SchemaFieldSpec) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func indentSchema(rawSchema string) string {
	trimmed := strings.TrimRight(rawSchema, "\n")
	if trimmed == "" {
		return "    <empty>"
	}
	lines := strings.Split(trimmed, "\n")
	for idx, line := range lines {
		lines[idx] = fmt.Sprintf("    %s", line)
	}
	return strings.Join(lines, "\n")
}
