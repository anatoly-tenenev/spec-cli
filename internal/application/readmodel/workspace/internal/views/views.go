// Package views narrows a parsed document to what the schema declares: only
// known meta fields and known sections reach the entity view.
//
// It builds two projections of the same fields, because they answer to
// different authorities. What a filter sees follows JMESPath: every number is a
// float64, so a comparison here behaves the way the same comparison behaves in
// any JMESPath engine. What the response returns follows the document: a number
// is returned as it was written, so an integer too large for a float64 comes
// back intact rather than rounded to the nearest representable one.
//
// The two therefore disagree about integers beyond 2^53, and deliberately so:
// filtering on such a value is approximate in JMESPath no matter what we do,
// while returning it is not allowed to be.
package views

import (
	"time"

	"github.com/anatoly-tenenev/spec-cli/internal/application/readmodel/internal/ordered"
	schemacapread "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/read"
	"github.com/anatoly-tenenev/spec-cli/internal/application/values"
)

// BuildMetadata projects the meta fields the response returns. Values keep the
// type the document gave them, so the number a read command returns is the
// number the document holds - the same answer get gives for the same field.
func BuildMetadata(frontmatter map[string]any, knownMeta map[string]schemacapread.MetaField) map[string]any {
	return buildMetadata(frontmatter, knownMeta, values.NormalizeValue)
}

// BuildWhereMetadata projects the meta fields a filter is evaluated against,
// with every number widened to float64 as JMESPath requires.
func BuildWhereMetadata(frontmatter map[string]any, knownMeta map[string]schemacapread.MetaField) map[string]any {
	return buildMetadata(frontmatter, knownMeta, normalizeForFilter)
}

func buildMetadata(
	frontmatter map[string]any,
	knownMeta map[string]schemacapread.MetaField,
	normalize func(any) any,
) map[string]any {
	meta := map[string]any{}
	for _, field := range ordered.MapKeys(knownMeta) {
		value, exists := frontmatter[field]
		if !exists {
			continue
		}
		meta[field] = normalize(value)
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

func normalizeForFilter(value any) any {
	if number, ok := values.NumberToFloat64(value); ok {
		return number
	}

	switch typed := value.(type) {
	case time.Time:
		return typed.Format("2006-01-02")
	case map[string]any:
		normalized := make(map[string]any, len(typed))
		for key, item := range typed {
			normalized[key] = normalizeForFilter(item)
		}
		return normalized
	case []any:
		normalized := make([]any, len(typed))
		for idx := range typed {
			normalized[idx] = normalizeForFilter(typed[idx])
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
