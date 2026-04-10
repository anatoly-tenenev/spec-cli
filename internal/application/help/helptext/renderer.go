package helptext

import (
	"sort"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/help/helpmodel"
	"github.com/anatoly-tenenev/spec-cli/internal/application/help/helpschema"
)

type SchemaView struct {
	WorkspacePath  string
	ResolvedPath   string
	Status         string
	ShowProjection bool
	Loaded         *helpschema.LoadedData
	ReasonCode     string
	Impact         string
	RecoveryClass  string
	RetryCommand   string
}

func (s SchemaView) IsLoaded() bool {
	return s.Status == "loaded" && s.Loaded != nil
}

func RenderGeneral(
	cliDescription string,
	globalOptions []helpmodel.GlobalOptionSpec,
	commands []helpmodel.CommandSpec,
	schema SchemaView,
) string {
	resolvedCommands := resolveCommands(commands, schema)

	sections := []string{
		renderCLISection(cliDescription),
		renderSchemaSection(schema),
		renderExecutionModelSection(),
		renderSpecificationModelSection(),
		renderProjectionConventionsSection(),
	}
	if schema.IsLoaded() {
		sections = append(sections, renderSpecificationProjectionSection(schema))
	}
	sections = append(sections,
		renderReferenceValueModelSection(),
		renderGlobalOptionsSection(globalOptions),
		renderCommandsSection(resolvedCommands),
		renderCommandDetailsSection(resolvedCommands),
	)
	return strings.Join(sections, "\n\n") + "\n"
}

func RenderCommand(command helpmodel.CommandSpec, schema SchemaView) string {
	resolved := resolveCommand(command, schema)
	rules := append([]string{}, resolved.Rules...)
	if !schema.IsLoaded() {
		rules = append(rules, schemaUnavailableRule(resolved, schema))
	}

	sections := []string{
		renderSingleCommandSection(resolved),
		renderSchemaSection(schema),
		renderSyntaxSection(resolved.Syntaxes),
	}
	if len(resolved.OperationModel) > 0 {
		sections = append(sections, renderOperationModelSection(resolved.OperationModel))
	}
	if schema.IsLoaded() && schema.ShowProjection {
		sections = append(sections, renderSpecificationProjectionSection(schema))
	}
	sections = append(sections, renderDetailSections(resolved.DetailSections)...)
	sections = append(sections, renderOptionsSection(resolved.Positionals, resolved.Options))
	if len(rules) > 0 {
		sections = append(sections, renderRulesSection(rules))
	}
	sections = append(sections, renderExamplesSection(resolved.Examples))
	return strings.Join(sections, "\n\n") + "\n"
}

func resolveCommands(commands []helpmodel.CommandSpec, schema SchemaView) []helpmodel.CommandSpec {
	resolved := make([]helpmodel.CommandSpec, 0, len(commands))
	for _, command := range commands {
		resolved = append(resolved, resolveCommand(command, schema))
	}
	return resolved
}

func resolveCommand(command helpmodel.CommandSpec, schema SchemaView) helpmodel.CommandSpec {
	resolved := command
	resolved.Examples = resolveExamples(command, schema)
	return resolved
}

func resolveExamples(command helpmodel.CommandSpec, schema SchemaView) []string {
	if !schema.IsLoaded() {
		return append([]string(nil), command.Examples...)
	}

	resolved := buildSchemaDerivedExamples(command.Name, schema.Loaded)
	if len(resolved) == 0 {
		return append([]string(nil), command.Examples...)
	}
	return resolved
}

func buildSchemaDerivedExamples(commandName string, loaded *helpschema.LoadedData) []string {
	if loaded == nil {
		return nil
	}

	switch commandName {
	case "schema":
		return []string{"spec-cli schema check"}
	case "validate":
		return buildValidateExamples(loaded)
	case "query":
		return buildQueryExamples(loaded)
	case "get":
		return buildGetExamples(loaded)
	case "add":
		return buildAddExamples(loaded)
	case "update":
		return buildUpdateExamples(loaded)
	case "delete":
		return buildDeleteExamples(loaded)
	case "help":
		return []string{
			"spec-cli help",
			"spec-cli help query",
			"spec-cli help query --show-schema-projection",
		}
	case "version":
		return []string{"spec-cli version"}
	default:
		return nil
	}
}

func buildValidateExamples(loaded *helpschema.LoadedData) []string {
	_ = loaded
	return []string{
		"spec-cli validate",
		"spec-cli validate --type <entity_type_1> --type <entity_type_2>",
		"spec-cli validate --fail-fast",
		"spec-cli validate --warnings-as-errors",
	}
}

