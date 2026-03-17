package helptext

import (
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/help/helpmodel"
)

type SchemaView struct {
	ResolvedPath   string
	Status         string
	ProjectionYAML string
	ReasonCode     string
	Impact         string
	RecoveryClass  string
	RetryCommand   string
}

func (s SchemaView) IsLoaded() bool {
	return s.Status == "loaded"
}

func RenderGeneral(
	cliDescription string,
	globalOptions []helpmodel.GlobalOptionSpec,
	commands []helpmodel.CommandSpec,
	schema SchemaView,
) string {
	sections := []string{
		renderCLISection(cliDescription),
		renderGlobalOptionsSection(globalOptions),
		renderCommandsSection(commands),
		renderSchemaSection(schema),
		renderCommandDetailsSection(commands),
	}

	return strings.Join(sections, "\n\n") + "\n"
}

func RenderCommand(command helpmodel.CommandSpec, schema SchemaView) string {
	rules := append([]string{}, command.Rules...)
	if !schema.IsLoaded() {
		rules = append(rules, schemaUnavailableRule(command, schema))
	}

	sections := []string{
		renderSingleCommandSection(command),
		renderSyntaxSection(command.Syntaxes),
		renderSchemaSection(schema),
		renderOptionsSection(command.Positionals, command.Options),
		renderRulesSection(rules),
		renderExamplesSection(command.Examples),
	}

	return strings.Join(sections, "\n\n") + "\n"
}

func renderCLISection(description string) string {
	return "CLI\n  " + description
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
		"  ResolvedPath: " + fieldOrDefault(schema.ResolvedPath, "none"),
		"  Status: " + fieldOrDefault(schema.Status, "error"),
	}
	if schema.IsLoaded() {
		lines = append(lines, "  CLI-oriented projection:")
		for _, rawLine := range strings.Split(strings.TrimRight(schema.ProjectionYAML, "\n"), "\n") {
			lines = append(lines, "    "+rawLine)
		}
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
		commandBlock := []string{
			command.Name,
			indentBlock(renderSyntaxSection(command.Syntaxes), 2),
			indentBlock(renderOptionsSection(command.Positionals, command.Options), 2),
			indentBlock(renderRulesSection(command.Rules), 2),
			indentBlock(renderExamplesSection(command.Examples), 2),
		}
		if idx > 0 {
			blocks = append(blocks, "")
		}
		blocks = append(blocks, strings.Join(commandBlock, "\n"))
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
	lines := []string{"Examples"}
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
	return strings.Join(groups, ", ")
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
