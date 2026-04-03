package source

import (
	"fmt"
	"os"
	"strings"

	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/diagnostics"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/source/internal/yamlnodes"
	"gopkg.in/yaml.v3"
)

type Document struct {
	Path        string
	DisplayPath string
	Raw         []byte
	Root        *yaml.Node
}

func Load(path string, displayPath string) (Document, []diagnostics.Issue) {
	display := strings.TrimSpace(displayPath)
	if display == "" {
		display = strings.TrimSpace(path)
	}

	raw, readErr := os.ReadFile(path)
	if readErr != nil {
		return Document{}, []diagnostics.Issue{
			diagnostics.NewError(
				"schema.source.read_failed",
				fmt.Sprintf("schema file is not readable: %v", replacePathInReason(readErr.Error(), path, display)),
				"schema",
			),
		}
	}

	if len(strings.TrimSpace(string(raw))) == 0 {
		return Document{}, []diagnostics.Issue{
			diagnostics.NewError(
				"schema.source.empty",
				"schema file is empty",
				"schema",
			),
		}
	}

	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return Document{}, []diagnostics.Issue{
			diagnostics.NewError(
				"schema.source.parse_failed",
				fmt.Sprintf("failed to parse schema yaml/json: %v", err),
				"schema",
			),
		}
	}

	doc := yamlnodes.FirstContentNode(&root)
	if doc == nil || doc.Kind != yaml.MappingNode {
		return Document{}, []diagnostics.Issue{
			diagnostics.NewError(
				"schema.source.root_not_mapping",
				"schema root must be a mapping object",
				"schema",
			),
		}
	}

	if duplicate, ok := yamlnodes.FindDuplicateMappingKey(doc); ok {
		return Document{}, []diagnostics.Issue{
			diagnostics.NewError(
				"schema.source.duplicate_key",
				fmt.Sprintf("schema contains duplicate key '%s'", duplicate.Key),
				duplicate.Path,
			),
		}
	}

	return Document{
		Path:        path,
		DisplayPath: display,
		Raw:         raw,
		Root:        doc,
	}, nil
}

func replacePathInReason(reason string, absolutePath string, displayPath string) string {
	absolute := strings.TrimSpace(absolutePath)
	display := strings.TrimSpace(displayPath)
	if absolute == "" || display == "" {
		return reason
	}
	return strings.Replace(reason, absolute, display, 1)
}
