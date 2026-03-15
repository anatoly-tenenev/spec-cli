package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/delete/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/delete/internal/support"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	"gopkg.in/yaml.v3"
)

var locatorIDPattern = regexp.MustCompile(`^id\s*:\s*(.+?)\s*$`)

func BuildSnapshot(workspacePath string, targetID string) (model.Snapshot, *domainerrors.AppError) {
	files, scanErr := scanMarkdownFiles(workspacePath)
	if scanErr != nil {
		return model.Snapshot{}, scanErr
	}

	snapshot := model.Snapshot{
		WorkspacePath: workspacePath,
		Documents:     make([]model.ParsedDocument, 0, len(files)),
		TargetMatches: []model.TargetMatch{},
	}

	trimmedTargetID := strings.TrimSpace(targetID)
	for _, pathAbs := range files {
		raw, err := os.ReadFile(pathAbs)
		if err != nil {
			return model.Snapshot{}, domainerrors.New(
				domainerrors.CodeReadFailed,
				"failed to read workspace document",
				map[string]any{"reason": classifyIOReason(err)},
			)
		}

		if trimmedTargetID != "" {
			if locatedID, ok := extractIDForLocate(raw); ok && locatedID == trimmedTargetID {
				snapshot.TargetMatches = append(snapshot.TargetMatches, model.TargetMatch{PathAbs: pathAbs, Raw: raw})
			}
		}

		frontmatter, _, parseErr := parseFrontmatter(raw)
		if parseErr != nil {
			continue
		}

		typeName, hasType := readStringField(frontmatter, "type")
		id, hasID := readStringField(frontmatter, "id")
		if !hasType || !hasID {
			continue
		}

		snapshot.Documents = append(snapshot.Documents, model.ParsedDocument{
			PathAbs:     pathAbs,
			Type:        typeName,
			ID:          id,
			Revision:    computeRevision(raw),
			Frontmatter: normalizeMap(frontmatter),
		})
	}

	return snapshot, nil
}

func FindTargetDocument(snapshot model.Snapshot, pathAbs string) (model.ParsedDocument, bool) {
	for _, doc := range snapshot.Documents {
		if doc.PathAbs == pathAbs {
			return doc, true
		}
	}
	return model.ParsedDocument{}, false
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
			map[string]any{"reason": classifyIOReason(walkErr)},
		)
	}

	sort.Strings(markdownFiles)
	return markdownFiles, nil
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

func computeRevision(raw []byte) string {
	hash := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(hash[:])
}

func classifyIOReason(err error) string {
	switch {
	case errors.Is(err, os.ErrPermission):
		return "permission denied"
	case errors.Is(err, os.ErrNotExist):
		return "not found"
	default:
		return "i/o error"
	}
}
