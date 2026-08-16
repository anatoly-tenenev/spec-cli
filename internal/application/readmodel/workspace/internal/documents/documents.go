package documents

import (
	"crypto/sha256"
	"encoding/hex"
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

	frontmatter, body, parseErr := entitydoc.ParseFrontmatter(raw)
	if parseErr != nil {
		return nil, diagnostics.NewReadError(
			"failed to parse workspace document",
			parseErr.Error(),
			diagnostics.FrontmatterStandardRef,
			nil,
		)
	}

	typeName, ok := entitydoc.ReadStringField(frontmatter, "type")
	if !ok {
		return nil, diagnostics.NewReadError(
			"failed to determine entity type",
			requiredBuiltinFieldMessage("type"),
			diagnostics.TypeStandardRef,
			nil,
		)
	}
	id, ok := entitydoc.ReadStringField(frontmatter, "id")
	if !ok {
		return nil, diagnostics.NewReadError(
			"failed to determine entity id",
			requiredBuiltinFieldMessage("id"),
			diagnostics.IDStandardRef,
			nil,
		)
	}
	slug, ok := entitydoc.ReadStringField(frontmatter, "slug")
	if !ok {
		return nil, diagnostics.NewReadError(
			"failed to determine entity slug",
			requiredBuiltinFieldMessage("slug"),
			diagnostics.SlugStandardRef,
			nil,
		)
	}

	createdDate, _ := entitydoc.ReadStringField(frontmatter, "createdDate")
	updatedDate, _ := entitydoc.ReadStringField(frontmatter, "updatedDate")

	revisionHash := sha256.Sum256(raw)
	revision := "sha256:" + hex.EncodeToString(revisionHash[:])

	parsedSections, duplicateLabels := entitydoc.ExtractSections(body)
	sections := make(map[string]string, len(parsedSections))
	for label, section := range parsedSections {
		sections[label] = section.Body
	}

	return &Entity{
		Type:                   typeName,
		ID:                     id,
		Slug:                   slug,
		CreatedDate:            createdDate,
		UpdatedDate:            updatedDate,
		Revision:               revision,
		Frontmatter:            frontmatter,
		Sections:               sections,
		DuplicateSectionLabels: duplicateLabels,
		RawContent:             body,
	}, nil
}

func requiredBuiltinFieldMessage(field string) string {
	return fmt.Sprintf("built-in field '%s' is required", field)
}
