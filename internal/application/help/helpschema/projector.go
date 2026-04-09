package helpschema

import (
	"fmt"
	"sort"
	"strings"

	helpprojection "github.com/anatoly-tenenev/spec-cli/internal/application/help/helpschema/internal/projection"
	schemacapread "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/read"
	schemacapreferences "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/references"
	schemacapvalidate "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/validate"
	schemacapwrite "github.com/anatoly-tenenev/spec-cli/internal/application/schema/capabilities/write"
	schemacompile "github.com/anatoly-tenenev/spec-cli/internal/application/schema/compile"
	"github.com/anatoly-tenenev/spec-cli/internal/application/schema/model"
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

const schemaUnavailableImpact = "schema-derived entity types, read/write paths, enum values and specification projection are unavailable; do not infer these values heuristically"

type LoadedData struct {
	ProjectionJSON       string
	Catalog              Catalog
	ValidateEntityTypes  []string
	ReferencesInboundMap map[string][]InboundReferenceSlot
}

type Catalog struct {
	EntityTypes []EntityTypeModel
	ByType      map[string]EntityTypeModel
}

type EntityTypeModel struct {
	Name            string
	IDPrefix        string
	MetaFields      []MetaFieldModel
	ScalarRefFields []RefFieldModel
	ArrayRefFields  []RefFieldModel
	SectionNames    []string
}

type MetaFieldModel struct {
	Name       string
	Kind       string
	EnumValues []any
}

type RefFieldModel struct {
	Name         string
	AllowedTypes []string
}

type InboundReferenceSlot struct {
	SourceType  string
	FieldName   string
	Cardinality string
}

type Report struct {
	ResolvedPath  string
	Status        Status
	Loaded        *LoadedData
	ReasonCode    ReasonCode
	Impact        string
	RecoveryClass RecoveryClass
	RetryCommand  string
}

func LoadReport(schemaPath string, resolvedPath string) Report {
	compiler := schemacompile.NewCompiler()
	compileResult, compileErr := compiler.Compile(schemaPath, resolvedPath)
	if compileErr != nil {
		return degradedReport(resolvedPath, compileErr)
	}

	loaded, buildErr := buildLoadedData(compileResult.Schema)
	if buildErr != nil {
		return degradedReport(resolvedPath, domainerrors.New(
			domainerrors.CodeSchemaProjectionError,
			"failed to build help schema projection",
			nil,
		))
	}

	return Report{
		ResolvedPath: resolvedPath,
		Status:       StatusLoaded,
		Loaded:       loaded,
	}
}

func buildLoadedData(compiled model.CompiledSchema) (*LoadedData, error) {
	readCapability := schemacapread.Build(compiled)
	writeCapability := schemacapwrite.Build(compiled)
	validateCapability := schemacapvalidate.Build(compiled)
	referencesCapability := schemacapreferences.Build(compiled)

	projectionJSON, err := renderSpecificationProjection(compiled)
	if err != nil {
		return nil, err
	}

	catalog := buildCatalog(readCapability, writeCapability)

	inbound := map[string][]InboundReferenceSlot{}
	for targetType, slots := range referencesCapability.InboundByTargetType {
		projected := make([]InboundReferenceSlot, 0, len(slots))
		for _, slot := range slots {
			projected = append(projected, InboundReferenceSlot{
				SourceType:  slot.SourceType,
				FieldName:   slot.FieldName,
				Cardinality: string(slot.Cardinality),
			})
		}
		inbound[targetType] = projected
	}

	return &LoadedData{
		ProjectionJSON:       projectionJSON,
		Catalog:              catalog,
		ValidateEntityTypes:  append([]string(nil), validateCapability.EntityOrder...),
		ReferencesInboundMap: inbound,
	}, nil
}

func buildCatalog(
	readCapability schemacapread.Capability,
	writeCapability schemacapwrite.Capability,
) Catalog {
	typeNames := sortedEntityTypeNames(writeCapability.EntityTypes)
	entities := make([]EntityTypeModel, 0, len(typeNames))
	byType := make(map[string]EntityTypeModel, len(typeNames))

	for _, typeName := range typeNames {
		writeModel, writeExists := writeCapability.EntityTypes[typeName]
		readModel, readExists := readCapability.EntityTypes[typeName]
		if !writeExists || !readExists {
			continue
		}

		metaFieldNames := sortedMetaFieldNames(readModel.MetaFields)
		metaFields := make([]MetaFieldModel, 0, len(metaFieldNames))
		for _, fieldName := range metaFieldNames {
			field := readModel.MetaFields[fieldName]
			enumValues := make([]any, 0, len(field.EnumValues))
			enumValues = append(enumValues, field.EnumValues...)
			metaFields = append(metaFields, MetaFieldModel{
				Name:       fieldName,
				Kind:       string(field.Kind),
				EnumValues: enumValues,
			})
		}

		refFieldNames := sortedRefFieldNames(readModel.RefFields)
		scalarRefs := make([]RefFieldModel, 0, len(refFieldNames))
		arrayRefs := make([]RefFieldModel, 0, len(refFieldNames))
		for _, fieldName := range refFieldNames {
			writeField, writable := writeModel.MetaFields[fieldName]
			if !writable || (!writeField.IsEntityRef && !writeField.IsEntityRefArray) {
				continue
			}

			ref := RefFieldModel{Name: fieldName}
			if writeField.IsEntityRefArray {
				ref.AllowedTypes = append([]string(nil), writeField.ItemRefTypes...)
				arrayRefs = append(arrayRefs, ref)
				continue
			}
			ref.AllowedTypes = append([]string(nil), writeField.RefTypes...)
			scalarRefs = append(scalarRefs, ref)
		}

		sectionNames := append([]string(nil), writeModel.SectionOrder...)
		if len(sectionNames) == 0 {
			for name := range writeModel.Sections {
				sectionNames = append(sectionNames, name)
			}
			sort.Strings(sectionNames)
		}

		entity := EntityTypeModel{
			Name:            typeName,
			IDPrefix:        writeModel.IDPrefix,
			MetaFields:      metaFields,
			ScalarRefFields: scalarRefs,
			ArrayRefFields:  arrayRefs,
			SectionNames:    sectionNames,
		}
		entities = append(entities, entity)
		byType[typeName] = entity
	}

	return Catalog{
		EntityTypes: entities,
		ByType:      byType,
	}
}

func renderSpecificationProjection(compiled model.CompiledSchema) (string, error) {
	return helpprojection.Render(compiled)
}

func sortedEntityTypeNames(values map[string]schemacapwrite.EntityWriteModel) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedMetaFieldNames(values map[string]schemacapread.MetaField) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedRefFieldNames(values map[string]schemacapread.RefField) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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
		report.RetryCommand = retrySchemaCheckCommand(resolvedPath)
	case domainerrors.CodeSchemaInvalid:
		report.Status = StatusInvalid
		report.ReasonCode = ReasonSchemaValidation
		report.RecoveryClass = RecoveryFixSchemaFile
		report.RetryCommand = retrySchemaCheckCommand(resolvedPath)
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

func retrySchemaCheckCommand(path string) string {
	schemaPath := strings.TrimSpace(path)
	if schemaPath == "" {
		schemaPath = "./spec.schema.yaml"
	}
	return fmt.Sprintf("spec-cli schema check --schema %s", schemaPath)
}
