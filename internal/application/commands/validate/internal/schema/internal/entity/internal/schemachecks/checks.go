package schemachecks

import (
	"fmt"

	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func EnsureOnlyKeys(path string, values map[string]any, allowed ...string) *domainerrors.AppError {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		allowedSet[key] = struct{}{}
	}

	for key := range values {
		if _, ok := allowedSet[key]; ok {
			continue
		}

		return domainerrors.New(
			domainerrors.CodeSchemaInvalid,
			fmt.Sprintf("%s has unsupported key '%s'", path, key),
			nil,
		)
	}
	return nil
}
