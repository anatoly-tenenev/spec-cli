// Package requests defines the input side of the CLI wire contract: the
// command being invoked, the options shared by every command, and the output
// format. These types are what the CLI layer produces and command handlers
// consume; they carry no behavior.
package requests

type OutputFormat string

const (
	FormatJSON OutputFormat = "json"
	FormatText OutputFormat = "text"
)

type GlobalOptions struct {
	Workspace            string
	SchemaPath           string
	Format               OutputFormat
	FormatExplicit       bool
	ConfigPath           string
	RequireAbsolutePaths bool
	Verbose              bool
}

type Command struct {
	Name   string
	Args   []string
	Global GlobalOptions
}
