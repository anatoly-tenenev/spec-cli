package model

type Options struct {
	ID        string
	Selectors []string
}

type SelectNode struct {
	Terminal bool
	Children map[string]*SelectNode
}

type SelectorPlan struct {
	Tree                 *SelectNode
	EffectiveSelectors   []string
	NullIfMissingPaths   map[string]struct{}
	RequiredRefFields    map[string]struct{}
	RequiredSectionNames map[string]struct{}
	RequiresRefs         bool
	RequiresSections     bool
	RequiresAllSections  bool
	RequiresContent      bool
	RequiresContentRaw   bool
}

type EntityTypeSpec struct {
	Name          string
	MetaFields    map[string]struct{}
	RefFields     map[string]struct{}
	SectionFields map[string]struct{}
}

type ReadModel struct {
	EntityTypes      map[string]EntityTypeSpec
	AllowedSelectors map[string]struct{}
}

type EntityIdentity struct {
	Type string
	ID   string
	Slug string
}

type LocateResult struct {
	TargetPath    string
	TargetRaw     []byte
	IdentityIndex map[string][]EntityIdentity
}

type ParsedTarget struct {
	Path                   string
	Type                   string
	ID                     string
	Slug                   string
	CreatedDate            string
	UpdatedDate            string
	Revision               string
	RawBody                string
	Frontmatter            map[string]any
	Sections               map[string]string
	DuplicateSectionLabels map[string]int
}
