package options

import (
	"fmt"
	"strings"

	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

type Parsed struct{}

func Parse(args []string) (Parsed, *domainerrors.AppError) {
	for _, token := range args {
		name, _, _ := splitLongFlag(token)
		if !strings.HasPrefix(name, "--") {
			return Parsed{}, domainerrors.New(
				domainerrors.CodeInvalidArgs,
				fmt.Sprintf("unknown version option: %s", token),
				nil,
			)
		}
		return Parsed{}, domainerrors.New(
			domainerrors.CodeInvalidArgs,
			fmt.Sprintf("unknown version option: %s", name),
			nil,
		)
	}

	return Parsed{}, nil
}

func splitLongFlag(token string) (string, string, bool) {
	parts := strings.SplitN(token, "=", 2)
	if len(parts) == 1 {
		return parts[0], "", false
	}
	return parts[0], parts[1], true
}
