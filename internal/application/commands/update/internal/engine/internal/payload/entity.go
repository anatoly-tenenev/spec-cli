package payload

import "github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/model"

func BuildEntity(typeSpec model.EntityTypeSpec, candidate *model.Candidate) map[string]any {
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
		"type":         candidate.Type,
		"id":           candidate.ID,
		"slug":         candidate.Slug,
		"revision":     candidate.Revision,
		"created_date": candidate.CreatedDate,
		"updated_date": candidate.UpdatedDate,
		"meta":         metaPayload,
		"refs":         refsPayload,
	}
}
