package helpmodel

import "fmt"

type PositionalSpec struct {
	Name        string
	Required    bool
	Repeatable  bool
	Description string
}

type OptionSpec struct {
	Name                    string
	ValueSyntax             string
	TakesValue              bool
	Required                bool
	Repeatable              bool
	MutuallyExclusiveGroups []string
	SchemaDerived           bool
	SchemaDerivation        string
	Description             string
}

type DetailSectionSpec struct {
	Title string
	Lines []string
}

type CommandSpec struct {
	Name           string
	Summary        string
	Syntaxes       []string
	OperationModel []string
	DetailSections []DetailSectionSpec
	Positionals    []PositionalSpec
	Options        []OptionSpec
	Rules          []string
	Examples       []string
}

type GlobalOptionSpec struct {
	Name        string
	ValueSyntax string
	Description string
}

type Catalog struct {
	ordered []CommandSpec
	byName  map[string]CommandSpec
}

func NewCatalog(specs []CommandSpec) (*Catalog, error) {
	ordered := make([]CommandSpec, 0, len(specs))
	byName := make(map[string]CommandSpec, len(specs))
	for _, spec := range specs {
		if spec.Name == "" {
			return nil, fmt.Errorf("command name is required")
		}
		if _, exists := byName[spec.Name]; exists {
			return nil, fmt.Errorf("duplicate command in catalog: %s", spec.Name)
		}
		ordered = append(ordered, spec)
		byName[spec.Name] = spec
	}
	return &Catalog{ordered: ordered, byName: byName}, nil
}

func MustCatalog(specs []CommandSpec) *Catalog {
	catalog, err := NewCatalog(specs)
	if err != nil {
		panic(err)
	}
	return catalog
}

func (c *Catalog) Ordered() []CommandSpec {
	if c == nil {
		return nil
	}
	out := make([]CommandSpec, len(c.ordered))
	copy(out, c.ordered)
	return out
}

func (c *Catalog) Names() []string {
	if c == nil {
		return nil
	}
	names := make([]string, len(c.ordered))
	for idx, command := range c.ordered {
		names[idx] = command.Name
	}
	return names
}

func (c *Catalog) Has(name string) bool {
	if c == nil {
		return false
	}
	_, exists := c.byName[name]
	return exists
}

func (c *Catalog) Find(name string) (CommandSpec, bool) {
	if c == nil {
		return CommandSpec{}, false
	}
	spec, exists := c.byName[name]
	return spec, exists
}
