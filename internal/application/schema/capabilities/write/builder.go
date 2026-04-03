package write

import (
	"sort"

	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
)

type Capability struct {
	EntityTypes map[string]EntityWriteModel
}

type EntityWriteModel struct {
	SetPaths     []string
	UnsetPaths   []string
	SetFilePaths []string
}

func Build(compiled model.CompiledSchema) Capability {
	result := Capability{EntityTypes: make(map[string]EntityWriteModel, len(compiled.Entities))}

	for typeName, entity := range compiled.Entities {
		setPaths := make([]string, 0)
		unsetPaths := make([]string, 0)
		setFilePaths := make([]string, 0)

		for fieldName, field := range entity.MetaFields {
			path := "meta." + fieldName
			if field.Value.Ref != nil || (field.Value.Kind == model.ValueKindArray && field.Value.Items != nil && field.Value.Items.Ref != nil) {
				path = "refs." + fieldName
			}
			setPaths = append(setPaths, path)
			unsetPaths = append(unsetPaths, path)
		}

		for sectionName := range entity.Sections {
			path := "content.sections." + sectionName
			setPaths = append(setPaths, path)
			unsetPaths = append(unsetPaths, path)
			setFilePaths = append(setFilePaths, path)
		}

		setPaths = dedupeSorted(setPaths)
		unsetPaths = dedupeSorted(unsetPaths)
		setFilePaths = dedupeSorted(setFilePaths)

		result.EntityTypes[typeName] = EntityWriteModel{
			SetPaths:     setPaths,
			UnsetPaths:   unsetPaths,
			SetFilePaths: setFilePaths,
		}
	}

	return result
}

func dedupeSorted(values []string) []string {
	if len(values) == 0 {
		return values
	}
	sort.Strings(values)
	result := make([]string, 0, len(values))
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}
