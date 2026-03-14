package workspace

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/validate/internal/model"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func BuildCandidateSet(workspace string, typeFilters map[string]struct{}) ([]model.WorkspaceCandidate, *domainerrors.AppError) {
	markdownFiles := make([]string, 0)
	walkErr := filepath.WalkDir(workspace, func(path string, entry fs.DirEntry, err error) error {
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
			domainerrors.CodeWriteFailed,
			"failed to scan workspace",
			map[string]any{"reason": walkErr.Error()},
		)
	}

	sort.Strings(markdownFiles)
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
				domainerrors.CodeWriteFailed,
				"failed to read workspace document",
				nil,
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
