// Package errormap translates a domain error code into the result_state a
// caller sees. It keeps that mapping out of the individual commands so the
// same code cannot be reported as two different states.
package errormap

import (
	"github.com/anatoly-tenenev/spec-cli/internal/contracts/responses"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

func ResultStateForCode(code domainerrors.Code) responses.ResultState {
	switch code {
	case domainerrors.CodeEntityNotFound:
		return responses.ResultStateNotFound
	case domainerrors.CodeCapabilityUnsupported, domainerrors.CodeNotImplemented:
		return responses.ResultStateUnsupported
	case domainerrors.CodeInternalError:
		return responses.ResultStateIndeterminate
	default:
		return responses.ResultStateInvalid
	}
}
