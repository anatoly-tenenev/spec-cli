package workspace

import (
	"regexp"
	"strings"
)

var (
	headingPattern            = regexp.MustCompile(`^\s{0,3}#{1,6}\s+(.+?)\s*$`)
	headingLinkLabelPattern   = regexp.MustCompile(`^\[(.+)]\(#([^\s#()]+)\)\s*$`)
	headingSuffixLabelPattern = regexp.MustCompile(`^(.*?)\s+\{#([^\s{}]+)\}\s*$`)
)

type sectionStart struct {
	line  int
	label string
}

func extractSections(body string) map[string]string {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	starts := make([]sectionStart, 0)
	for idx, line := range lines {
		headingMatches := headingPattern.FindStringSubmatch(line)
		if len(headingMatches) != 2 {
			continue
		}
		label, _, ok := parseHeadingLabel(strings.TrimSpace(headingMatches[1]))
		if !ok {
			continue
		}
		starts = append(starts, sectionStart{line: idx, label: label})
	}

	sections := map[string]string{}
	for idx, start := range starts {
		startLine := start.line + 1
		endLine := len(lines)
		if idx+1 < len(starts) {
			endLine = starts[idx+1].line
		}

		rawText := strings.Join(lines[startLine:endLine], "\n")
		sections[start.label] = strings.TrimSpace(rawText)
	}
	return sections
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
