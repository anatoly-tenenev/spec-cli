package document

import (
	"encoding/json"
	"io"
	"os"

	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

type Request struct {
	Query         string
	File          string
	VariablesJSON string
	VariablesFile string
}

type Loaded struct {
	Query     string
	Variables map[string]any
}

func Load(request Request) (Loaded, *domainerrors.AppError) {
	queryText := request.Query
	if request.File != "" {
		raw, err := readInput(request.File)
		if err != nil {
			return Loaded{}, domainerrors.New(domainerrors.CodeReadFailed, "failed to read GraphQL query document", map[string]any{"reason": err.Error()})
		}
		queryText = string(raw)
	}
	vars := map[string]any{}
	if request.VariablesJSON != "" || request.VariablesFile != "" {
		raw := []byte(request.VariablesJSON)
		if request.VariablesFile != "" {
			data, err := readInput(request.VariablesFile)
			if err != nil {
				return Loaded{}, domainerrors.New(domainerrors.CodeReadFailed, "failed to read GraphQL variables", map[string]any{"reason": err.Error()})
			}
			raw = data
		}
		parsed, err := parseVariables(raw)
		if err != nil {
			return Loaded{}, domainerrors.New(domainerrors.CodeInvalidQuery, "GraphQL variables must be a JSON object", map[string]any{"reason": err.Error()})
		}
		vars = parsed
	}
	return Loaded{Query: queryText, Variables: vars}, nil
}

func readInput(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

func parseVariables(raw []byte) (map[string]any, error) {
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, err
	}
	obj, ok := decoded.(map[string]any)
	if !ok {
		return nil, errNotObject{}
	}
	return obj, nil
}

type errNotObject struct{}

func (errNotObject) Error() string {
	return "variables JSON root is not an object"
}
