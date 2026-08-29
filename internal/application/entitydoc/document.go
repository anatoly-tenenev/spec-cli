package entitydoc

import (
	"crypto/sha256"
	"encoding/hex"
)

// Document is what a file says about the entity in it: the built-in fields,
// the frontmatter exactly as written, the body, and the revision computed from
// the bytes on disk.
//
// Values are not normalized here. add and update write documents back, and
// normalizing on the way in would reformat fields nobody touched, so callers
// that need normalized values apply it themselves.
//
// Nothing is required either. A missing built-in field arrives as an empty
// string, and what that means is the caller's decision: readmodel reports it as
// an error, add skips the document, get tolerates a missing slug. Holding that
// policy here would force one answer on all of them.
type Document struct {
	Type        string
	ID          string
	Slug        string
	CreatedDate string
	UpdatedDate string
	Revision    string
	Frontmatter map[string]any
	Body        string
}

// ParseDocument splits a document and pulls out the built-in fields. It fails
// only when the frontmatter cannot be read at all.
func ParseDocument(raw []byte) (Document, error) {
	frontmatter, body, err := ParseFrontmatter(raw)
	if err != nil {
		return Document{}, err
	}

	typeName, _ := ReadStringField(frontmatter, "type")
	id, _ := ReadStringField(frontmatter, "id")
	slug, _ := ReadStringField(frontmatter, "slug")
	createdDate, _ := ReadStringField(frontmatter, "createdDate")
	updatedDate, _ := ReadStringField(frontmatter, "updatedDate")

	return Document{
		Type:        typeName,
		ID:          id,
		Slug:        slug,
		CreatedDate: createdDate,
		UpdatedDate: updatedDate,
		Revision:    Revision(raw),
		Frontmatter: frontmatter,
		Body:        body,
	}, nil
}

// Revision identifies the exact bytes of a document, so a write can tell
// whether the file changed since it was read.
func Revision(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
