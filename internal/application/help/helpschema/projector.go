package helpschema

import (
	"fmt"
	"strings"

	projector "github.com/anatoly-tenenev/spec-cli/internal/application/help/helpschema/internal/projector"
	domainerrors "github.com/anatoly-tenenev/spec-cli/internal/domain/errors"
)

type Status string

const (
	StatusLoaded  Status = "loaded"
	StatusMissing Status = "missing"
	StatusInvalid Status = "invalid"
	StatusError   Status = "error"
)

type ReasonCode string

const (
	ReasonSchemaNotFound    ReasonCode = "SCHEMA_NOT_FOUND"
	ReasonSchemaNotReadable ReasonCode = "SCHEMA_NOT_READABLE"
	ReasonSchemaParseError  ReasonCode = "SCHEMA_PARSE_ERROR"
	ReasonSchemaValidation  ReasonCode = "SCHEMA_VALIDATION_ERROR"
	ReasonSchemaProjection  ReasonCode = "SCHEMA_PROJECTION_ERROR"
	RetryCommandNone                   = "none"
	RetryCommandSchemaHelp             = "spec-cli --schema <path> help"
)

type RecoveryClass string

const (
	RecoveryProvideExplicitSchema RecoveryClass = "provide_explicit_schema"
	RecoveryFixSchemaFile         RecoveryClass = "fix_schema_file"
	RecoveryFixPathOrPermissions  RecoveryClass = "fix_path_or_permissions"
	RecoveryStopAndReport         RecoveryClass = "stop_and_report"
)

const schemaUnavailableImpact = "schema-derived entity types, namespace paths, enum values and CLI projection of schema-derived fields are unavailable; do not infer these values heuristically"

type Report struct {
	ResolvedPath   string
	Status         Status
	ProjectionYAML string
	ReasonCode     ReasonCode
	Impact         string
	RecoveryClass  RecoveryClass
	RetryCommand   string
}

func LoadReport(schemaPath string, resolvedPath string) Report {
	projection, err := projector.LoadProjection(schemaPath, resolvedPath)
	if err != nil {
		return degradedReport(resolvedPath, err)
	}
	return Report{
		ResolvedPath:   resolvedPath,
		Status:         StatusLoaded,
		ProjectionYAML: projection.ProjectionYAML,
	}
}

func degradedReport(resolvedPath string, appErr *domainerrors.AppError) Report {
	report := Report{
		ResolvedPath: resolvedPath,
		Impact:       schemaUnavailableImpact,
		RetryCommand: RetryCommandNone,
	}

	switch appErr.Code {
	case domainerrors.CodeSchemaNotFound:
		report.Status = StatusMissing
		report.ReasonCode = ReasonSchemaNotFound
		report.RecoveryClass = RecoveryProvideExplicitSchema
		report.RetryCommand = RetryCommandSchemaHelp
	case domainerrors.CodeSchemaReadError:
		report.Status = StatusError
		report.ReasonCode = ReasonSchemaNotReadable
		report.RecoveryClass = RecoveryFixPathOrPermissions
		report.RetryCommand = RetryCommandSchemaHelp
	case domainerrors.CodeSchemaParseError:
		report.Status = StatusInvalid
		report.ReasonCode = ReasonSchemaParseError
		report.RecoveryClass = RecoveryFixSchemaFile
		report.RetryCommand = retryValidateCommand(resolvedPath)
	case domainerrors.CodeSchemaInvalid:
		report.Status = StatusInvalid
		report.ReasonCode = ReasonSchemaValidation
		report.RecoveryClass = RecoveryFixSchemaFile
		report.RetryCommand = retryValidateCommand(resolvedPath)
	case domainerrors.CodeSchemaProjectionError:
		report.Status = StatusError
		report.ReasonCode = ReasonSchemaProjection
		report.RecoveryClass = RecoveryStopAndReport
	default:
		report.Status = StatusError
		report.ReasonCode = ReasonSchemaProjection
		report.RecoveryClass = RecoveryStopAndReport
	}

	return report
}

func retryValidateCommand(path string) string {
	schemaPath := strings.TrimSpace(path)
	if schemaPath == "" {
		schemaPath = "./spec.schema.yaml"
	}
	return fmt.Sprintf("spec-cli validate --schema %s", schemaPath)
}
