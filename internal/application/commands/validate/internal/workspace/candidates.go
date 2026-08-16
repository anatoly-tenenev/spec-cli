package workspace

import (
	"os"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/entitydoc"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func BuildCandidateSet(workspace string, typeFilters map[string]struct{}) ([]model.WorkspaceCandidate, *domainerrors.AppError) {
	markdownFiles, walkErr := entitydoc.ScanMarkdownFiles(workspace)
	if walkErr != nil {
		return nil, domainerrors.New(
			domainerrors.CodeReadFailed,
			"failed to scan workspace",
			entitydoc.IOFailureDetails(walkErr),
		)
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
	frontmatter, _, err := ParseFrontmatter(raw)
	if err != nil {
		return "", false
	}
	typeName, ok := ReadStringField(frontmatter, "type")
	if !ok {
		return "", false
	}
	return typeName, true
}
