package views

import (
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/internal/ordered"
	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/internal/values"
	schemacapread "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/read"
)

func BuildMetadata(frontmatter map[string]any, knownMeta map[string]schemacapread.MetaField) map[string]any {
	meta := map[string]any{}
	for _, field := range ordered.MapKeys(knownMeta) {
		value, exists := frontmatter[field]
		if !exists {
			continue
		}
		meta[field] = NormalizeValue(value)
	}
	return meta
}

func BuildWhereSections(parsedSections map[string]string, knownSections map[string]schemacapread.Section) map[string]any {
	sections := map[string]any{}
	for _, sectionName := range ordered.MapKeys(knownSections) {
		sectionValue, exists := parsedSections[sectionName]
		if !exists {
			continue
		}
		sections[sectionName] = sectionValue
	}
	return sections
}

func NormalizeValue(value any) any {
	if number, ok := values.NumberToFloat64(value); ok {
		return number
	}

	switch typed := value.(type) {
	case time.Time:
		return typed.Format("2006-01-02")
	case map[string]any:
		normalized := make(map[string]any, len(typed))
		for key, item := range typed {
			normalized[key] = NormalizeValue(item)
		}
		return normalized
	case []any:
		normalized := make([]any, len(typed))
		for idx := range typed {
			normalized[idx] = NormalizeValue(typed[idx])
		}
		return normalized
	default:
		return typed
	}
}

func SectionsToAnyMap(sections map[string]string) map[string]any {
	mapped := make(map[string]any, len(sections))
	for name, value := range sections {
		mapped[name] = value
	}
	return mapped
}
