package documents

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/internal/yamlnodes"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/workspace/internal/diagnostics"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	"gopkg.in/yaml.v3"
)

type Entity struct {
	Type        string
	ID          string
	Slug        string
	CreatedDate string
	UpdatedDate string
	Revision    string
	Frontmatter map[string]any
	Sections    map[string]string
	RawContent  string
}

func ScanMarkdownFiles(workspacePath string) ([]string, *domainerrors.AppError) {
	markdownFiles := make([]string, 0)
	walkErr := filepath.WalkDir(workspacePath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			markdownFiles = append(markdownFiles, path)
		}
		return nil
	})
	if walkErr != nil {
		return nil, domainerrors.New(
			domainerrors.CodeReadFailed,
			"failed to scan workspace",
			map[string]any{"reason": walkErr.Error()},
		)
	}

	sort.Strings(markdownFiles)
	return markdownFiles, nil
}

func ParseEntityFile(path string) (*Entity, *domainerrors.AppError) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, domainerrors.New(
			domainerrors.CodeReadFailed,
			"failed to read workspace document",
			map[string]any{"reason": err.Error()},
		)
	}

	frontmatter, body, parseErr := parseFrontmatter(raw)
	if parseErr != nil {
		return nil, diagnostics.NewReadError(
			"failed to parse workspace document",
			parseErr.Error(),
			diagnostics.FrontmatterStandardRef,
			nil,
		)
	}

	typeName, ok := readStringField(frontmatter, "type")
	if !ok {
		return nil, diagnostics.NewReadError(
			"failed to determine entity type",
			requiredBuiltinFieldMessage("type"),
			diagnostics.TypeStandardRef,
			nil,
		)
	}
	id, ok := readStringField(frontmatter, "id")
	if !ok {
		return nil, diagnostics.NewReadError(
			"failed to determine entity id",
			requiredBuiltinFieldMessage("id"),
			diagnostics.IDStandardRef,
			nil,
		)
	}
	slug, ok := readStringField(frontmatter, "slug")
	if !ok {
		return nil, diagnostics.NewReadError(
			"failed to determine entity slug",
			requiredBuiltinFieldMessage("slug"),
			diagnostics.SlugStandardRef,
			nil,
		)
	}

	createdDate, _ := readStringField(frontmatter, "createdDate")
	updatedDate, _ := readStringField(frontmatter, "updatedDate")

	revisionHash := sha256.Sum256(raw)
	revision := "sha256:" + hex.EncodeToString(revisionHash[:])

	return &Entity{
		Type:        typeName,
		ID:          id,
		Slug:        slug,
		CreatedDate: createdDate,
		UpdatedDate: updatedDate,
		Revision:    revision,
		Frontmatter: frontmatter,
		Sections:    extractSections(body),
		RawContent:  body,
	}, nil
}

func requiredBuiltinFieldMessage(field string) string {
	return fmt.Sprintf("built-in field '%s' is required", field)
}

func parseFrontmatter(raw []byte) (map[string]any, string, error) {
	source := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(source, "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return nil, "", fmt.Errorf("frontmatter must start with '---' on the first line")
	}

	endIdx := -1
	for idx := 1; idx < len(lines); idx++ {
		if lines[idx] == "---" || lines[idx] == "..." {
			endIdx = idx
			break
		}
	}
	if endIdx == -1 {
		return nil, "", fmt.Errorf("frontmatter closing delimiter ('---' or '...') is missing")
	}

	frontmatterBody := strings.Join(lines[1:endIdx], "\n")
	body := strings.Join(lines[endIdx+1:], "\n")

	var root yaml.Node
	if err := yaml.Unmarshal([]byte(frontmatterBody), &root); err != nil {
		return nil, "", fmt.Errorf("frontmatter is not valid yaml: %w", err)
	}

	doc := yamlnodes.FirstContentNode(&root)
	if doc == nil || doc.Kind != yaml.MappingNode {
		return nil, "", fmt.Errorf("frontmatter root must be a yaml mapping")
	}

	if duplicateKey, ok := yamlnodes.FindDuplicateMappingKey(doc); ok {
		return nil, "", fmt.Errorf("frontmatter contains duplicate key '%s'", duplicateKey)
	}

	fields := map[string]any{}
	if err := doc.Decode(&fields); err != nil {
		return nil, "", fmt.Errorf("frontmatter decode failed: %w", err)
	}

	return fields, body, nil
}

func readStringField(values map[string]any, key string) (string, bool) {
	raw, exists := values[key]
	if !exists {
		return "", false
	}

	var value string
	switch typed := raw.(type) {
	case string:
		value = typed
	case time.Time:
		value = typed.Format("2006-01-02")
	default:
		return "", false
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return value, true
}

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
