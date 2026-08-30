// Package workspace reads the existing workspace into the snapshot add places
// a new entity against: which ids and slugs are taken, how far the id sequence
// for each type has run, and which paths already exist.
//
// Documents are read through the shared entitydoc parsing, so add sees the
// same frontmatter and sections every other command does.
package workspace

import (
	"os"
	"path"
	"path/filepath"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/entityids"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/writemodel"
	"github.com/anatoly-tenenev/spec-cli/internal/application/entitydoc"
	"github.com/anatoly-tenenev/spec-cli/internal/application/iofailure"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func BuildSnapshot(workspacePath string, entityTypes map[string]model.EntityTypeSpec) (model.Snapshot, *domainerrors.AppError) {
	files, scanErr := entitydoc.ScanMarkdownFiles(workspacePath)
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
				iofailure.Details(err),
			)
		}

		frontmatter, body, parseErr := entitydoc.ParseFrontmatter(raw)
		if parseErr != nil {
			continue
		}

		typeName, hasType := entitydoc.ReadStringField(frontmatter, "type")
		id, hasID := entitydoc.ReadStringField(frontmatter, "id")
		slug, hasSlug := entitydoc.ReadStringField(frontmatter, "slug")
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
			Meta:         writemodel.BuildMeta(frontmatter),
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
		suffix, ok := entityids.ParseSuffix(id, typeSpec.IDPrefix)
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
