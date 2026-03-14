package workspace

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

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/get/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/get/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	"gopkg.in/yaml.v3"
)

const (
	getWorkspaceFrontmatterStandardRef = "10.2"
	getWorkspaceTypeStandardRef        = "5.3"
	getWorkspaceIDStandardRef          = "11.1"
)

var (
	headingPattern            = regexp.MustCompile(`^\s{0,3}#{1,6}\s+(.+?)\s*$`)
	headingLinkLabelPattern   = regexp.MustCompile(`^\[(.+)]\(#([^\s#()]+)\)\s*$`)
	headingSuffixLabelPattern = regexp.MustCompile(`^(.*?)\s+\{#([^\s{}]+)\}\s*$`)
	locatorIDPattern          = regexp.MustCompile(`^id\s*:\s*(.+?)\s*$`)
)

type sectionStart struct {
	line  int
	label string
}

type locatedCandidate struct {
	path string
	raw  []byte
}

func LocateByID(workspacePath string, targetID string) (model.LocateResult, *domainerrors.AppError) {
	files, scanErr := scanMarkdownFiles(workspacePath)
	if scanErr != nil {
		return model.LocateResult{}, scanErr
	}

	identityIndex := map[string][]model.EntityIdentity{}
	matches := make([]locatedCandidate, 0, 2)

	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			return model.LocateResult{}, domainerrors.New(
				domainerrors.CodeReadFailed,
				"failed to read workspace document",
				map[string]any{"reason": err.Error()},
			)
		}

		if extractedID, ok := extractIDForLocate(raw); ok && extractedID == targetID {
			matches = append(matches, locatedCandidate{path: path, raw: raw})
		}

		if identity, ok := extractIdentity(raw); ok {
			identityIndex[identity.ID] = append(identityIndex[identity.ID], identity)
		}
	}

	switch len(matches) {
	case 0:
		return model.LocateResult{}, domainerrors.New(
			domainerrors.CodeEntityNotFound,
			"target entity is not found",
			map[string]any{"id": targetID},
		)
	case 1:
		return model.LocateResult{
			TargetPath:    matches[0].path,
			TargetRaw:     matches[0].raw,
			IdentityIndex: identityIndex,
		}, nil
	default:
		return model.LocateResult{}, domainerrors.New(
			domainerrors.CodeTargetAmbiguous,
			"target id is ambiguous",
			map[string]any{"id": targetID, "matches": len(matches)},
		)
	}
}

func ReadTarget(path string, raw []byte, requestedID string) (model.ParsedTarget, *domainerrors.AppError) {
	frontmatter, body, parseErr := parseFrontmatter(raw)
	if parseErr != nil {
		return model.ParsedTarget{}, newReadError(
			"failed to parse target frontmatter",
			parseErr.Error(),
			getWorkspaceFrontmatterStandardRef,
			nil,
		)
	}

	typeName, ok := readStringField(frontmatter, "type")
	if !ok {
		return model.ParsedTarget{}, newReadError(
			"failed to determine entity type",
			"built-in field 'type' is required",
			getWorkspaceTypeStandardRef,
			nil,
		)
	}

	entityID, ok := readStringField(frontmatter, "id")
	if !ok {
		return model.ParsedTarget{}, newReadError(
			"failed to determine entity id",
			"built-in field 'id' is required",
			getWorkspaceIDStandardRef,
			nil,
		)
	}

	if entityID != requestedID {
		return model.ParsedTarget{}, newReadError(
			"failed to read target entity",
			"target document id does not match requested --id",
			getWorkspaceIDStandardRef,
			map[string]any{"expected_id": requestedID, "actual_id": entityID},
		)
	}

	slug, _ := readStringField(frontmatter, "slug")
	createdDate, _ := readStringField(frontmatter, "created_date")
	updatedDate, _ := readStringField(frontmatter, "updated_date")

	revisionHash := sha256.Sum256(raw)
	revision := "sha256:" + hex.EncodeToString(revisionHash[:])
	sections, duplicateLabels := extractSections(body)

	return model.ParsedTarget{
		Path:                   path,
		Type:                   typeName,
		ID:                     entityID,
		Slug:                   slug,
		CreatedDate:            createdDate,
		UpdatedDate:            updatedDate,
		Revision:               revision,
		RawBody:                body,
		Frontmatter:            normalizeMap(frontmatter),
		Sections:               sections,
		DuplicateSectionLabels: duplicateLabels,
	}, nil
}

