// Package workspace collects the documents validate will check and parses them
// the way validate needs. A document whose type cannot be read survives the
// --type filter rather than being skipped: it may be exactly the broken
// document the caller is looking for.
//
// candidates.go builds the candidate set; frontmatter.go parses a document,
// normalizing values so a date is compared as the string the schema declares.
package workspace

import (
	"os"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/entitydoc"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func BuildCandidateSet(workspace string, typeFilters map[string]struct{}) ([]model.WorkspaceCandidate, *domainerrors.AppError) {
	markdownFiles, scanErr := entitydoc.ScanMarkdownFiles(workspace)
	if scanErr != nil {
		return nil, scanErr
	}

	if len(typeFilters) == 0 {
		candidates := make([]model.WorkspaceCandidate, 0, len(markdownFiles))
		for _, path := range markdownFiles {
			candidates = append(candidates, model.WorkspaceCandidate{Path: path})
		}
		return candidates, nil
	}

	candidates := make([]model.WorkspaceCandidate, 0, len(markdownFiles))
	for _, path := range markdownFiles {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, domainerrors.New(
				domainerrors.CodeReadFailed,
				"failed to read workspace document",
				entitydoc.IOFailureDetails(err),
			)
		}

		typeName, ok := extractTypeForFilter(raw)
		if !ok {
			candidates = append(candidates, model.WorkspaceCandidate{Path: path})
			continue
		}
		if _, included := typeFilters[typeName]; included {
			candidates = append(candidates, model.WorkspaceCandidate{Path: path})
		}
	}

	return candidates, nil
}

func extractTypeForFilter(raw []byte) (string, bool) {
	frontmatter, _, err := ParseWithNormalizedValues(raw)
	if err != nil {
		return "", false
	}
	typeName, ok := entitydoc.ReadStringField(frontmatter, "type")
	if !ok {
		return "", false
	}
	return typeName, true
}
