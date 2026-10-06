// payload.go builds the entity object add returns. It reports the logical
// model - meta, refs and sections - and never the path the document was
// written to, because storage layout is not part of what a caller may depend
// on.

package engine

import "github.com/anatoly-tenenev/spec-cli/internal/application/commands/add/internal/model"

func buildEntityPayload(typeSpec model.EntityTypeSpec, candidate *model.Candidate) map[string]any {
	refsPayload := map[string]any{}
	for _, fieldName := range typeSpec.MetaFieldOrder {
		field := typeSpec.MetaFields[fieldName]
		switch {
		case field.IsEntityRef:
			idValue, exists := candidate.RefIDs[fieldName]
			if !exists || idValue == "" {
				continue
			}
			refsPayload[fieldName] = map[string]any{"id": idValue}
		case field.IsEntityRefArray:
			refIDs, exists := candidate.RefIDArrays[fieldName]
			if !exists {
				continue
			}
			itemsPayload := make([]any, 0, len(refIDs))
			for _, refID := range refIDs {
				itemsPayload = append(itemsPayload, map[string]any{"id": refID})
			}
			refsPayload[fieldName] = itemsPayload
		}
	}

	sectionsPayload := map[string]any{}
	for name, value := range candidate.Sections {
		sectionsPayload[name] = value
	}

	metaPayload := map[string]any{}
	for _, fieldName := range typeSpec.MetaFieldOrder {
		field := typeSpec.MetaFields[fieldName]
		if field.IsEntityRef || field.IsEntityRefArray {
			continue
		}
		value, exists := candidate.Meta[fieldName]
		if !exists {
			continue
		}
		metaPayload[fieldName] = value
	}

	return map[string]any{
		"type":        candidate.Type,
		"id":          candidate.ID,
		"slug":        candidate.Slug,
		"revision":    candidate.Revision,
		"createdDate": candidate.CreatedDate,
		"updatedDate": candidate.UpdatedDate,
		"meta":        metaPayload,
		"refs":        refsPayload,
		"content": map[string]any{
			"raw":      candidate.Body,
			"sections": sectionsPayload,
		},
	}
}
