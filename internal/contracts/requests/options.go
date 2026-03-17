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
