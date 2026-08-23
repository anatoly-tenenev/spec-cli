package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/delete/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/values"
	"github.com/anatoly-tenenev/spec-cli/internal/application/entitydoc"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

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
				entitydoc.IOFailureDetails(err),
			)
		}

		if trimmedTargetID != "" {
			if locatedID, ok := entitydoc.ExtractIDLenient(raw); ok && locatedID == trimmedTargetID {
				snapshot.TargetMatches = append(snapshot.TargetMatches, model.TargetMatch{PathAbs: pathAbs, Raw: raw})
			}
		}

		frontmatter, _, parseErr := entitydoc.ParseFrontmatter(raw)
		if parseErr != nil {
			continue
		}

		typeName, hasType := entitydoc.ReadStringField(frontmatter, "type")
		id, hasID := entitydoc.ReadStringField(frontmatter, "id")
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
	markdownFiles, walkErr := entitydoc.ScanMarkdownFiles(workspacePath)
	if walkErr != nil {
		return nil, domainerrors.New(
			domainerrors.CodeReadFailed,
			"failed to scan workspace",
			entitydoc.IOFailureDetails(walkErr),
		)
	}
	return markdownFiles, nil
}

func normalizeMap(input map[string]any) map[string]any {
	normalized := make(map[string]any, len(input))
	for key, value := range input {
		normalized[key] = values.NormalizeValue(value)
	}
	return normalized
}

func computeRevision(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
