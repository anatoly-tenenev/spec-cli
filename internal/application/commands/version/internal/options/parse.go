// Package options parses the version command's arguments, which are none.
// It exists so that an unexpected argument is refused rather than ignored: a
// caller that passed something meant it, and silently dropping it would hide
// the mistake.
package options

import (
	"fmt"
	"github.com/anatoly-tenenev/spec-cli/internal/cliflags"
	"strings"

	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

type Parsed struct{}

func Parse(args []string) (Parsed, *domainerrors.AppError) {
	for _, token := range args {
		name, _, _ := cliflags.SplitLong(token)
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
