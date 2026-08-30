package entitydoc

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/iofailure"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

// ScanMarkdownFiles walks a workspace and returns its Markdown documents in a
// stable order. The raw walk error is returned as-is; callers decide which
// diagnostic code and message to report.
// ScanMarkdownFiles lists the workspace documents in a stable order.
//
// Unlike the parsing functions it returns a domain error: every caller reported
// a scan failure identically, so leaving the wrapping to them bought six copies
// of the same three lines and no freedom anyone used.
func ScanMarkdownFiles(workspacePath string) ([]string, *domainerrors.AppError) {
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
			iofailure.Details(walkErr),
		)
	}

	sort.Strings(markdownFiles)
	return markdownFiles, nil
}
