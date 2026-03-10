package requests

type OutputFormat string

const (
	FormatJSON OutputFormat = "json"
)

type GlobalOptions struct {
	Workspace            string
	SchemaPath           string
	Format               OutputFormat
	ConfigPath           string
	RequireAbsolutePaths bool
	Verbose              bool
}

type Command struct {
	Name   string
	Args   []string
	Global GlobalOptions
}
