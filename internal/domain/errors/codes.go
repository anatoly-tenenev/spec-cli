package errors

type Code string

const (
	CodeInvalidArgs      Code = "INVALID_ARGS"
	CodeSchemaNotFound   Code = "SCHEMA_NOT_FOUND"
	CodeSchemaParseError Code = "SCHEMA_PARSE_ERROR"
	CodeSchemaInvalid    Code = "SCHEMA_INVALID"

	CodeEntityTypeUnknown      Code = "ENTITY_TYPE_UNKNOWN"
	CodeEntityNotFound         Code = "ENTITY_NOT_FOUND"
	CodeTargetAmbiguous        Code = "TARGET_AMBIGUOUS"
	CodePathConflict           Code = "PATH_CONFLICT"
	CodeIDConflict             Code = "ID_CONFLICT"
	CodeSlugConflict           Code = "SLUG_CONFLICT"
	CodeWriteContractViolation Code = "WRITE_CONTRACT_VIOLATION"
	CodeValidationFailed       Code = "VALIDATION_FAILED"
	CodeConcurrencyConflict    Code = "CONCURRENCY_CONFLICT"
	CodeInvalidQuery           Code = "INVALID_QUERY"
	CodeReadFailed             Code = "READ_FAILED"
	CodeWriteFailed            Code = "WRITE_FAILED"
	CodeInternalError          Code = "INTERNAL_ERROR"

	CodeNotImplemented Code = "NOT_IMPLEMENTED"
)

type AppError struct {
	Code     Code
	Message  string
	Details  map[string]any
	ExitCode int
}

func (e *AppError) Error() string {
	return e.Message
}

func New(code Code, message string, details map[string]any) *AppError {
	return &AppError{
		Code:     code,
		Message:  message,
		Details:  details,
		ExitCode: ExitCodeFor(code),
	}
}

func ExitCodeFor(code Code) int {
	switch code {
	case CodeInvalidArgs, CodeInvalidQuery:
		return 2
	case CodeEntityTypeUnknown:
		return 2
	case CodeReadFailed, CodeWriteFailed:
		return 3
	case CodeSchemaNotFound, CodeSchemaParseError, CodeSchemaInvalid:
		return 4
	case CodeInternalError:
		return 5
	default:
		return 1
	}
}
