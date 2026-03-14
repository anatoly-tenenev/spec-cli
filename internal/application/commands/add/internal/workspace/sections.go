package workspace

import (
	"regexp"
	"sort"
	"strings"
)

var (
	headingPattern            = regexp.MustCompile(`^\s{0,3}#{1,6}\s+(.+?)\s*$`)
	headingLinkLabelPattern   = regexp.MustCompile(`^\[(.+)]\(#([^\s#()]+)\)\s*$`)
	headingSuffixLabelPattern = regexp.MustCompile(`^(.*?)\s+\{#([^\s{}]+)\}\s*$`)
)

type SectionContent struct {
	Title string
	Body  string
}

type sectionStart struct {
	line  int
	label string
	title string
}

func ExtractSections(body string) (map[string]SectionContent, []string) {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	starts := make([]sectionStart, 0)
	labelCounts := map[string]int{}

	for idx, line := range lines {
		matches := headingPattern.FindStringSubmatch(line)
		if len(matches) != 2 {
			continue
		}

		label, title, ok := parseHeadingLabel(strings.TrimSpace(matches[1]))
		if !ok {
			continue
		}
		starts = append(starts, sectionStart{line: idx, label: label, title: title})
		labelCounts[label]++
	}

	duplicateSet := map[string]struct{}{}
	duplicates := make([]string, 0)
	for _, start := range starts {
		if labelCounts[start.label] <= 1 {
			continue
		}
		if _, seen := duplicateSet[start.label]; seen {
			continue
		}
		duplicateSet[start.label] = struct{}{}
		duplicates = append(duplicates, start.label)
	}
	sort.Strings(duplicates)

	sections := map[string]SectionContent{}
	for idx, start := range starts {
		if labelCounts[start.label] > 1 {
			continue
		}

		startLine := start.line + 1
		endLine := len(lines)
		if idx+1 < len(starts) {
			endLine = starts[idx+1].line
		}

		rawText := strings.Join(lines[startLine:endLine], "\n")
		sections[start.label] = SectionContent{Title: start.title, Body: strings.TrimSpace(rawText)}
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
