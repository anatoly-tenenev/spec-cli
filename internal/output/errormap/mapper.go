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