func scanMarkdownFiles(workspacePath string) ([]string, *domainerrors.AppError) {
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

func extractIdentity(raw []byte) (model.EntityIdentity, bool) {
	frontmatter, _, err := parseFrontmatter(raw)
	if err != nil {
		return model.EntityIdentity{}, false
	}

	typeName, ok := readStringField(frontmatter, "type")
	if !ok {
		return model.EntityIdentity{}, false
	}
	id, ok := readStringField(frontmatter, "id")
	if !ok {
		return model.EntityIdentity{}, false
	}
	slug, ok := readStringField(frontmatter, "slug")
	if !ok {
		return model.EntityIdentity{}, false
	}

	return model.EntityIdentity{Type: typeName, ID: id, Slug: slug}, true
}

func extractIDForLocate(raw []byte) (string, bool) {
	source := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(source, "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return "", false
	}

	endIdx := -1
	for idx := 1; idx < len(lines); idx++ {
		if lines[idx] == "---" || lines[idx] == "..." {
			endIdx = idx
			break
		}
	}
	if endIdx == -1 {
		// Keep locator tolerant: malformed frontmatter without a closing delimiter
		// may still contain a determinable id we can use to avoid masking with NOT_FOUND.
		return extractIDFromLines(strings.Join(lines[1:], "\n"))
	}

	frontmatterBody := strings.Join(lines[1:endIdx], "\n")
	if parsedID, ok := extractIDFromYAML(frontmatterBody); ok {
		return parsedID, true
	}
	return extractIDFromLines(frontmatterBody)
}

func extractIDFromYAML(frontmatterBody string) (string, bool) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(frontmatterBody), &root); err != nil {
		return "", false
	}

	doc := support.FirstContentNode(&root)
	if doc == nil || doc.Kind != yaml.MappingNode {
		return "", false
	}

	fields := map[string]any{}
	if err := doc.Decode(&fields); err != nil {
		return "", false
	}

	return readStringField(fields, "id")
}

func extractIDFromLines(frontmatterBody string) (string, bool) {
	for _, line := range strings.Split(frontmatterBody, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		matches := locatorIDPattern.FindStringSubmatch(trimmed)
		if len(matches) != 2 {
			continue
		}

		value := strings.TrimSpace(matches[1])
		if index := strings.Index(value, " #"); index >= 0 {
			value = strings.TrimSpace(value[:index])
		}
		value = trimWrappingQuotes(value)
		if value == "" {
			continue
		}
		return value, true
	}
	return "", false
}

func trimWrappingQuotes(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) >= 2 {
		if (trimmed[0] == '\'' && trimmed[len(trimmed)-1] == '\'') ||
			(trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"') {
			return strings.TrimSpace(trimmed[1 : len(trimmed)-1])
		}
	}
	return trimmed
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

	doc := support.FirstContentNode(&root)
	if doc == nil || doc.Kind != yaml.MappingNode {
		return nil, "", fmt.Errorf("frontmatter root must be a yaml mapping")
	}
	if duplicateKey, ok := support.FindDuplicateMappingKey(doc); ok {
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

func normalizeMap(values map[string]any) map[string]any {
	normalized := make(map[string]any, len(values))
	for key, value := range values {
		normalized[key] = normalizeValue(value)
	}
	return normalized
}

func normalizeValue(value any) any {
	switch typed := value.(type) {
	case time.Time:
		return typed.Format("2006-01-02")
	case map[string]any:
		normalized := make(map[string]any, len(typed))
		for key, item := range typed {
			normalized[key] = normalizeValue(item)
		}
		return normalized
	case []any:
		normalized := make([]any, len(typed))
		for idx := range typed {
			normalized[idx] = normalizeValue(typed[idx])
		}
		return normalized
	default:
		return typed
	}
}

func extractSections(body string) (map[string]string, map[string]int) {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	starts := make([]sectionStart, 0)
	labelCounts := map[string]int{}

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
		labelCounts[label]++
	}

	duplicates := map[string]int{}
	for label, count := range labelCounts {
		if count > 1 {
			duplicates[label] = count
		}
	}

	sections := map[string]string{}
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
		sections[start.label] = strings.TrimSpace(rawText)
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

func newReadError(message string, issueMessage string, standardRef string, details map[string]any) *domainerrors.AppError {
	issue := support.ValidationIssue("error", "InstanceError", issueMessage, standardRef)
	return domainerrors.New(domainerrors.CodeReadFailed, message, support.WithValidationIssues(details, issue))
}
