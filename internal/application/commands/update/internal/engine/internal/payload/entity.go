package payload

import "github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/model"

func BuildEntity(typeSpec model.EntityTypeSpec, candidate *model.Candidate) map[string]any {
	refsPayload := map[string]any{}
	for _, fieldName := range typeSpec.MetaFieldOrder {
		field := typeSpec.MetaFields[fieldName]
		if !field.IsEntityRef {
			continue
		}
		idValue, exists := candidate.RefIDs[fieldName]
		if !exists || idValue == "" {
			continue
		}
		refsPayload[fieldName] = map[string]any{"id": idValue}
	}

	metaPayload := map[string]any{}
	for _, fieldName := range typeSpec.MetaFieldOrder {
		field := typeSpec.MetaFields[fieldName]
		if field.IsEntityRef {
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
