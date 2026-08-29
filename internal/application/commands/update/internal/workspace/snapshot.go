// Package workspace reads the workspace for update and locates the target.
// The target id is extracted leniently - by YAML, and failing that line by
// line - so a document whose frontmatter no longer parses can still be
// repaired, which is the case update exists to serve.
//
// snapshot.go builds the snapshot and locates the target; frontmatter.go and
// sections.go adapt the shared entitydoc parsing to what update needs,
// including the line ranges that let one section be rewritten in place.
package workspace

import (
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/writemodel"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/entitydoc"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

var locatorIDPattern = regexp.MustCompile(`^id\s*:\s*(.+?)\s*$`)

func BuildSnapshot(workspacePath string, targetID string) (model.Snapshot, *domainerrors.AppError) {
	files, scanErr := entitydoc.ScanMarkdownFiles(workspacePath)
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
				entitydoc.IOFailureDetails(err),
			)
		}

		if trimmedTargetID != "" {
			if locatedID, ok := entitydoc.ExtractIDLenient(raw); ok && locatedID == trimmedTargetID {
				snapshot.TargetMatches = append(snapshot.TargetMatches, model.TargetMatch{
					PathAbs: cleanPath,
					Raw:     raw,
				})
			}
		}

		frontmatter, body, parseErr := ParseWithNormalizedDates(raw)
		if parseErr != nil {
			continue
		}

		typeName, hasType := entitydoc.ReadStringField(frontmatter, "type")
		id, hasID := entitydoc.ReadStringField(frontmatter, "id")
		slug, hasSlug := entitydoc.ReadStringField(frontmatter, "slug")
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
			Meta:         writemodel.BuildMeta(frontmatter),
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
