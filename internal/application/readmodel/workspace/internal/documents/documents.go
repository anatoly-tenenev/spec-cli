// Package documents turns a file on disk into the entity the read layer works
// with, and checks only what reading itself needs: the builtin identity fields
// and a computed revision. Conformance to the schema is not judged here -
// reads must keep working on a non-conforming workspace, because repairing one
// starts by reading it.
//
// Markdown parsing itself is not repeated here; it comes from
// internal/application/entitydoc.
package documents

import (
	"fmt"
	"os"

	"github.com/anatoly-tenenev/spec-cli/internal/application/entitydoc"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/workspace/internal/diagnostics"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

type Entity struct {
	Type        string
	ID          string
	Slug        string
	CreatedDate string
	UpdatedDate string
	Revision    string
	Frontmatter map[string]any
	Sections    map[string]string
	// DuplicateSectionLabels lists labels that appear more than once. Such a
	// section is absent from Sections: the document does not say which block is
	// meant, so reading it is left to fail where it is actually requested.
	DuplicateSectionLabels []string
	RawContent             string
}

func ScanMarkdownFiles(workspacePath string) ([]string, *domainerrors.AppError) {
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

func ParseEntityFile(path string) (*Entity, *domainerrors.AppError) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, domainerrors.New(
			domainerrors.CodeReadFailed,
			"failed to read workspace document",
			entitydoc.IOFailureDetails(err),
		)
	}

	document, parseErr := entitydoc.ParseDocument(raw)
	if parseErr != nil {
		return nil, diagnostics.NewReadError(
			"failed to parse workspace document",
			parseErr.Error(),
			diagnostics.FrontmatterStandardRef,
			nil,
		)
	}

	// The built-in fields are required for reading: without them the document
	// cannot be identified, so it is reported rather than silently skipped.
	if document.Type == "" {
		return nil, diagnostics.NewReadError(
			"failed to determine entity type",
			requiredBuiltinFieldMessage("type"),
			diagnostics.TypeStandardRef,
			nil,
		)
	}
	if document.ID == "" {
		return nil, diagnostics.NewReadError(
			"failed to determine entity id",
			requiredBuiltinFieldMessage("id"),
			diagnostics.IDStandardRef,
			nil,
		)
	}
	if document.Slug == "" {
		return nil, diagnostics.NewReadError(
			"failed to determine entity slug",
			requiredBuiltinFieldMessage("slug"),
			diagnostics.SlugStandardRef,
			nil,
		)
	}

	parsedSections, duplicateLabels := entitydoc.ExtractSections(document.Body)
	sections := make(map[string]string, len(parsedSections))
	for label, section := range parsedSections {
		sections[label] = section.Body
	}

	return &Entity{
		Type:                   document.Type,
		ID:                     document.ID,
		Slug:                   document.Slug,
		CreatedDate:            document.CreatedDate,
		UpdatedDate:            document.UpdatedDate,
		Revision:               document.Revision,
		Frontmatter:            document.Frontmatter,
		Sections:               sections,
		DuplicateSectionLabels: duplicateLabels,
		RawContent:             document.Body,
	}, nil
}

func requiredBuiltinFieldMessage(field string) string {
	return fmt.Sprintf("built-in field '%s' is required", field)
}
