package lookup

import (
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/commands/update/internal/model"
)

type Candidate struct {
	Candidate *model.Candidate
}

func (l Candidate) Lookup(pathValue string) (any, bool) {
	pathValue = strings.TrimSpace(pathValue)
	if pathValue == "" || l.Candidate == nil {
		return nil, false
	}

	switch pathValue {
	case "type":
		return l.Candidate.Type, true
	case "id":
		return l.Candidate.ID, true
	case "slug":
		return l.Candidate.Slug, true
	case "createdDate":
		return l.Candidate.CreatedDate, true
	case "updatedDate":
		return l.Candidate.UpdatedDate, true
	}

	if strings.HasPrefix(pathValue, "meta.") {
		suffix := strings.TrimPrefix(pathValue, "meta.")
		parts := strings.Split(suffix, ".")
		value, exists := l.Candidate.Frontmatter[parts[0]]
		if !exists {
			return nil, false
		}
		if len(parts) == 1 {
			return value, true
		}
		return lookupNested(value, parts[1:])
	}

	if strings.HasPrefix(pathValue, "refs.") {
		suffix := strings.TrimPrefix(pathValue, "refs.")
		parts := strings.Split(suffix, ".")
		if len(parts) < 1 {
			return nil, false
		}
		ref, exists := l.Candidate.Refs[parts[0]]
		if exists {
			if len(parts) == 1 {
				return map[string]any{
					"type":     ref.Type,
					"id":       ref.ID,
					"slug":     ref.Slug,
					"dirPath": ref.DirPath,
				}, true
			}

			switch parts[1] {
			case "type":
				if len(parts) == 2 {
					return ref.Type, true
				}
			case "id":
				if len(parts) == 2 {
					return ref.ID, true
				}
			case "slug":
				if len(parts) == 2 {
					return ref.Slug, true
				}
			case "dirPath":
				if len(parts) == 2 {
					return ref.DirPath, true
				}
			default:
				value, exists := ref.Meta[parts[1]]
				if !exists {
					return nil, false
				}
				if len(parts) == 2 {
					return value, true
				}
				return lookupNested(value, parts[2:])
			}
		}

		refArray, exists := l.Candidate.RefArrays[parts[0]]
		if !exists {
			return nil, false
		}
		if len(parts) != 1 {
			return nil, false
		}
		values := make([]any, 0, len(refArray))
		for _, item := range refArray {
			values = append(values, map[string]any{
				"type":     item.Type,
				"id":       item.ID,
				"slug":     item.Slug,
				"dirPath": item.DirPath,
			})
		}
		return values, true
	}

	return nil, false
}

func lookupNested(value any, parts []string) (any, bool) {
	current := value
	for _, part := range parts {
		mapValue, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		next, exists := mapValue[part]
		if !exists {
			return nil, false
		}
		current = next
	}
	return current, true
}
