package workspace

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func BuildSnapshot(workspacePath string, entityTypes map[string]model.EntityTypeSpec) (model.Snapshot, *domainerrors.AppError) {
	files, scanErr := scanMarkdownFiles(workspacePath)
	if scanErr != nil {
		return model.Snapshot{}, scanErr
	}

	snapshot := model.Snapshot{
		WorkspacePath:   workspacePath,
		Entities:        make([]model.WorkspaceEntity, 0, len(files)),
		EntitiesByID:    map[string][]model.WorkspaceEntity{},
		SlugsByType:     map[string]map[string][]model.WorkspaceEntity{},
		MaxSuffixByType: map[string]int{},
		ExistingPaths:   map[string]struct{}{},
	}

	for _, pathAbs := range files {
		snapshot.ExistingPaths[pathAbs] = struct{}{}

		raw, err := os.ReadFile(pathAbs)
		if err != nil {
			return model.Snapshot{}, domainerrors.New(
				domainerrors.CodeReadFailed,
				"failed to read workspace document",
				map[string]any{"reason": err.Error()},
			)
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

		relPath, relErr := filepath.Rel(workspacePath, pathAbs)
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
			PathAbs:      pathAbs,
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

		typeSpec, knownType := entityTypes[typeName]
		if !knownType {
			continue
		}
		suffix, ok := parseIDSuffix(id, typeSpec.IDPrefix)
		if !ok {
			continue
		}
		if current, exists := snapshot.MaxSuffixByType[typeName]; !exists || suffix > current {
			snapshot.MaxSuffixByType[typeName] = suffix
		}
	}

	for typeName := range entityTypes {
		if _, exists := snapshot.MaxSuffixByType[typeName]; !exists {
			snapshot.MaxSuffixByType[typeName] = -1
		}
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

func parseIDSuffix(id string, prefix string) (int, bool) {
	expectedPrefix := prefix + "-"
	if !strings.HasPrefix(id, expectedPrefix) {
		return 0, false
	}

	rawSuffix := strings.TrimPrefix(id, expectedPrefix)
	if rawSuffix == "" {
		return 0, false
	}
	for _, ch := range rawSuffix {
		if ch < '0' || ch > '9' {
			return 0, false
		}
	}

	value := 0
	for _, ch := range rawSuffix {
		value = value*10 + int(ch-'0')
	}
	return value, true
}
