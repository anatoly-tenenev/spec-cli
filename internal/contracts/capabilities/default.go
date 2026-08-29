// Package capabilities holds the capability list the CLI advertises to
// callers. It is part of the wire contract, so entries are added only together
// with the feature they announce.
package capabilities

var Default = []string{
	"help",
	"schema",
	"query",
	"get",
	"add",
	"update",
	"delete",
	"validate",
	"version",
	"format:json",
	"format:text",
}
