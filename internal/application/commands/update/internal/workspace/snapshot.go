package workspace

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/support"
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
		Entities:      make([]model.WorkspaceEntity, 0, len(files)),
		EntitiesByID:  map[string][]model.WorkspaceEntity{},
		SlugsByType:   map[string]map[string][]model.WorkspaceEntity{},
		ExistingPaths: map[string]struct{}{},
		TargetMatches: []model.TargetMatch{},
	}

	trimmedTargetID := strings.TrimSpace(targetID)
	for _, pathAbs := range files {
		cleanPath := filepath.Clean(pathAbs)
		snapshot.ExistingPaths[cleanPath] = struct{}{}

		raw, err := os.ReadFile(cleanPath)
		if err != nil {
			return model.Snapshot{}, domainerrors.New(
				domainerrors.CodeReadFailed,
				"failed to read workspace document",
				map[string]any{"reason": err.Error()},
			)
		}

		if trimmedTargetID != "" {
			if locatedID, ok := extractIDForLocate(raw); ok && locatedID == trimmedTargetID {
				snapshot.TargetMatches = append(snapshot.TargetMatches, model.TargetMatch{
					PathAbs: cleanPath,
					Raw:     raw,
				})
			}
		}

		frontmatter, body, parseErr := ParseFrontmatter(raw)
		if parseErr != nil {
			continue
		}

		typeName, hasType := ReadStringField(frontmatter, "type")
		id, hasID := ReadStringField(frontmatter, "id")
		slug, hasSlug := ReadStringField(frontmatter, "slug")
		if !hasType || !hasID || !hasSlug {
			continue
		}

		relPath, relErr := filepath.Rel(workspacePath, cleanPath)
		if relErr != nil {
			return model.Snapshot{}, domainerrors.New(
				domainerrors.CodeReadFailed,
				"failed to resolve workspace-relative path",
				map[string]any{"reason": relErr.Error()},
			)
		}
		relPosix := filepath.ToSlash(relPath)
		dirPath := path.Dir(relPosix)
		if dirPath == "." {
			dirPath = ""
		}

		entity := model.WorkspaceEntity{
			PathAbs:      cleanPath,
			PathRelPOSIX: relPosix,
			DirPath:      dirPath,
			Type:         typeName,
			ID:           id,
			Slug:         slug,
			Frontmatter:  frontmatter,
			Meta:         BuildMeta(frontmatter),
			Body:         body,
		}

		snapshot.Entities = append(snapshot.Entities, entity)
		snapshot.EntitiesByID[id] = append(snapshot.EntitiesByID[id], entity)
		if _, exists := snapshot.SlugsByType[typeName]; !exists {
			snapshot.SlugsByType[typeName] = map[string][]model.WorkspaceEntity{}
		}
		snapshot.SlugsByType[typeName][slug] = append(snapshot.SlugsByType[typeName][slug], entity)
	}

	return snapshot, nil
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
	return ReadStringField(fields, "id")
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
