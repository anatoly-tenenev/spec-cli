package workspace

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

type SectionContent struct {
	Title string
	Body  string
}

type SectionRange struct {
	Label        string
	Title        string
	HeadingLine  int
	BodyStart    int
	EndLine      int
	HeadingLevel int
}

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

func BuildSectionLayout(body string) SectionLayout {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	starts := make([]sectionStart, 0)
	labelCounts := map[string]int{}

	for idx, line := range lines {
		matches := headingPattern.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}

		label, title, ok := parseHeadingLabel(strings.TrimSpace(matches[2]))
		if !ok {
			continue
		}
		level := len(matches[1])
		starts = append(starts, sectionStart{
			line:         idx,
			label:        label,
			title:        title,
			headingLevel: level,
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

	return SectionLayout{
		Lines:      lines,
		Ranges:     ranges,
		LabelCount: labelCounts,
	}
}

func (layout SectionLayout) FirstRange(label string) (SectionRange, bool) {
	for _, item := range layout.Ranges {
		if item.Label == label {
			return item, true
		}
	}
	return SectionRange{}, false
}

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

func ExtractSections(body string) (map[string]SectionContent, []string) {
	layout := BuildSectionLayout(body)
	duplicates := layout.DuplicateLabels()
	duplicateSet := map[string]struct{}{}
	for _, label := range duplicates {
		duplicateSet[label] = struct{}{}
	}

	sections := map[string]SectionContent{}
	for _, block := range layout.Ranges {
		if _, isDuplicate := duplicateSet[block.Label]; isDuplicate {
			continue
		}
		rawBody := strings.Join(layout.Lines[block.BodyStart:block.EndLine], "\n")
		sections[block.Label] = SectionContent{
			Title: block.Title,
			Body:  strings.TrimSpace(rawBody),
		}
	}

	return sections, duplicates
}

func parseHeadingLabel(heading string) (label string, title string, ok bool) {
	if linkMatches := headingLinkLabelPattern.FindStringSubmatch(heading); len(linkMatches) == 3 {
		return linkMatches[2], strings.TrimSpace(linkMatches[1]), true
	}

	if suffixMatches := headingSuffixLabelPattern.FindStringSubmatch(heading); len(suffixMatches) == 3 {
		return suffixMatches[2], strings.TrimSpace(suffixMatches[1]), true
	}

	return "", "", false
}
