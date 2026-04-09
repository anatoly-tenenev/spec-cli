package version

import "github.com/anatoly-tenenev/spec-cli/internal/application/help/helpmodel"

func HelpSpec() helpmodel.CommandSpec {
	return helpmodel.CommandSpec{
		Name:    "version",
		Summary: "show CLI version",
		Syntaxes: []string{
			"spec-cli version",
		},
		OperationModel: []string{
			"Returns the CLI version only.",
		},
		Rules: []string{
			"Returns semantic version string for current binary build.",
		},
		Examples: []string{
			"spec-cli version",
		},
	}
}
