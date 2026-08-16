package entitydoc

import (
	"regexp"
	"sort"
	"strings"
)

var (
	headingPattern            = regexp.MustCompile(`^\s{0,3}(#{1,6})\s+(.+?)\s*$`)
	headingLinkLabelPattern   = regexp.MustCompile(`^\[(.+)]\(#([^\s#()]+)\)\s*$`)
	headingSuffixLabelPattern = regexp.MustCompile(`^(.*?)\s+\{#([^\s{}]+)\}\s*$`)
)

// SectionContent is a section reduced to what most callers need.
type SectionContent struct {
	Title string
	Body  string
}

// SectionRange locates one section within the document body.
type SectionRange struct {
	Label        string
	Title        string
	HeadingLine  int
	BodyStart    int
	EndLine      int
	HeadingLevel int
}

// SectionLayout is the raw parse result: where every section sits and how many
// times each label occurs. It reports facts only — deciding what a repeated
// label means is left to the caller.
type SectionLayout struct {
	Lines      []string
	Ranges     []SectionRange
	LabelCount map[string]int
}

type sectionStart struct {
	line         int
	label        string
	title        string
	headingLevel int
}

// BuildSectionLayout scans the body for labelled headings.
func BuildSectionLayout(body string) SectionLayout {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	starts := make([]sectionStart, 0)
	labelCounts := map[string]int{}

	for idx, line := range lines {
		matches := headingPattern.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}

		label, title, ok := ParseHeadingLabel(strings.TrimSpace(matches[2]))
		if !ok {
			continue
		}
		starts = append(starts, sectionStart{
			line:         idx,
			label:        label,
			title:        title,
			headingLevel: len(matches[1]),
		})
		labelCounts[label]++
	}

	ranges := make([]SectionRange, 0, len(starts))
	for idx, start := range starts {
		endLine := len(lines)
		if idx+1 < len(starts) {
			endLine = starts[idx+1].line
		}
		ranges = append(ranges, SectionRange{
			Label:        start.label,
			Title:        start.title,
			HeadingLine:  start.line,
			BodyStart:    start.line + 1,
			EndLine:      endLine,
			HeadingLevel: start.headingLevel,
		})
	}

	return SectionLayout{Lines: lines, Ranges: ranges, LabelCount: labelCounts}
}

// FirstRange returns the first occurrence of a label.
func (layout SectionLayout) FirstRange(label string) (SectionRange, bool) {
	for _, item := range layout.Ranges {
		if item.Label == label {
			return item, true
		}
	}
	return SectionRange{}, false
}

// DuplicateLabels lists labels that occur more than once, sorted for stable output.
func (layout SectionLayout) DuplicateLabels() []string {
	duplicates := make([]string, 0)
	for label, count := range layout.LabelCount {
		if count > 1 {
			duplicates = append(duplicates, label)
		}
	}
	sort.Strings(duplicates)
	return duplicates
}

// Body returns the trimmed text of one section range.
func (layout SectionLayout) Body(item SectionRange) string {
	return strings.TrimSpace(strings.Join(layout.Lines[item.BodyStart:item.EndLine], "\n"))
}

// ExtractSections returns unambiguous sections plus the labels that repeated.
// A repeated label yields no section: the document does not answer which block
// is meant.
func ExtractSections(body string) (map[string]SectionContent, []string) {
	layout := BuildSectionLayout(body)
	duplicates := layout.DuplicateLabels()
	duplicateSet := make(map[string]struct{}, len(duplicates))
	for _, label := range duplicates {
		duplicateSet[label] = struct{}{}
	}

	sections := map[string]SectionContent{}
	for _, item := range layout.Ranges {
		if _, isDuplicate := duplicateSet[item.Label]; isDuplicate {
			continue
		}
		sections[item.Label] = SectionContent{Title: item.Title, Body: layout.Body(item)}
	}

	return sections, duplicates
}

// ParseHeadingLabel extracts the explicit label from a heading, supporting both
// the `[Title](#label)` and the `Title {#label}` forms.
func ParseHeadingLabel(heading string) (label string, title string, ok bool) {
	if linkMatches := headingLinkLabelPattern.FindStringSubmatch(heading); len(linkMatches) == 3 {
		return linkMatches[2], strings.TrimSpace(linkMatches[1]), true
	}

	if suffixMatches := headingSuffixLabelPattern.FindStringSubmatch(heading); len(suffixMatches) == 3 {
		return suffixMatches[2], strings.TrimSpace(suffixMatches[1]), true
	}

	return "", "", false
}