func buildQueryExamples(loaded *helpschema.LoadedData) []string {
	_ = loaded
	return []string{
		"spec-cli query --type <entity_type> --select id --select meta.<meta_field>",
		"spec-cli query --select refs.<ref_field> --select content.sections.<section_name>",
		"spec-cli query --where \"meta.<meta_scalar_field> == '<string_value>'\"",
		"spec-cli query --where \"refs.<scalar_ref_field>.id == '<entity_id>'\"",
		"spec-cli query --where \"contains(content.sections.<section_name> || '', '<substring>')\"",
		"spec-cli query --where \"length(refs.<array_ref_field>[?reason == '<reason_value>']) > `0`\"",
		"spec-cli query --sort updatedDate:desc --limit 50 --offset 0",
		"Available paths, value kinds, and ref cardinality depend on the effective schema.",
	}
}

func buildGetExamples(loaded *helpschema.LoadedData) []string {
	_ = loaded
	return []string{
		"spec-cli get --id <entity_id>",
		"spec-cli get --id <entity_id> --select id --select meta.<meta_field>",
	}
}

func buildAddExamples(loaded *helpschema.LoadedData) []string {
	_ = loaded
	return []string{
		"spec-cli add --type <entity_type> --slug <slug> --set meta.<meta_scalar_field>=<string_value>",
		"spec-cli add --type <entity_type> --slug <slug> --set refs.<scalar_ref_field>=<entity_id>",
		"spec-cli add --type <entity_type> --slug <slug> --set refs.<array_ref_field>='[<entity_id_1>, <entity_id_2>]'",
		"spec-cli add --type <entity_type> --slug <slug> --content-file ./input/body.md --set meta.<meta_scalar_field>=<string_value> --set refs.<scalar_ref_field>=<entity_id>",
		"spec-cli add --type <entity_type> --slug <slug> --set-file content.sections.<section_name>=./input/section.md",
		"spec-cli add --type <entity_type> --slug <slug> --content-file ./input/body.md",
	}
}

func buildUpdateExamples(loaded *helpschema.LoadedData) []string {
	_ = loaded
	return []string{
		"spec-cli update --id <entity_id> --set meta.<meta_scalar_field>=<string_value>",
		"spec-cli update --id <entity_id> --set refs.<scalar_ref_field>=<entity_id>",
		"spec-cli update --id <entity_id> --unset refs.<ref_field>",
		"spec-cli update --id <entity_id> --set refs.<array_ref_field>='[<entity_id_1>, <entity_id_2>]'",
		"spec-cli update --id <entity_id> --content-file ./input/body.md --set meta.<meta_scalar_field>=<string_value> --set refs.<scalar_ref_field>=<entity_id>",
		"spec-cli update --id <entity_id> --set-file content.sections.<section_name>=./input/section.md",
		"spec-cli update --id <entity_id> --content-file ./input/body.md --expect-revision <token>",
	}
}

func buildDeleteExamples(loaded *helpschema.LoadedData) []string {
	_ = loaded
	return []string{
		"spec-cli delete --id <entity_id>",
		"spec-cli delete --id <entity_id> --expect-revision <token>",
		"spec-cli delete --id <entity_id> --dry-run",
	}
}

