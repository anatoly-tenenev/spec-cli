// Package workspace reads the whole workspace once into the snapshot delete
// decides from: every parsed document, plus the files that match the target
// id. The target is matched leniently, so a document with broken frontmatter
// can still be deleted - otherwise the command would refuse to remove exactly
// the documents most likely to need removing.
//
// Documents that fail to parse are skipped as reference sources: they cannot
// be shown to point at the target, and delete does not guess.
package workspace

import (
	"os"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/delete/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/entitydoc"
	"github.com/anatoly-tenenev/spec-cli/internal/application/iofailure"
	"github.com/anatoly-tenenev/spec-cli/internal/application/values"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func BuildSnapshot(workspacePath string, targetID string) (model.Snapshot, *domainerrors.AppError) {
	files, scanErr := entitydoc.ScanMarkdownFiles(workspacePath)
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
				iofailure.Details(err),
			)
		}

		if trimmedTargetID != "" {
			if locatedID, ok := entitydoc.ExtractIDLenient(raw); ok && locatedID == trimmedTargetID {
				snapshot.TargetMatches = append(snapshot.TargetMatches, model.TargetMatch{PathAbs: pathAbs, Raw: raw})
			}
		}

		document, parseErr := entitydoc.ParseDocument(raw)
		if parseErr != nil {
			continue
		}

		// A document that cannot be identified is skipped rather than reported:
		// delete judges the target, not the rest of the workspace.
		if document.Type == "" || document.ID == "" {
			continue
		}

		snapshot.Documents = append(snapshot.Documents, model.ParsedDocument{
			PathAbs:     pathAbs,
			Type:        document.Type,
			ID:          document.ID,
			Revision:    document.Revision,
			Frontmatter: values.NormalizeMap(document.Frontmatter),
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
