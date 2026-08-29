package entitydoc

import (
	"errors"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
			IOFailureDetails(walkErr),
		)
	}

	sort.Strings(markdownFiles)
	return markdownFiles, nil
}

// IOFailureDetails describes a filesystem error twice: `reason` is a stable
// value a caller can branch on, `detail` is the original message with the path
// for a human to read. The raw text differs per platform, so it must not be the
// field consumers key off.
func IOFailureDetails(err error) map[string]any {
	return map[string]any{
		"reason": ClassifyIOReason(err),
		"detail": err.Error(),
	}
}

// ClassifyIOReason maps a filesystem error onto a small, stable vocabulary.
// It compares the kernel error number rather than the message, so the result is
// the same on every platform while err.Error() is not.
func ClassifyIOReason(err error) string {
	switch {
	case errors.Is(err, os.ErrPermission):
		return "permission denied"
	case errors.Is(err, os.ErrNotExist):
		return "not found"
	default:
		return "i/o error"
	}
}