func dedupeExamples(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func renderCLISection(description string) string {
	return "CLI\n  " + description
}

func renderExecutionModelSection() string {
	lines := []string{
		"Execution model",
		"  `schema check` validates the schema language and reports compile diagnostics only.",
		"  `validate` validates workspace entities against the effective schema in one deterministic run.",
		"  `query`, `get`, `add`, `update`, and `delete` use schema-derived command models.",
		"  `help` remains tolerant; when schema-derived values are unavailable, do not infer them heuristically.",
	}
	return strings.Join(lines, "\n")
}

func renderSpecificationModelSection() string {
	lines := []string{
		"Specification model",
		"  A specification is a collection of typed entities.",
		"  Each entity is a logical JSON-like document with built-ins, meta, refs, and content.sections.",
		"  Storage details such as filesystem paths, markdown layout, frontmatter, and headings are internal details and are not part of the primary user model.",
	}
	return strings.Join(lines, "\n")
}

func renderProjectionConventionsSection() string {
	lines := []string{
		"Projection conventions",
		"  The specification projection uses JSON Schema-like structure plus spec-cli extensions.",
		"  Standard JSON Schema keywords keep their usual meaning.",
		"  spec-cli extensions:",
		"    - x-presence: required | optional | conditional",
		"    - x-requiredWhen: condition for conditional presence, evaluated over the logical entity document model",
		"    - x-kind: logical value kind not expressed by base JSON Schema",
		"    - x-refTypes: allowed target entity types for entityRef values",
		"    - x-const: dynamic const as {\"kind\":\"interpolation\",\"source\":\"<original-source>\"}",
		"    - x-enum: dynamic or mixed enum as ordered [{\"kind\":\"literal\",\"value\":...}|{\"kind\":\"interpolation\",\"source\":\"<original-source>\"}]",
		"  Native const/enum are emitted only for fully static literal constraints.",
		"  content.sections.<section_name>.title is canonical scalar heading title when specified in schema.",
		"  x-kind=entityRef describes the schema-level meaning of a reference field.",
		"  Read-side ref objects exposed by query/get are described separately in Reference value model.",
	}
	return strings.Join(lines, "\n")
}

func renderSpecificationProjectionSection(schema SchemaView) string {
	lines := []string{"Specification projection"}
	projection := strings.TrimSpace(schema.Loaded.ProjectionJSON)
	for _, line := range strings.Split(projection, "\n") {
		lines = append(lines, "  "+line)
	}
	return strings.Join(lines, "\n")
}

func renderReferenceValueModelSection() string {
	lines := []string{
		"Reference value model",
		"  Schema-level meaning:",
		"    - entityRef is a logical link to another entity",
		"    - scalar/array ref cardinality is encoded by projection shape (x-kind on scalar refs; type=array + items.x-kind on array refs)",
		"    - x-refTypes and x-presence define allowed target types and presence",
		"  Read-side projection:",
		"    - query/get expose scalar refs as {id, type, slug, resolved, reason}",
		"    - query/get expose array refs as arrays of the same ref objects",
		"  Write-side form:",
		"    - add/update accept target ids for refs.<ref_field>",
		"    - array ref fields accept arrays of target ids",
	}
	return strings.Join(lines, "\n")
}

func renderGlobalOptionsSection(options []helpmodel.GlobalOptionSpec) string {
	lines := []string{"Global options"}
	for _, option := range options {
		name := option.Name
		if option.ValueSyntax != "" {
			name += " " + option.ValueSyntax
		}
		lines = append(lines, "  "+name+": "+option.Description)
	}
	return strings.Join(lines, "\n")
}

func renderCommandsSection(commands []helpmodel.CommandSpec) string {
	lines := []string{"Commands"}
	for _, command := range commands {
		lines = append(lines, "  "+command.Name+": "+command.Summary)
	}
	return strings.Join(lines, "\n")
}

func renderSchemaSection(schema SchemaView) string {
	lines := []string{
		"Schema",
		"  Workspace: " + fieldOrDefault(schema.WorkspacePath, "."),
		"  ResolvedPath: " + fieldOrDefault(schema.ResolvedPath, "none"),
		"  Status: " + fieldOrDefault(schema.Status, "error"),
	}
	if schema.IsLoaded() {
		return strings.Join(lines, "\n")
	}

	lines = append(lines, "  ReasonCode: "+fieldOrDefault(schema.ReasonCode, "none"))
	lines = append(lines, "  Impact: "+fieldOrDefault(schema.Impact, "none"))
	lines = append(lines, "  RecoveryClass: "+fieldOrDefault(schema.RecoveryClass, "none"))
	lines = append(lines, "  RetryCommand: "+fieldOrDefault(schema.RetryCommand, "none"))
	return strings.Join(lines, "\n")
}

func renderCommandDetailsSection(commands []helpmodel.CommandSpec) string {
	blocks := []string{"Command details"}
	for idx, command := range commands {
		commandBlocks := []string{command.Name}
		commandBlocks = append(commandBlocks, indentBlock(renderSyntaxSection(command.Syntaxes), 2))
		if len(command.OperationModel) > 0 {
			commandBlocks = append(commandBlocks, indentBlock(renderOperationModelSection(command.OperationModel), 2))
		}
		for _, section := range renderDetailSections(command.DetailSections) {
			commandBlocks = append(commandBlocks, indentBlock(section, 2))
		}
		commandBlocks = append(commandBlocks, indentBlock(renderOptionsSection(command.Positionals, command.Options), 2))
		if len(command.Rules) > 0 {
			commandBlocks = append(commandBlocks, indentBlock(renderRulesSection(command.Rules), 2))
		}
		commandBlocks = append(commandBlocks, indentBlock(renderExamplesSection(command.Examples), 2))
		if idx > 0 {
			blocks = append(blocks, "")
		}
		blocks = append(blocks, strings.Join(commandBlocks, "\n"))
	}
	return strings.Join(blocks, "\n")
}

func renderCommandDetailsSectionLegacy(commands []helpmodel.CommandSpec) string {
	blocks := []string{"Command details"}
	for idx, command := range commands {
		commandBlocks := []string{
			command.Name,
			indentBlock(renderSyntaxSection(command.Syntaxes), 2),
			indentBlock(renderOptionsSection(command.Positionals, command.Options), 2),
		}
		if len(command.Rules) > 0 {
			commandBlocks = append(commandBlocks, indentBlock(renderRulesSection(command.Rules), 2))
		}
		commandBlocks = append(commandBlocks, indentBlock(renderExamplesSection(command.Examples), 2))
		if idx > 0 {
			blocks = append(blocks, "")
		}
		blocks = append(blocks, strings.Join(commandBlocks, "\n"))
	}
	return strings.Join(blocks, "\n")
}

func renderSingleCommandSection(command helpmodel.CommandSpec) string {
	return "Command\n  " + command.Name + ": " + command.Summary
}

func renderSyntaxSection(syntaxes []string) string {
	lines := []string{"Syntax"}
	for _, syntax := range syntaxes {
		lines = append(lines, "  "+syntax)
	}
	return strings.Join(lines, "\n")
}

func renderOperationModelSection(linesIn []string) string {
	lines := []string{"Operation model"}
	for _, line := range linesIn {
		lines = append(lines, "  "+line)
	}
	return strings.Join(lines, "\n")
}

func renderDetailSections(sections []helpmodel.DetailSectionSpec) []string {
	rendered := make([]string, 0, len(sections))
	for _, section := range sections {
		lines := []string{section.Title}
		for _, line := range section.Lines {
			lines = append(lines, "  "+line)
		}
		rendered = append(rendered, strings.Join(lines, "\n"))
	}
	return rendered
}

func renderOptionsSection(positionals []helpmodel.PositionalSpec, options []helpmodel.OptionSpec) string {
	lines := []string{"Options"}
	if len(positionals) == 0 && len(options) == 0 {
		lines = append(lines, "  none")
		return strings.Join(lines, "\n")
	}

	for _, positional := range positionals {
		lines = append(lines, "  "+positional.Name)
		lines = append(lines, "    required: "+boolLiteral(positional.Required))
		lines = append(lines, "    repeatable: "+boolLiteral(positional.Repeatable))
		lines = append(lines, "    description: "+positional.Description)
	}

	for _, option := range options {
		header := option.Name
		if option.ValueSyntax != "" {
			header += " " + option.ValueSyntax
		}
		lines = append(lines, "  "+header)
		lines = append(lines, "    takes_value: "+boolLiteral(option.TakesValue))
		lines = append(lines, "    required: "+boolLiteral(option.Required))
		lines = append(lines, "    repeatable: "+boolLiteral(option.Repeatable))
		lines = append(lines, "    mutually_exclusive_groups: "+mutuallyExclusiveValue(option.MutuallyExclusiveGroups))
		lines = append(lines, "    schema_derived: "+boolLiteral(option.SchemaDerived))
		lines = append(lines, "    schema_derivation: "+schemaDerivationValue(option))
		lines = append(lines, "    description: "+option.Description)
	}

	return strings.Join(lines, "\n")
}

func renderRulesSection(rules []string) string {
	lines := []string{"Rules"}
	for _, rule := range rules {
		lines = append(lines, "  - "+rule)
	}
	return strings.Join(lines, "\n")
}

func renderExamplesSection(examples []string) string {
	lines := []string{"Examples (syntax-only)"}
	for _, example := range examples {
		lines = append(lines, "  "+example)
	}
	return strings.Join(lines, "\n")
}

func boolLiteral(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func mutuallyExclusiveValue(groups []string) string {
	if len(groups) == 0 {
		return "none"
	}
	copyGroups := append([]string(nil), groups...)
	sort.Strings(copyGroups)
	return strings.Join(copyGroups, ", ")
}

func schemaDerivationValue(option helpmodel.OptionSpec) string {
	if strings.TrimSpace(option.SchemaDerivation) == "" {
		return "none"
	}
	return option.SchemaDerivation
}

func schemaUnavailableRule(command helpmodel.CommandSpec, schema SchemaView) string {
	schemaDerived := schemaDerivedOptionNames(command.Options)
	if len(schemaDerived) == 0 {
		return "Schema status is " + schema.Status + "; schema-derived catalog values are unavailable and must not be inferred heuristically."
	}
	return "Schema status is " + schema.Status + "; options marked schema_derived (" + strings.Join(schemaDerived, ", ") + ") keep derivation rules, but concrete schema-derived values are intentionally not listed."
}

func schemaDerivedOptionNames(options []helpmodel.OptionSpec) []string {
	seen := make(map[string]struct{}, len(options))
	names := make([]string, 0, len(options))
	for _, option := range options {
		if !option.SchemaDerived {
			continue
		}
		if _, exists := seen[option.Name]; exists {
			continue
		}
		seen[option.Name] = struct{}{}
		names = append(names, option.Name)
	}
	return names
}

func fieldOrDefault(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func indentBlock(input string, spaces int) string {
	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(input, "\n")
	for idx, line := range lines {
		lines[idx] = prefix + line
	}
	return strings.Join(lines, "\n")
}
