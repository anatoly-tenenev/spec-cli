package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"os"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/get/internal/issuedetails"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/get/internal/model"
	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/internal/values"
	"github.com/anatoly-tenenev/spec-cli/internal/application/entitydoc"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

const (
	getWorkspaceFrontmatterStandardRef = "11"
	getWorkspaceTypeStandardRef        = "5.3"
	getWorkspaceIDStandardRef          = "11.1"
)

type locatedCandidate struct {
	path string
	raw  []byte
}

func LocateByID(workspacePath string, targetID string) (model.LocateResult, *domainerrors.AppError) {
	files, scanErr := scanMarkdownFiles(workspacePath)
	if scanErr != nil {
		return model.LocateResult{}, scanErr
	}

	identityIndex := map[string][]model.EntityIdentity{}
	matches := make([]locatedCandidate, 0, 2)

	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			return model.LocateResult{}, domainerrors.New(
				domainerrors.CodeReadFailed,
				"failed to read workspace document",
				entitydoc.IOFailureDetails(err),
			)
		}

		// Locating stays tolerant of malformed frontmatter so that a broken
		// target is reported as broken rather than masked as NOT_FOUND.
		if extractedID, ok := entitydoc.ExtractIDLenient(raw); ok && extractedID == targetID {
			matches = append(matches, locatedCandidate{path: path, raw: raw})
		}

		if identity, ok := extractIdentity(raw); ok {
			identityIndex[identity.ID] = append(identityIndex[identity.ID], identity)
		}
	}

	switch len(matches) {
	case 0:
		return model.LocateResult{}, domainerrors.New(
			domainerrors.CodeEntityNotFound,
			"target entity is not found",
			map[string]any{"id": targetID},
		)
	case 1:
		return model.LocateResult{
			TargetPath:    matches[0].path,
			TargetRaw:     matches[0].raw,
			IdentityIndex: identityIndex,
		}, nil
	default:
		return model.LocateResult{}, domainerrors.New(
			domainerrors.CodeTargetAmbiguous,
			"target id is ambiguous",
			map[string]any{"id": targetID, "matches": len(matches)},
		)
	}
}

func ReadTarget(path string, raw []byte, requestedID string) (model.ParsedTarget, *domainerrors.AppError) {
	frontmatter, body, parseErr := entitydoc.ParseFrontmatter(raw)
	if parseErr != nil {
		return model.ParsedTarget{}, newReadError(
			"failed to parse target frontmatter",
			parseErr.Error(),
			getWorkspaceFrontmatterStandardRef,
			nil,
		)
	}

	typeName, ok := entitydoc.ReadStringField(frontmatter, "type")
	if !ok {
		return model.ParsedTarget{}, newReadError(
			"failed to determine entity type",
			"built-in field 'type' is required",
			getWorkspaceTypeStandardRef,
			nil,
		)
	}

	entityID, ok := entitydoc.ReadStringField(frontmatter, "id")
	if !ok {
		return model.ParsedTarget{}, newReadError(
			"failed to determine entity id",
			"built-in field 'id' is required",
			getWorkspaceIDStandardRef,
			nil,
		)
	}

	if entityID != requestedID {
		return model.ParsedTarget{}, newReadError(
			"failed to read target entity",
			"target document id does not match requested --id",
			getWorkspaceIDStandardRef,
			map[string]any{"expected_id": requestedID, "actual_id": entityID},
		)
	}

	slug, _ := entitydoc.ReadStringField(frontmatter, "slug")
	createdDate, _ := entitydoc.ReadStringField(frontmatter, "createdDate")
	updatedDate, _ := entitydoc.ReadStringField(frontmatter, "updatedDate")

	revisionHash := sha256.Sum256(raw)
	revision := "sha256:" + hex.EncodeToString(revisionHash[:])
	sections, duplicateLabels := extractSections(body)

	return model.ParsedTarget{
		Path:                   path,
		Type:                   typeName,
		ID:                     entityID,
		Slug:                   slug,
		CreatedDate:            createdDate,
		UpdatedDate:            updatedDate,
		Revision:               revision,
		RawBody:                body,
		Frontmatter:            normalizeMap(frontmatter),
		Sections:               sections,
		DuplicateSectionLabels: duplicateLabels,
	}, nil
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

func extractIdentity(raw []byte) (model.EntityIdentity, bool) {
	frontmatter, _, err := entitydoc.ParseFrontmatter(raw)
	if err != nil {
		return model.EntityIdentity{}, false
	}

	typeName, ok := entitydoc.ReadStringField(frontmatter, "type")
	if !ok {
		return model.EntityIdentity{}, false
	}
	id, ok := entitydoc.ReadStringField(frontmatter, "id")
	if !ok {
		return model.EntityIdentity{}, false
	}
	slug, ok := entitydoc.ReadStringField(frontmatter, "slug")
	if !ok {
		return model.EntityIdentity{}, false
	}

	return model.EntityIdentity{Type: typeName, ID: id, Slug: slug}, true
}

func normalizeMap(input map[string]any) map[string]any {
	normalized := make(map[string]any, len(input))
	for key, value := range input {
		normalized[key] = values.NormalizeValue(value)
	}
	return normalized
}

// extractSections returns unambiguous sections plus a count per repeated label.
func extractSections(body string) (map[string]string, map[string]int) {
	layout := entitydoc.BuildSectionLayout(body)

	duplicates := map[string]int{}
	for label, count := range layout.LabelCount {
		if count > 1 {
			duplicates[label] = count
		}
	}

	sections := map[string]string{}
	for _, item := range layout.Ranges {
		if layout.LabelCount[item.Label] > 1 {
			continue
		}
		sections[item.Label] = layout.Body(item)
	}
	return sections, duplicates
}

func newReadError(message string, issueMessage string, standardRef string, details map[string]any) *domainerrors.AppError {
	issue := issuedetails.ValidationIssue("error", "InstanceError", issueMessage, standardRef)
	return domainerrors.New(domainerrors.CodeReadFailed, message, issuedetails.WithValidationIssues(details, issue))
}
