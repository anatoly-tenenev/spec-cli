# Codebase Index (Agent Map)

Compact project map for fast entry into the code.

## Freshness Rule

- Any change to directory structure, entrypoint files, layer roles, or the supported command set must update this file in the same change.

## Fast CLI Execution Path

1. `internal/cli/app.go` - `NewApp`, `(*App).Run`.
2. `internal/application/commandbus/bus.go` - `(*Bus).Dispatch`.
3. `internal/application/commands/<command>/handler.go` - `(*Handler).Handle`.
4. For `help`: `options.Parse` -> `options.NormalizePaths` (canonical `ResolvedPath`) -> `helpschema.LoadReport` -> `helptext.RenderGeneral|RenderCommand`.
5. For `schema check`: `options.Parse` -> `options.NormalizeSchemaPath` -> `schema/compile.(*Compiler).Compile` -> on compile failure build top-level `error + schema`, otherwise return success `schema` block.
6. For `validate`: `options.Parse` -> `options.NormalizePaths` -> `schema/compile.(*Compiler).Compile` -> on compile failure build top-level `error + schema` with zero runtime summary/issues -> otherwise `schema/capabilities/validate.Build` -> `workspace.BuildCandidateSet` -> `engine.RunValidation`.
7. For `query`: `options.Parse` -> `options.NormalizePaths` -> `schema/compile.(*Compiler).Compile` -> `schema/capabilities/read.Build` -> `engine.BuildPlan` -> `workspace.LoadEntities` -> `engine.Execute` (compile failures stop before plan/workspace and return top-level `error + schema`).
8. For `get`: `options.Parse` -> `options.NormalizePaths` -> `schema/compile.(*Compiler).Compile` -> `schema/capabilities/read.Build` -> `engine.BuildSelectorPlan` -> `workspace.LocateByID` -> `workspace.ReadTarget` -> `engine.BuildEntityView` -> `engine.ProjectEntity` (compile failures stop before selector/workspace pipeline and return top-level `error + schema`).
9. For `add`: `options.Parse` -> `options.NormalizePaths` -> `workspacelock.AcquireExclusive` -> `schema/compile.(*Compiler).Compile` -> `schema/capabilities/write.Build` -> unknown type check -> `workspace.BuildSnapshot` -> `engine.Execute` (compile failures stop before snapshot/execution and every post-compile response includes top-level `schema`).
10. For `update`: `options.Parse` -> `options.NormalizePaths` -> `workspacelock.AcquireExclusive` -> `schema/compile.(*Compiler).Compile` -> `schema/capabilities/write.Build` -> `workspace.BuildSnapshot` -> `engine.Execute` (compile failures stop before snapshot/execution and every post-compile response includes top-level `schema`).
11. For `delete`: `options.Parse` -> `options.NormalizePaths` -> `workspacelock.AcquireExclusive` -> `schema/compile.(*Compiler).Compile` -> `schema/capabilities/references.Build` -> `workspace.BuildSnapshot` -> `engine.Execute` (compile failures stop before snapshot/execution and every post-compile response includes top-level `schema`).
12. For `version`: `options.Parse` -> `buildinfo.ResolveVersion` -> build payload `result_state/version`.

## Binary Entrypoint Status

- Binary entrypoint: `cmd/spec-cli/main.go` - `main`.
- `main` initializes `cli.NewApp(os.Stdout, os.Stderr, time.Now)` and exits through `os.Exit(app.Run(context.Background(), os.Args[1:]))`.
- `make build` builds `./cmd/spec-cli` into `bin/spec-cli` with `-ldflags` injection of `internal/buildinfo.Version` (default `VERSION=dev`); `make run` executes `./cmd/spec-cli`.

## Layer Map

- `internal/cli`
  - Entrypoint: `internal/cli/app.go` - `NewApp`, `(*App).Run`.
  - Responsibilities:
    - Build the application and register handlers (`help`, `schema`, `query`, `get`, `add`, `update`, `delete`, `validate`, `version`) through `internal/cli/command_catalog.go`.
    - Parse global options (`--format`, `--workspace`, `--schema`, `--config`, `--require-absolute-paths`, `--verbose`) both before and after the command name.
    - Load active JSON config before dispatch (`--config <path>` or auto-discovered `cwd/spec-cli.json`), apply only supported keys (`schema`, `workspace`), resolve relative config paths from config directory, and enforce priority `explicit CLI > config > defaults`.
    - Return deterministic `INVALID_CONFIG` for missing/unreadable/unparseable explicit config, invalid auto-discovered config, and unknown config keys.
    - Validate global options deterministically: reject duplicate non-repeatable flags and enforce early `--require-absolute-paths` checks for explicit `--workspace/--schema`.
    - Apply the pre-dispatch capability gate: `--format text` is allowed only for `help`.
    - Render successful and error responses uniformly: JSON for command payload/error, text-first for `help`.
  - Subpackages: none.

- `internal/application/commandbus`
  - Entrypoint: `internal/application/commandbus/bus.go` - `New`, `(*Bus).Register`, `(*Bus).Dispatch`.
  - Responsibilities:
    - Store command handlers by command name.
    - Dispatch a request to the correct handler.
    - Return domain error `INVALID_ARGS` for an unknown command.
  - Subpackages: none.

- `internal/application/workspacelock`
  - Entrypoint: `internal/application/workspacelock/locker.go` - `AcquireExclusive`, `(*Guard).Release`.
  - Responsibilities:
    - Build workspace-level lock path (`<workspace>/.spec-cli/workspace.lock`) from normalized workspace root.
    - Acquire fail-fast advisory exclusive lock before mutating command snapshot/validation stages.
    - Return deterministic lock contention error (`CONCURRENCY_CONFLICT`) for concurrent mutating operations.
    - Keep release lifecycle in one guard API used by `add`, `update`, and `delete` handlers.
  - Subpackages:
    - `workspacelock/internal/flock` - platform lock backend (`flock` on Unix, capability fallback on unsupported targets).

- `internal/application/commands/internal/expressions`
  - Entrypoint: `engine.go` - `NewEngine`, `(*Engine).Compile`, `CompileScalarInterpolation`, `CompileTemplate`, `ContainsLegacyPlaceholder`, `Evaluate`, `RenderTemplate`, `IsTruthy`.
  - Responsibilities:
    - Provide one shared JMESPath compile/cache adapter for legacy mutating command flows (`update`) with deterministic compile/eval diagnostics.
    - Compile scalar interpolation constraints (`"${expr}"`) and mixed string templates with `${...}` parts.
    - Detect and reject legacy `{...}` placeholders in template literals while preserving `${...}` interpolation syntax.
    - Evaluate expressions against runtime entity context (`type/id/slug/createdDate/updatedDate/meta/refs`) and apply JMESPath truthiness semantics.
    - Render templates and stringify interpolation values with deterministic runtime type checks.
  - Subpackages: none.

- `internal/application/schema/diagnostics`
  - Entrypoint: `diagnostics.go` - `NewError`, `NewWarning`, `Summarize`, `HasErrors`.
  - Responsibilities:
    - Define shared schema-diagnostic severity model (`error`, `warning`) and payload shape.
    - Provide canonical diagnostic classes (`SchemaError`, `SchemaWarning`) for compiler output.
    - Calculate deterministic summary counters (`errors`, `warnings`) used by command responses.
  - Subpackages: none.

- `internal/application/schema/derivedschema`
  - Entrypoint: `builder.go` - `LiteralMatchesKind`, `ProjectValueSpec`, `ProjectMetaField`, `StaticConstValue`, `StaticEnumValues`.
  - Responsibilities:
    - Provide one shared literal-to-kind compatibility predicate reused by primary value diagnostics and derived-schema projection.
    - Build downstream-safe projections for derived consumers by dropping incompatible `const` and filtering incompatible `enum` literals.
    - Provide one shared conservative projection for static literal constraints: interpolated literals (`Template != nil`) are excluded from `const/enum` static constraints.
    - Preserve schema shape (`kind/format/ref/array fields`) while recursively projecting nested `items` value specs.
    - Provide meta-field projection that keeps `required/description/schemaPath` untouched and projects only value constraints.
  - Subpackages: none.

- `internal/application/schema/expressioncontext`
  - Entrypoint: `context.go` - `IsBuiltinMetaField`, `BuildEntityExpressionSchema`, `IsPathGuaranteedBySchema`, `GuardRootForPath`.
  - Responsibilities:
    - Keep canonical built-in frontmatter keys for schema-level reserved-name checks.
    - Build schema-aware JMESPath context (`type/id/slug/createdDate/updatedDate/meta/refs`) from canonical entity model.
    - Apply derived-schema projection for meta-field value constraints before serializing JMESPath `const/enum` fragments.
    - Project scalar `entityRef` fields into `refs.<field>={id,type,slug,dirPath}` for static expression checks.
    - Provide path/guard helpers used by compiler pathTemplate safety validation.
  - Subpackages: none.

- `internal/application/schema/expressions`
  - Entrypoints:
    - `compiler.go` - `NewEngine`, `NewSchemaAwareEngine`, `(*Engine).Compile`
    - `interpolation.go` - `ContainsInterpolation`, `CompileScalarInterpolation`, `CompileTemplate`, `RenderTemplate`
    - `evaluator.go` - `Evaluate`, `IsTruthy`, `StringifyInterpolationValue`
  - Responsibilities:
    - Provide shared JMESPath compile/cache adapter for schema compile-time checks and runtime evaluation.
    - Compile expressions in scalar/template modes with inferred-type validation and stable diagnostic-code mapping.
    - Compile and render `${...}` templates with deterministic parsing and interpolation boundary handling.
    - Expose guard analysis (`ProtectsWhenTrue`, guarded paths) for schema-level pathTemplate safety checks.
    - Evaluate compiled expressions/templates at runtime and normalize interpolation output types.
  - Subpackages: none.

- `internal/application/schema/model`
  - Entrypoint: `model.go` - canonical schema model types (`CompiledSchema`, `EntityType`, `MetaField`, `Section`, `ValueSpec`, `PathTemplate`, `RefSpec`).
  - Responsibilities:
    - Hold command-agnostic canonical schema semantics after compile.
    - Keep compiled expression/template objects in canonical required/enum/const/pathTemplate rules.
    - Preserve deterministic source-derived projection order (`MetaFieldOrder`, `SectionOrder`) and `HasContent` for write-side capability builders.
    - Preserve schema provenance paths (`required/title/pathTemplate use`) for runtime diagnostics.
    - Represent scalar/array value kinds and reference cardinality in one shared IR.
  - Subpackages: none.

- `internal/application/schema/source`
  - Entrypoint: `load.go` - `Load`.
  - Responsibilities:
    - Read raw schema source from filesystem and keep display-path context for diagnostics.
    - Parse YAML/JSON root node and reject empty files.
    - Validate source-level structure (`root mapping`, duplicate keys) before semantic compile.
  - Subpackages:
    - `source/internal/yamlnodes` - YAML-node helpers (`FirstContentNode`, recursive duplicate-key scan with path reconstruction).

- `internal/application/schema/compile`
  - Entrypoint: `compile.go` - `NewCompiler`, `Compile`, `(*Compiler).Compile`.
  - Responsibilities:
    - Coordinate shared compile pipeline: source load -> semantic compile -> summary flags.
    - Keep in-process compile cache per command run (`path + displayPath`) and return cloned cached results.
    - Guarantee `Result.Issues` is always a non-nil slice (JSON `[]` for zero diagnostics).
    - Return canonical result bundle (`schema`, diagnostics list, summary, valid flag) plus shared compile failure classification into domain `AppError` (`SCHEMA_NOT_FOUND | SCHEMA_READ_ERROR | SCHEMA_PARSE_ERROR | SCHEMA_INVALID`).
    - Keep warnings-only diagnostics non-blocking (`compileErr=nil`, `result.Valid=true`) and classify failures once in shared layer for all migrated commands.
  - Subpackages:
    - `compile/internal/compiler` - thin bridge from compile entrypoint to semantic compiler.
    - `compile/internal/compiler/internal/semantic` - canonical semantic compiler for `idPrefix`, `required`, section-title rules, const/enum interpolation, pathTemplate cases/guards, and expression-aware checks.
    - `compile/internal/compiler/internal/shared` - shared parsing primitives for mappings/scalars, requirements, value-schema constraints (`type/enum/const/refTypes/items/min/max`), deterministic diagnostics, and primary literal mismatch checks via `derivedschema.LiteralMatchesKind`.

- `internal/application/schema/capabilities/read`
  - Entrypoint: `builder.go` - `Build`.
  - Responsibilities:
    - Build read-side capability projection from compiled schema entity map with explicit `EntityTypes -> {MetaFields, RefFields, Sections}`.
    - Split non-ref `meta` fields from `entityRef` fields at shared boundary (`MetaFields` excludes refs; `RefFields` contains scalar and array entityRef only).
    - Project read semantics for `query/get`: field kinds (`kind/itemKind`), static-only `enum/const`, `required` (`Always` only), ref cardinality, and normalized ref `AllowedTypes`.
    - Apply conservative static-literal projection for read planning: interpolated string `const` and any `enum` containing an interpolated literal are excluded from static constraints.
    - Normalize open ref targets once in shared layer (`refTypes` omitted => all entity types) so consumers do not implement special-case expansion.
    - Keep read capability independent from YAML parsing and command handlers.
  - Subpackages: none.

- `internal/application/schema/capabilities/write`
  - Entrypoint: `builder.go` - `Build`.
  - Responsibilities:
    - Derive write-side capability from compiled schema for mutating commands: `idPrefix`, `pathTemplate`, `meta/section` rules, `hasContent`, and write-path allowlists.
    - Project write namespace split (`meta.<field>`, `refs.<field>`, `content.sections.<name>`) including scalar/array `entityRef` support for `refs`.
    - Expose deterministic field/section order plus value constraints (`type/refTypes/items/min/max/unique/enum/const`) for write parsing and validation runtime, preserving template-aware `const/enum` as `{literal, template}`.
    - Preserve compiler-owned provenance paths for runtime diagnostics (`meta.required`, `section.required`, `pathTemplate.cases[].when`, `pathTemplate.cases[].use`) in capability projection.
    - Keep canonical sorted/deduplicated `SetPaths/UnsetPaths/SetFilePaths` output while preserving source-derived serialization order in dedicated fields.
  - Subpackages: none.

- `internal/application/schema/capabilities/validate`
  - Entrypoint: `builder.go` - `Build`.
  - Responsibilities:
    - Build deterministic runtime validation plan from canonical schema (`EntityOrder` + per-type rules).
    - Project required field/section rules with expression paths, type/ref/array constraints, and const/enum values.
    - Project allowed frontmatter set, pathTemplate cases (`use/when` + compiled templates/expressions), and stable type ordering.
    - Keep runtime engine independent from YAML parsing and command-local schema IR.
  - Subpackages: none.

- `internal/application/schema/capabilities/references`
  - Entrypoint: `builder.go` - `Build`.
  - Responsibilities:
    - Build reverse-reference capability (`inbound slots`) keyed by target entity type.
    - Build source-oriented slot capability (`slots by source type`) consumed by `delete` reverse-ref scanning.
    - Distinguish scalar vs array cardinality for both inbound and source slot views.
    - Normalize allowed target-type expansion (`refTypes` or all entity types) and deterministic slot ordering in both views.
  - Subpackages: none.

- `internal/application/commands/schema`
  - Entrypoints:
    - `handler.go` - `NewHandler`, `(*Handler).Handle`
    - `help.go` - `HelpSpec`
  - Responsibilities:
    - Orchestrate `schema check`: parse subcommand -> normalize schema path -> run shared compiler.
    - On compile success, build JSON response with `validation_scope: "schema"` and top-level `schema` diagnostics block (`valid`, `summary`, `issues`).
    - On compile failure, return top-level `error` together with top-level `schema` and use process exit code from compile `AppError`.
    - Own `schema` command help inside shared `help`.
  - Subpackages:
    - `schema/internal/options` - parse `schema` subcommand contract (`check`) and normalize schema path.

- `internal/application/commands/validate`
  - Entrypoints:
    - `internal/application/commands/validate/handler.go` - `NewHandler`, `(*Handler).Handle`
    - `internal/application/commands/validate/help.go` - `HelpSpec`
  - Responsibilities:
    - Orchestrate `validate`: parse options -> normalize paths -> compile schema -> build top-level `schema` block.
    - Stop before workspace scan when compile returns `AppError` and return top-level `error + schema`, `issues: []`, and runtime summary with zero scanned entities.
    - Build validation capability and validate `--type` filters against capability entity map.
    - Run workspace scan + runtime engine only after successful compile and build response with top-level `schema`.
    - Keep runtime issues only in top-level `issues` and schema diagnostics only in top-level `schema.issues`.
    - Compute process exit code from compile `AppError` for schema compile failures; for runtime success path keep unified `--warnings-as-errors` over schema + runtime warning planes.
    - Own command help for `validate` inside shared `help`, including syntax-only repeated type placeholders in examples.
  - Subpackages:
    - `validate/internal/options` - option parsing and path normalization.
    - `validate/internal/workspace` - markdown workspace scan and frontmatter/content parsing.
    - `validate/internal/engine` - runtime validation pipeline.
    - `validate/internal/model` - internal command-state types.
    - `validate/internal/support` - pure helpers for YAML, collections, and values.

- `internal/application/commands/validate/internal/options`
  - Entrypoints:
    - `parse.go` - `Parse`
    - `paths.go` - `NormalizePaths`
  - Responsibilities:
    - Parse `validate` flags (`--type`, `--fail-fast`, `--warnings-as-errors`).
    - Normalize `workspace/schema` paths to absolute form.
    - Enforce `--require-absolute-paths` with `INVALID_ARGS` on violations.
  - Subpackages: none.

- `internal/application/commands/validate/internal/workspace`
  - Entrypoints:
    - `candidates.go` - `BuildCandidateSet`
    - `frontmatter.go` - `ParseFrontmatter`
  - Responsibilities:
    - Deterministically scan workspace `.md` files and filter by `--type`.
    - Parse frontmatter with duplicate-key control.
    - Extract section labels and detect duplicate headings.
  - Subpackages: none.

- `internal/application/commands/validate/internal/engine`
  - Entrypoints:
    - `runner.go` - `RunValidation`
    - `issues.go` - `CountIssuesByLevel`
  - Responsibilities:
    - Run runtime validation over workspace candidates using shared validation capability (no local schema parsing).
    - Build runtime evaluation context (`type/id/slug/createdDate/updatedDate/meta/refs`) for required/pathTemplate expression evaluation.
    - Evaluate required field/section rules, const/enum interpolations, and array/ref constraints from capability plan.
    - Select first matching pathTemplate case left-to-right, render `use` template, and emit runtime mismatch/evaluation diagnostics.
    - Resolve scalar and array `entityRef` targets (`missing|ambiguous|type_mismatch`) using deterministic global id index.
    - Aggregate runtime/global uniqueness issues and compute coverage/validity/conformance metrics.
  - Subpackages: none.

- `internal/application/commands/query`
  - Entrypoints:
    - `handler.go` - `NewHandler`, `(*Handler).Handle`
    - `help.go` - `HelpSpec`
  - Responsibilities:
    - Orchestrate `query`: parse options -> normalize paths -> compile schema -> build top-level `schema` payload -> build shared read capability -> plan -> workspace load -> execute.
    - Return top-level `schema` in every post-compile JSON response (`success`, compile failures, and non-schema runtime/query failures).
    - Build contractual JSON response (`items`, `matched`, `page`) over the shared compile/read capability pipeline.
    - Keep namespace split in user contract/diagnostics: `projection-namespace` for `--select`, `filter-namespace` for `--sort` and `--where` (JMESPath).
    - Own `query` help as a structured read-model contract (`Active type set`, `Read-model path forms`, `Where language`, `Defaults`, `Rules`) including role-specific syntax-only examples/placeholders (`meta.<meta_field>`, `refs.<scalar_ref_field>`, `refs.<array_ref_field>`, `content.raw`, `content.sections.<section_name>`), explicit selected-leaf `null` materialization, sparse aggregate-selector semantics, explicit `content.raw` support for read/select/sort, a `--where`-level ban on `content.raw`, and the runtime-mandatory hidden sort tail contract (`type:asc`, `id:asc`).
    - Own `query` help inside shared `help`.
    - Preserve query-level error classification for non-schema paths (`INVALID_ARGS`, `INVALID_QUERY`, `ENTITY_TYPE_UNKNOWN`, `READ_FAILED`) after successful compile.
  - Subpackages:
    - `query/internal/options` - `--type`, `--where`, `--select`, `--sort`, `--limit`, `--offset`.
    - `query/internal/workspace` - full read-view building and `refs.<field>` resolution.
    - `query/internal/engine` - planner, schema-aware JMESPath `--where` compile/evaluate, sorting, pagination, projection.
    - `query/internal/model` - internal request/plan/AST/response types.
    - `query/internal/support` - pure helpers for YAML/collections/value operations reused by workspace/frontmatter and deterministic map handling.

- `internal/application/commands/query/internal/options`
  - Entrypoints:
    - `parse.go` - `Parse`
    - `paths.go` - `NormalizePaths`
  - Responsibilities:
    - Parse query options with defaults (`limit=100`, `offset=0`) and basic `--sort` syntax validation.
    - Return `INVALID_ARGS` centrally for unknown/incomplete arguments.
    - Normalize `workspace/schema` paths with `--require-absolute-paths`.
  - Subpackages: none.

- `internal/application/commands/query/internal/workspace`
  - Entrypoint: `loader.go` - `LoadEntities`.
  - Responsibilities:
    - Deterministically scan `.md` files and parse entity frontmatter/body.
    - Build full read-view (`type/id/slug/revision/createdDate/updatedDate/meta/refs/content.raw/content.sections`).
    - Use shared read capability directly for schema-known `meta`/`refs`/`sections` sets (no command-local schema re-mapping).
    - Build global `id` index and resolve `refs.<field>` into scalar/array refs with unresolved classification `missing|ambiguous|type_mismatch`; unresolved public refs include `reason`.
    - Distinguish explicit scalar `null` ref (public `null`) from unresolved ref object; where-context skips explicit-null scalar refs and keeps `reason` leaf for unresolved/resolved paths.
    - Materialize where-context as schema-known `meta` (non-ref fields only), `refs`, and `content.sections` (schema-known sections only; empty object when absent).
    - Normalize YAML values (`time.Time` -> `YYYY-MM-DD`, numeric scalars -> `float64`) and compute opaque `revision` (`sha256:<hex>`).
    - Return `READ_FAILED` for unknown entity types in workspace and syntactically invalid scalar/array ref values.
  - Subpackages: none.

- `internal/application/commands/query/internal/engine`
  - Entrypoint: `planner.go` - `BuildPlan`.
  - Responsibilities:
    - Validate type filters and build active type set used by path validation and where-schema compilation.
    - Validate `--select` against `projection-namespace`, including `refs.<field>.reason` and array-ref leaf restrictions.
    - Apply default projection (`type`, `id`, `slug`, `meta`, `refs`) when `--select` is omitted.
    - Compile `--where` as schema-aware JMESPath expression over query-item schema (`oneOf` over active types) with AST policy checks (`content.raw`, root `content`, `meta.<entityRef>`).
    - Build effective sort (default + hidden tail), including `refs.<field>.reason`, with active-type-set sort-kind compatibility checks, `meta.<entityRef>` rejection, and array-ref leaf restrictions.
    - Execute filter/sort/paginate/project pipeline and build page metadata (`matched`, `returned`, `has_more`, `next_offset`, `effective_sort`).
  - Subpackages: none.

- `internal/application/commands/help`
  - Entrypoints:
    - `handler.go` - `NewHandler`, `(*Handler).Handle`
    - `help.go` - `HelpSpec`
  - Responsibilities:
    - Orchestrate `spec-cli help`, `spec-cli help <command>`, and `spec-cli help <command> --show-schema-projection` through one ordered command catalog.
    - Reject explicit `--format json` for `help` with `CAPABILITY_UNSUPPORTED`.
    - Build schema status/model once (`helpschema.LoadReport`) and render one stable `Schema` block contract in every branch: `Workspace`, deterministic absolute `ResolvedPath`, `Status`, and recovery diagnostics (`ReasonCode`, `Impact`, `RecoveryClass`, `RetryCommand`) only for degraded statuses.
    - Keep stable placement rules: general help places `Schema` right after `CLI`; command help places `Schema` right after `Command` and before `Syntax`.
    - Render loaded general help as structured sections (`CLI`, `Schema`, `Execution model`, `Specification model`, `Projection conventions`, `Specification projection`, `Reference value model`, `Global options`, `Commands`, `Command details`) and keep `Specification projection` as a separate block.
    - Render loaded command help from one structured command contract (`Operation model` + `DetailSections`) with syntax-only examples that use role-specific schema-neutral placeholders; include the same schema-derived `Specification projection` block only when `--show-schema-projection` is explicitly set.
    - Keep `spec-cli help --show-schema-projection` non-duplicating for the general help projection block.
    - Keep tolerant behavior: on schema problems (`missing|invalid|error`) render degraded help with explicit recovery contract (`ReasonCode/Impact/RecoveryClass/RetryCommand`) instead of hard failure.
    - Return `INVALID_ARGS` for unknown `help <command>`.
  - Subpackages:
    - `help/internal/options` - parse `help` positionals plus `--show-schema-projection`, canonical absolute workspace/schema paths, deterministic `WorkspaceDisplay`, and fixed-root normalized absolute `ResolvedPath`.

- `internal/application/help/helpmodel`
  - Entrypoint: `model.go` - `NewCatalog`, `MustCatalog`, `(*Catalog).Ordered`, `(*Catalog).Find`, `(*Catalog).Names`, `(*Catalog).Has`.
  - Responsibilities:
    - Typed model for command help contract (`CommandSpec`, `PositionalSpec`, `OptionSpec`, `DetailSectionSpec`, `GlobalOptionSpec`).
    - One ordered command catalog for `Commands` and `Command details`.
    - Validate command uniqueness.
  - Subpackages: none.

- `internal/application/help/helpglobal`
  - Entrypoint: `options.go` - `Options`.
  - Responsibilities:
    - Define compact shared descriptor of global options for the top-level `Global options` section.
    - Provide normative wording for `--workspace`, `--schema`, `--config`, `--format`, `--require-absolute-paths`.
  - Subpackages: none.

- `internal/application/help/helpschema`
  - Entrypoint: `projector.go` - `LoadReport`.
  - Responsibilities:
    - Compile the effective schema through shared compiler (`schema/compile`) and keep help schema semantics aligned with runtime command semantics.
    - Classify schema unavailability into `loaded|missing|invalid|error` plus reason codes (`SCHEMA_NOT_FOUND`, `SCHEMA_NOT_READABLE`, `SCHEMA_PARSE_ERROR`, `SCHEMA_VALIDATION_ERROR`, `SCHEMA_PROJECTION_ERROR`).
    - Build loaded help payload from shared capabilities (`read`, `write`, `validate`, `references`): specification projection JSON, schema-derived catalog for examples, validate entity order, inbound reference slots.
    - Delegate specification projection rendering to a data-driven shared-model projector so all compiled value facets (`items`, `minItems`, `maxItems`, `uniqueItems`, `const`, `enum`, `format`, refs metadata) flow automatically from `model.ValueSpec`.
    - Publish ref cardinality only through projection shape (`x-kind=entityRef` on scalar refs; `type=array` + `items.x-kind=entityRef` on array refs); `x-cardinality` is intentionally not emitted.
    - Preserve dynamic literal constraints in projection without heuristics: static `const/enum` stay native JSON Schema keywords, dynamic `const` is published as `x-const`, and dynamic/mixed `enum` is published as ordered `x-enum` entries (`literal|interpolation` objects).
    - Normalize projection-level `x-refTypes` for open refs (`refTypes` omitted) to the full deterministic entity-type list from the effective schema (never `null`).
    - Publish `content.raw` together with `content.sections` in the specification projection whenever the entity type has content sections.
    - Publish section heading metadata as canonical scalar `title` only (first schema title), intentionally hiding additional schema aliases in help projection.
    - Keep deterministic ordering for entity types, fields, sections, and projection output.
    - Build degraded-mode recovery contract (`Impact`, `RecoveryClass`, `RetryCommand`) without heuristic schema-derived values.
    - Use `spec-cli schema check --schema <path>` as degraded `RetryCommand` for schema syntax/semantic failures (`SCHEMA_PARSE_ERROR`, `SCHEMA_VALIDATION_ERROR`) while preserving existing retry semantics for missing/read/projection scenarios.
    - Treat any schema validation failure (`SCHEMA_INVALID`) as degraded help status (`invalid` + `SCHEMA_VALIDATION_ERROR`) and never mask it as loaded.
    - Pass absolute `ResolvedPath` into the report.
  - Subpackages:
    - `helpschema/internal/projection` - deterministic specification projection renderer over `model.CompiledSchema` (`built-ins/meta/refs/content.raw/content.sections`) with recursive value-facet serialization and preserved help `x-*` extensions.

- `internal/application/help/helptext`
  - Entrypoint: `renderer.go` - `RenderGeneral`, `RenderCommand`.
  - Responsibilities:
    - Render one unified `Schema` section shape for loaded and degraded help branches (`Workspace`, `ResolvedPath`, `Status`; degraded mode adds recovery diagnostics).
    - Enforce stable section placement: `CLI -> Schema` for general help and `Command -> Schema -> Syntax` for command help.
    - Keep the same schema-neutral section layout in loaded and degraded modes (`Execution model`, `Specification model`, `Projection conventions`, `Reference value model`, command `Operation model` and `DetailSections`), while hiding only schema-dependent projection/catalog payloads when schema is unavailable.
    - Document projection-specific extensions in text contract: `x-const`/`x-enum` for dynamic interpolation constraints and canonical scalar section `title` semantics for whole-body heading defaults.
    - Document ref cardinality contract as shape-derived (`x-kind` for scalar refs, `type=array` + `items.x-kind` for array refs) without `x-cardinality`.
    - Render command help via `Operation model` + `DetailSections` in both loaded and degraded branches; append the schema-unavailable rule in degraded mode without replacing the structural command contract.
    - Reuse the shared specification-projection renderer for both general help and `help <command> --show-schema-projection` so projection source/format stays identical.
    - Render normalized options block (positionals before flags, sentinel `none`) and stable command sections without ANSI/table layout.
    - Generate deterministic command examples as syntax-only templates with role-specific placeholders (`<entity_type>`, `<entity_id>`, `meta.<meta_field>`, `meta.<meta_scalar_field>`, `meta.<meta_array_field>`, `refs.<ref_field>`, `refs.<scalar_ref_field>`, `refs.<array_ref_field>`, `content.sections.<section_name>`) instead of concrete schema literals.
    - Preserve the same syntax-only example families in loaded and degraded modes (no heuristic substitution of enum values, ids, slugs, field names, section names, or free-text literals).
    - Use one command-order source from the shared help catalog for both loaded/degraded `Commands` and `Command details` sections.
  - Subpackages: none.

- `internal/application/commands/get`
  - Entrypoints:
    - `handler.go` - `NewHandler`, `(*Handler).Handle`
    - `help.go` - `HelpSpec`
  - Responsibilities:
    - Orchestrate `get`: parse options -> normalize paths -> compile schema -> build top-level `schema` payload -> build shared read capability -> validate selectors -> locate target by `id` -> read target -> build read-view -> project JSON.
    - Return top-level `schema` in every post-compile JSON response (`success`, compile failures, and non-schema lookup/read failures).
    - Own `get` help as a structured single-entity read-model contract (`Target type`, `Read-model path forms`, `Defaults`, `Rules`) with selected-leaf `null` materialization semantics, sparse aggregate selectors, explicit `content.raw` support in the read/select contract, and syntax-only examples using `<entity_id>` plus role-specific read-path placeholders.
    - Build contractual JSON response (`result_state`, `target`, `entity`) for single-entity read over shared compile/read capability.
    - Enforce projection contract for scalar and array `entityRef`: `meta.<ref_field>` is not selectable; refs are projected via `refs|refs.<name>`; path-based ref leaf selectors `refs.<name>.id|resolved|type|slug|reason` are scalar-only and are rejected for array refs or scalar/array conflicts across schema types.
    - Own `get` help inside shared `help`.
    - Preserve error-class mapping after compile (`INVALID_ARGS`, `ENTITY_NOT_FOUND`, `TARGET_AMBIGUOUS`, `READ_FAILED`) and shared compile-class mapping (`SCHEMA_*`).
  - Subpackages:
    - `get/internal/options` - parse `--id` and repeatable `--select`, normalize paths.
    - `get/internal/workspace` - deterministic scan, fast `id` locator, strict target parsing, section extraction, `revision`.
    - `get/internal/engine` - selector plan/tree over shared read capability, absent-value projection, special-case `content.sections.<name> = null`, read-view build, blocking policy.
    - `get/internal/model` - internal options, selector-plan, and parsed-target types.
    - `get/internal/support` - pure helpers for YAML/maps/deep-copy/validation issues.

- `internal/application/commands/get/internal/options`
  - Entrypoints:
    - `parse.go` - `Parse`
    - `paths.go` - `NormalizePaths`
  - Responsibilities:
    - Parse `get` options (`--id`, `--select`) and enforce required `--id`.
    - Validate unknown/incomplete arguments with `INVALID_ARGS`.
    - Normalize `workspace/schema` paths with `--require-absolute-paths`.
  - Subpackages: none.

- `internal/application/commands/get/internal/workspace`
  - Entrypoint: `reader.go` - `LocateByID`, `ReadTarget`.
  - Responsibilities:
    - Deterministically scan `.md` files and locate target by exact `id` without full workspace validation.
    - Distinguish not found (`ENTITY_NOT_FOUND`), ambiguous (`TARGET_AMBIGUOUS`), and single target.
    - Strictly parse target frontmatter/body, validate `type/id`, compute `revision`, extract sections.
    - Build parseable-entity `id` index for expanding `refs`.
  - Subpackages: none.

- `internal/application/commands/get/internal/engine`
  - Entrypoints:
    - `plan.go` - `BuildSelectorPlan`
    - `entity.go` - `BuildEntityView`
  - Responsibilities:
    - Validate selectors against shared read capability and build terminal select tree, including rejection of path-based ref leaf selectors for array refs and scalar/array conflicts across schema types.
    - Apply default projection (`type`, `id`, `slug`, `meta`, `refs`) when `--select` is omitted.
    - Classify projection requirements (`refs`, `content.raw`, `content.sections`, requested ref/section fields) and apply null policy for `refs.<name>`/`refs.<name>.<leaf>` and `content.sections.<name>`.
    - Build read-view of target entity (`meta`, expanded `refs`, `content`) with scalar/array refs and unresolved classification `missing|ambiguous|type_mismatch` (`reason` in unresolved public refs) driven by shared ref `AllowedTypes`.
    - Mark `resolved=true` only for unique ref target compatible with `AllowedTypes`; unique incompatible target stays unresolved with deterministic fallback and `reason`.
    - Apply blocking policy only when a requested ref slot is structurally unreadable and deterministic `id` cannot be obtained.
    - Project only the selected response subtree.
  - Subpackages: none.

- `internal/application/commands/get/internal/model`
  - Entrypoint: `types.go` - package-visible model types.
  - Responsibilities:
    - Command options (`Options`).
    - Selector-plan types (`SelectNode`, `SelectorPlan`).
    - Workspace read and lookup types (`LocateResult`, `ParsedTarget`, `EntityIdentity`).
  - Subpackages: none.

- `internal/application/commands/get/internal/support`
  - Entrypoints:
    - `yaml.go` - `FirstContentNode`, `FindDuplicateMappingKey`
    - `collections.go` - `SortedMapKeys`
    - `values.go` - `DeepCopy`, `ValidationIssue`, `WithValidationIssues`
  - Responsibilities:
    - YAML AST helpers and duplicate-key checks.
    - Stable map-key sorting.
    - Deep copy and `validation.issues` payload assembly for domain errors.
  - Subpackages: none.

- `internal/application/commands/add`
  - Entrypoints:
    - `handler.go` - `NewHandler`, `(*Handler).Handle`
    - `help.go` - `HelpSpec`
  - Responsibilities:
    - Orchestrate `add`: parse options -> normalize paths -> acquire workspace lock -> compile schema -> build shared write capability -> build workspace snapshot -> execute candidate build/validation/write.
    - Stop immediately on compile failure and return top-level `error + schema` before unknown-type checks, snapshot, or execution.
    - Return top-level `schema` in every post-compile JSON response (`success`, unknown type, write-contract/validation/conflict/read/write errors).
    - Keep schema diagnostics only in top-level `schema.issues`; runtime validation diagnostics stay under `error.details.validation.issues` for `VALIDATION_FAILED`.
    - Inject `Clock` for deterministic `createdDate`/`updatedDate`.
    - Own `add` help inside shared `help`, including schema-derived write-model path catalog plus role-specific syntax-only value forms (`meta.<meta_scalar_field>`, `meta.<meta_array_field>`, `refs.<scalar_ref_field>`, `refs.<array_ref_field>`), explicit whole-body heading contract (`## <title> {#<section_name>}`), and canonical-title linkage to `Specification projection` (`content.sections.<section_name>.title`).
  - Subpackages:
    - `add/internal/options` - parse/norm of `--type`, `--slug`, `--set`, `--set-file`, `--content-file`, `--content-stdin`, `--dry-run`.
    - `add/internal/workspace` - deterministic workspace snapshot (`id`/`slug`/suffix indexes, existing paths, parseable identities, `dirPath` context) keyed by shared capability entity map.
    - `add/internal/engine` - apply writes, typed YAML parsing, canonical serialization, `pathTemplate` / expression evaluation, full validation, `revision`, dry-run, atomic write.
    - `add/internal/model` - internal use-case types plus aliases for shared write capability models.
    - `add/internal/support` - YAML/deep-copy/literal-compare/stable-collection/error-detail helpers.

- `internal/application/commands/add/internal/options`
  - Entrypoints:
    - `parse.go` - `Parse`
    - `paths.go` - `NormalizePaths`
  - Responsibilities:
    - Parse and validate `add` arguments, including whole-body vs section conflicts, duplicate paths, and mutually exclusive flags.
    - Normalize `workspace/schema` and file-path arguments with `--require-absolute-paths`.
  - Subpackages: none.

- `internal/application/commands/add/internal/workspace`
  - Entrypoints:
    - `snapshot.go` - `BuildSnapshot`
    - `frontmatter.go` - `ParseFrontmatter`
    - `sections.go` - `ExtractSections`
  - Responsibilities:
    - Deterministically scan `.md` files and build snapshot indexes (`entitiesByID`, `slugsByType`, `maxSuffixByType`, `existingPaths`).
    - Tolerantly parse frontmatter to collect identity/meta context of existing entities.
    - Extract labeled sections and detect duplicate labels for full candidate validation.
  - Subpackages: none.

- `internal/application/commands/add/internal/engine`
  - Entrypoint: `execute.go` - `Execute`.
  - Responsibilities:
    - Run create pipeline: apply writes -> build candidate -> resolve refs -> build expression context -> evaluate path -> validate -> serialize -> write/dry-run response.
    - Route `WRITE_CONTRACT_VIOLATION`, `VALIDATION_FAILED`, `PATH_CONFLICT`, `WRITE_FAILED`, `INTERNAL_ERROR`.
    - Build contractual `entity` payload and final `result_state=valid`.
  - Subpackages:
    - `.../writes` - apply write ops, typed YAML parsing, write-contract guard rails, Markdown body build.
    - `.../refresolve` - resolve scalar `entityRef` and `array.items.type=entityRef` via workspace snapshot and diagnose deterministic `missing|ambiguous|type_mismatch` (indexed for arrays).
    - `.../pathcalc` - evaluate `pathTemplate` case conditions (`when`) and `use` templates via shared JMESPath `${expr}` engine, normalize path, guard workspace boundaries, and emit runtime diagnostics on compiler-owned `when/use` paths.
    - `.../validation` - full instance validation, including expression-based `required` checks on compiler-owned required paths, template-aware `schema.const/schema.enum` resolution, dedicated interpolation-failure diagnostics, and deterministic issue sorting.
    - `.../markdown` - canonical frontmatter/body serialization and `revision` computation (`sha256:*`).
    - `.../storage` - filesystem checks and atomic write.
    - `.../payload` - public `entity` payload (`meta`, `refs`, `content.sections`).
    - `.../lookup` - candidate-value lookup adapter for expressions/path evaluation.
    - `.../issues` - `domainvalidation.Issue` factory with entity context.

- `internal/application/commands/add/internal/model`
  - Entrypoint: `types.go` - package-visible command-state structs.
  - Responsibilities:
    - Option and write-operation types.
    - Shared write-capability aliases (`EntityTypeSpec`, `MetaField`, `RuleValue`, `PathPattern`, write-path kinds/specs) consumed by add runtime.
    - Snapshot/candidate entity model types.
    - Shared structures passed between `options -> capability -> workspace -> engine`.
  - Subpackages: none.

- `internal/application/commands/add/internal/support`
  - Entrypoints:
    - `yaml.go` - `FirstContentNode`, `FindDuplicateMappingKey`, `ParseYAMLValue`, `EncodeYAMLNode`
    - `collections.go` - `SortedMapKeys`, `SortedUniqueStrings`
    - `values.go` - `NormalizeValue`, `LiteralEqual`, `WithValidationIssues`
  - Responsibilities:
    - YAML AST helpers and typed scalar parsing.
    - Value normalization/comparison for validation and expression evaluation.
    - Shared assembly of `validation.issues` details for error envelopes.
  - Subpackages: none.

- `internal/application/commands/delete`
  - Entrypoints:
    - `handler.go` - `NewHandler`, `(*Handler).Handle`
    - `help.go` - `HelpSpec`
  - Responsibilities:
    - Orchestrate `delete`: parse options -> normalize paths -> acquire workspace lock -> compile schema -> build top-level `schema` payload -> build shared references capability -> build workspace snapshot -> execute delete/checks.
    - Return top-level `schema` in every post-compile response (`success`, compile failure, and post-compile non-schema errors).
    - Map `INVALID_ARGS`, `SCHEMA_*`, `CONCURRENCY_CONFLICT`, `ENTITY_NOT_FOUND`, `AMBIGUOUS_ENTITY_ID`, `REVISION_UNAVAILABLE`, `DELETE_BLOCKED_BY_REFERENCES`, `WRITE_FAILED`.
    - Own `delete` help inside shared `help` with syntax-only `<entity_id>` examples and schema-neutral revision token placeholder (`<token>`).
  - Subpackages:
    - `delete/internal/options` - parse/norm of `--id`, `--expect-revision`, `--dry-run`.
    - `delete/internal/workspace` - deterministic scan, target lookup by `id`, tolerant frontmatter parse, `revision`.
    - `delete/internal/engine` - target lookup, optimistic concurrency, reverse-ref blocking from shared source-slot capability, dry-run/commit payload.
    - `delete/internal/storage` - file deletion and filesystem error mapping.
    - `delete/internal/model` - internal use-case types.
    - `delete/internal/support` - YAML AST helpers used by tolerant workspace parsing.

- `internal/application/commands/delete/internal/options`
  - Entrypoints:
    - `parse.go` - `Parse`
    - `paths.go` - `NormalizePaths`
  - Responsibilities:
    - Parse and validate `delete` arguments, including `--flag=value`.
    - Enforce required `--id` and normalize boolean `--dry-run`.
    - Normalize `workspace/schema` paths with `--require-absolute-paths`.
  - Subpackages: none.

- `internal/application/commands/delete/internal/workspace`
  - Entrypoint: `index.go` - `BuildSnapshot`, `FindTargetDocument`.
  - Responsibilities:
    - Deterministically scan `.md` files and build baseline snapshot of parseable entities (`type/id/revision/frontmatter`).
    - Locate exact-`id` target candidates with tolerant fallback (`yaml parse` -> line-based extraction).
    - Normalize YAML values (`time.Time` -> `YYYY-MM-DD`) and compute opaque `revision` (`sha256:<hex>`).
    - Map scan/read failures to `READ_FAILED` with normalized `reason`.
  - Subpackages: none.

- `internal/application/commands/delete/internal/engine`
  - Entrypoints:
    - `execute.go` - `Execute`
  - Responsibilities:
    - Resolve target from `snapshot.TargetMatches` and diagnose `ENTITY_NOT_FOUND`, `AMBIGUOUS_ENTITY_ID`, `REVISION_UNAVAILABLE`.
    - Enforce optimistic concurrency through `--expect-revision`.
    - Find blocking incoming references and build stable `blocking_refs`.
    - Execute dry-run or real delete and build contractual payload (`result_state`, `dry_run`, `deleted`, `target.id/revision`).
  - Subpackages: none.

- `internal/application/commands/delete/internal/storage`
  - Entrypoint: `delete.go` - `Delete`.
  - Responsibilities:
    - Remove the target markdown file through `os.Remove`.
    - Map filesystem delete reasons into contractual `reason`.
    - Support test-only failure injection via `SPEC_CLI_TEST_INJECT_DELETE_FAILURE`.
  - Subpackages: none.

- `internal/application/commands/delete/internal/model`
  - Entrypoint: `types.go` - package-visible command-state structs.
  - Responsibilities:
    - Command options.
    - Workspace snapshot types.
    - Blocking-reference diagnostics.
  - Subpackages: none.

- `internal/application/commands/delete/internal/support`
  - Entrypoint: `yaml.go` - `FirstContentNode`, `FindDuplicateMappingKey`
  - Responsibilities:
    - YAML AST helpers for tolerant frontmatter parsing.
    - Duplicate-key checks for parsed frontmatter mappings.
  - Subpackages: none.

- `internal/application/commands/update`
  - Entrypoints:
    - `handler.go` - `NewHandler`, `(*Handler).Handle`
    - `help.go` - `HelpSpec`
  - Responsibilities:
    - Orchestrate `update`: parse options -> normalize paths -> acquire workspace lock -> compile schema -> build shared write capability -> build workspace snapshot -> execute.
    - Support mutating patch (`--set`, `--set-file`, `--unset`, whole-body ops) plus optimistic concurrency (`--expect-revision`).
    - Return contractual JSON response (`updated/noop/changes/entity/validation`) and include top-level `schema` for every post-compile success/error path.
    - Own `update` help inside shared `help`, including schema-derived write-model path catalog plus role-specific syntax-only value forms (`meta.<meta_scalar_field>`, `meta.<meta_array_field>`, `refs.<scalar_ref_field>`, `refs.<array_ref_field>`), explicit whole-body heading contract (`## <title> {#<section_name>}`), and canonical-title linkage to `Specification projection` (`content.sections.<section_name>.title`).
  - Subpackages:
    - `update/internal/options` - parse/norm of `update` options and conflict checks.
    - `update/internal/workspace` - workspace snapshot, frontmatter parsing, section layout.
    - `update/internal/engine` - apply writes, resolve refs/path, full validation, serialize/revision, dry-run/commit.
    - `update/internal/model` - command request/snapshot types + aliases to shared write capability types.
    - `update/internal/support` - YAML/value/collection helpers.

- `internal/application/commands/update/internal/options`
  - Entrypoints:
    - `parse.go` - `Parse`
    - `paths.go` - `NormalizePaths`
  - Responsibilities:
    - Parse `update` flags (`--id`, `--set`, `--set-file`, `--unset`, `--content-file`, `--content-stdin`, `--clear-content`, `--expect-revision`, `--dry-run`).
    - Validate conflicts, duplicate paths, required `--id`, and non-empty patch.
    - Normalize `workspace/schema`, `--content-file`, and `--set-file` with `--require-absolute-paths`.
  - Subpackages: none.

- `internal/application/commands/update/internal/workspace`
  - Entrypoints:
    - `snapshot.go` - `BuildSnapshot`
    - `frontmatter.go` - `ParseFrontmatter`, `ReadStringField`, `BuildMeta`
    - `sections.go` - `BuildSectionLayout`, `ExtractSections`
  - Responsibilities:
    - Deterministically scan `.md` workspace and build snapshot indexes (`EntitiesByID`, `SlugsByType`, `ExistingPaths`, `TargetMatches`).
    - Locate exact-`id` target candidates with tolerant fallback.
    - Parse frontmatter/body, normalize built-in date scalars (`createdDate`, `updatedDate`) to `YYYY-MM-DD`, and normalize meta fields for reference/validation context.
    - Extract labeled sections and detect duplicate labels.
  - Subpackages: none.

- `internal/application/commands/update/internal/engine`
  - Entrypoint: `execute.go` - `Execute`.
  - Responsibilities:
    - Resolve target and enforce optimistic concurrency on persisted `revision`.
    - Apply patch ops / whole-body changes, bump `updatedDate`, resolve scalar/array `entityRef`, build runtime expression context, evaluate `pathTemplate`.
    - Run full post-update validation and map deterministic issues into `VALIDATION_FAILED`.
    - Serialize markdown/frontmatter, recompute `revision`, perform dry-run or commit (atomic write / move), build `changes` / `entity` payload.
  - Subpackages:
    - `.../writes` - preflight write-contract checks, typed YAML parsing, section/body patching, deterministic diff.
    - `.../refresolve` - scalar and array `entityRef` resolution with deterministic `missing|ambiguous|type_mismatch` diagnostics.
    - `.../pathcalc` - `pathTemplate` case selection (`when`) and `use` template rendering via shared JMESPath `${expr}` engine, plus workspace-boundary guard; unit coverage asserts compiler-owned `WhenPath`/`UsePath` propagation into runtime issues.
    - `.../validation` - built-in/meta/content/global rules, expression-based `required` checks, and section-title checks; unit coverage locks template-aware `schema.const`/`schema.enum` interpolation (`happy/mismatch/interpolation failure`) and compiler-owned `RequiredPath` propagation for meta/content required-evaluation failures.
    - `.../storage` - atomic write/move, rollback on rename failure, path conflict checks, test-only write-failure injection.
    - `.../markdown` - canonical frontmatter/body serialization and `revision` computation, including date-literal output for `createdDate`/`updatedDate` (`YYYY-MM-DD`) without RFC3339 expansion.
    - `.../payload` - public `entity` payload.
    - `.../lookup` - path lookup adapter for expressions/path evaluation.
    - `.../issues` - `domainvalidation.Issue` factory with entity context.

- `internal/application/commands/update/internal/model`
  - Entrypoint: `types.go` - package-visible command-state structs.
  - Responsibilities:
    - Patch-option types (`WriteOperation`, `BodyOperation`) and normalized request state.
    - Aliases from shared write capability (`EntityTypeSpec`, `MetaField`, `SectionSpec`, `PathPattern`, write-path specs, `RuleValue`) used by update runtime.
    - Workspace snapshot and candidate-entity types (`WorkspaceEntity`, `TargetMatch`, `Candidate`, `ResolvedRef`).
  - Subpackages: none.

- `internal/application/commands/update/internal/support`
  - Entrypoints:
    - `yaml.go` - `FirstContentNode`, `FindDuplicateMappingKey`, `ToStringMap`, `ToSlice`, `ParseYAMLValue`, `EncodeYAMLNode`
    - `collections.go` - `SortedMapKeys`, `SortedUniqueStrings`
    - `values.go` - `DeepCopy`, `LiteralEqual`, `NormalizeValue`, `ValidationIssue`, `WithValidationIssues`
  - Responsibilities:
    - YAML AST helpers, including deep duplicate-key traversal, and typed YAML value parsing.
    - Value normalization/comparison for patch and validation flows.
    - Shared map/value helpers reused by workspace parsing, patch application, and validation logic.
  - Subpackages: none.

- `internal/application/commands/version`
  - Entrypoints:
    - `handler.go` - `NewHandler`, `(*Handler).Handle`
    - `help.go` - `HelpSpec`
  - Responsibilities:
    - Orchestrate `version`: parse command options -> resolve build version -> build success payload.
    - Return minimal JSON payload (`result_state`, `version`) without touching workspace/schema.
    - Return `INTERNAL_ERROR` on version-provider failure and `INVALID_ARGS` on invalid command arguments.
    - Own `version` help inside shared `help`.
  - Subpackages:
    - `version/internal/options` - parse command-specific options (none in baseline) and reject unknown arguments.

- `internal/application/commands/version/internal/options`
  - Entrypoint: `parse.go` - `Parse`.
  - Responsibilities:
    - Parse `version` arguments without domain/file options.
    - Return `INVALID_ARGS` for any unknown command-specific options.
    - Normalize `--flag=value` form for diagnostics.
  - Subpackages: none.

- `internal/buildinfo`
  - Entrypoint: `version.go` - `ResolveVersion`.
  - Responsibilities:
    - Provide the single runtime source of the build version string.
    - Accept build-time injection of `Version` through `-ldflags`.
    - Return deterministic fallback (`dev`) for local builds without CI injection.
  - Subpackages: none.

- `internal/contracts/requests`
  - Entrypoint: `options.go` - public types `OutputFormat`, `GlobalOptions`, `Command`.
  - Responsibilities:
    - Define global-option and command-request contract.
    - Define one output-format enum (`json`, `text`) and explicit-format flag (`FormatExplicit`).
    - Transport requests between CLI, bus, and handlers.
  - Subpackages: none.

- `internal/contracts/responses`
  - Entrypoint: `types.go` - public types `ResultState`, `CommandOutput`.
  - Responsibilities:
    - Define command-response contract for JSON and text-first payload (`CommandOutput.Text`).
    - Define shared `result_state` enum.
    - Transport `ExitCode` from use-case to CLI.
  - Subpackages: none.

- `internal/contracts/capabilities`
  - Entrypoint: `default.go` - public variable `Default`.
  - Responsibilities:
    - Declare supported commands (`help`, `schema`, `query`, `get`, `add`, `update`, `delete`, `validate`, `version`).
    - Declare supported output formats (`json`, `text`).
  - Subpackages: none.

- `internal/domain/errors`
  - Entrypoint: `codes.go` - `New`, `ExitCodeFor`.
  - Responsibilities:
    - Store the unified catalog of domain error codes, including help-specific ones (`CAPABILITY_UNSUPPORTED`, `SCHEMA_READ_ERROR`, `SCHEMA_PROJECTION_ERROR`).
    - Build `AppError` with code/message/details/exit_code.
    - Map domain codes to process exit codes.
  - Subpackages: none.

- `internal/domain/reservedkeys`
  - Entrypoint: `keys.go` - package constants.
  - Responsibilities:
    - Centralize reserved schema/frontmatter/ref names (`idPrefix`, `pathTemplate`, `entityRef`, `createdDate`, `updatedDate`, `dirPath`, and service model names).
    - Provide one shared source of reserved-key literals for parsers/validators and expression contexts.
    - Keep strict no-compatibility behavior: old `snake_case` names are rejected by closed-world schema checks.
  - Subpackages: none.

- `internal/domain/validation`
  - Entrypoint: `issue.go` - public types `IssueLevel`, `Entity`, `Issue`.
  - Responsibilities:
    - Define domain validation-issue model.
    - Provide one shared shape for errors/warnings in response contracts.
  - Subpackages: none.

- `internal/output/jsonwriter`
  - Entrypoint: `writer.go` - `New`, `(*Writer).Write`.
  - Responsibilities:
    - Write one JSON payload into `io.Writer`.
    - Disable HTML escaping for machine-stable output.
  - Subpackages: none.

- `internal/output/payload`
  - Entrypoint: `payload.go` - `BuildSchemaPayload`, `BuildErrorPayload`.
  - Responsibilities:
    - Build shared top-level `schema` payload from shared compile result (`valid`, `summary`, `issues`).
    - Build shared top-level `error` payload (`code`, `message`, `exit_code`, optional `details`) from domain `AppError`.
    - Keep schema/error response fragments centralized for migrated handlers (`schema check`, `validate`, `query`, `get`, `add`).
  - Subpackages: none.

- `internal/output/errormap`
  - Entrypoint: `mapper.go` - `ResultStateForCode`.
  - Responsibilities:
    - Map domain error code to contractual `result_state`.
    - Special-case `CAPABILITY_UNSUPPORTED` / `NOT_IMPLEMENTED` -> `unsupported`.
    - Centralize error-display policy for output layer.
  - Subpackages: none.

- `tests/integration`
  - Entrypoints:
    - `run_cases_test.go` - `TestValidateCases`, `TestQueryCases`, `TestGetCases`, `TestAddCases`, `TestUpdateCases`, `TestDeleteCases`, `TestVersionCases`, `TestSchemaCases`
    - `help_cases_test.go` - `TestHelpGeneralCases`, `TestHelpSchemaRecoveryCases`, `TestHelpErrorCases`, `TestHelpSelectedCases`
    - `global_options_cases_test.go` - `TestGlobalOptionsCases`
    - `delete_multirun_test.go` - `TestDeleteHappy02DryRunMatchesRealRevision`
    - `workspace_lock_test.go` - `TestMutatingCommandsLockConflict`, `TestMutatingCommandsDryRunRespectsWorkspaceLock`
  - Responsibilities:
    - Run data-first integration cases for `validate`, `query`, `get`, `add`, `update`, `delete`, `version`, `schema` from `tests/integration/cases/<command>/<group>/<NNNN_outcome_case-id>`.
    - Run black-box `help` cases for groups `cases/help/10_general`, `cases/help/15_schema_recovery`, `cases/help/20_errors`, including explicit coverage for `help <command> --show-schema-projection` and non-duplication in `help --show-schema-projection`.
    - Run black-box global-config cases (`--config` explicit path, auto-discovery of `cwd/spec-cli.json`, CLI-over-config priority, `INVALID_CONFIG` failures).
    - Run targeted `help` error-path checks (`CAPABILITY_UNSUPPORTED`, `INVALID_ARGS`) through subprocess invocations.
    - Traverse groups/cases deterministically (lexicographic sort on every level).
    - Validate naming rules `NNNN_ok_*` / `NNNN_err_*` against `expect.exit_code`, `case.json.id`, and `case.json.command`.
    - Delegate shared subprocess/workspace/assert helpers to `tests/integration/internal/harness`.
    - Prepare temporary workspace/schema and execute CLI as subprocess through the integration harness.
    - Support optional case runtime working directory override (`runtime.cwd`) with `${WORKSPACE}` / `${SCHEMA}` placeholders; default working directory remains repository root.
    - Keep entrypoint/data-first case traversal and naming validation in `run_cases_test.go`; keep runtime/assert details in `tests/integration/internal/harness`.
    - Compare `exit_code`, `stderr`, and response (`json|text`) with golden expectations.
    - Treat `workspace.in` as optional for `help` cases; create empty workspace when absent.
    - Compare text-help stdout directly, stabilize `ResolvedPath` through runtime fixed-root injection, and assert the unified `Schema` contract (`Schema` section only; no `Environment`, `SchemaPath`, or `SchemaStatus`).
    - Compare `workspace.out` for mutating scenarios against actual post-command workspace state (ignoring internal `.spec-cli/workspace.lock` service file).
    - Use marker `workspace.in/.keep` for empty input workspaces so they stay in Git and CI.
    - Keep early-fail argument/planner/schema fixtures minimal: cases that fail before workspace scan (for example `INVALID_ARGS`, `INVALID_QUERY`, `ENTITY_TYPE_UNKNOWN`, `SCHEMA_PARSE_ERROR`) should use `workspace.in/.keep` only unless the scenario explicitly validates filesystem/schema/read behavior.
    - For mutating happy-path cases (`add`/`update`/`delete`), keep at least one valid unrelated entity file in `workspace.in/out` to assert absence of collateral file changes.
    - Run extra dynamic black-box test that compares `delete` dry-run and real-run by `target.revision` on clean workspace copies.
    - Run dynamic black-box lock-contention checks for `add`, `update`, `delete` (regular and `--dry-run`) using a dedicated helper process that holds workspace lock.
    - Cover `refs` namespace boundaries and optional-leaf missing semantics: object-level `--select refs` is covered for both `query` and `get`, `refs.<field>` and `refs.<field>.<leaf>` are valid in projection (leaf support is intentionally hidden in help), and `refs.<field>.type|slug=null` behaves as missing in where/sort.
    - Cover scalar and array `entityRef` namespace split in `query`: `meta.<ref_field>` is rejected in both `--select` and `--where`, while ref filters/selectors must use `refs.<field>` / `refs.<field>.<leaf>`.
    - Cover open-ref behavior in `query` when schema omits `refTypes` / `items.refTypes`: scalar and array refs resolve against all schema entity types, and `--where` on `refs.<field>.type` / `refs.<field>[].type` compiles and filters by resolved target entity types.
    - Cover schema-aware `--where` literal validation for built-in entity `type`: unknown literals such as `type == 'unknown'` fail as `INVALID_QUERY` before workspace scan, so their fixtures stay on `workspace.in/.keep`.
    - Cover `add`/`update` array-write contract: `meta.<array_field>` set/replace/unset, `refs.<field>` for `array.items.type=entityRef`, deterministic array-ref diagnostics (`missing|ambiguous|type_mismatch`), and no-partial-write behavior on post-validation failure.
    - Cover explicit projection of built-in `revision` for both `query --select revision` and `get --select revision` with stable opaque tokens in JSON responses.
  - Subpackages:
    - `tests/integration/internal/harness` - shared integration test harness (`case.json` loading, subprocess execution, placeholder/path utilities, stderr/response/workspace assertions, permission setup).
    - `tests/integration/internal/runner` - response normalization and workspace permission adapter used by harness and selected tests.
    - `tests/integration/cases/validate/10_contract/*` - contract scenarios.
    - `tests/integration/cases/validate/20_schema/*` - schema-level scenarios, including `schema.items.refTypes` constraints for arrays and `required` expressions rejected by static nullable-function checks.
    - `tests/integration/cases/validate/30_instance_builtin/*` - built-in entity checks.
    - `tests/integration/cases/validate/40_instance_meta_content/*` - `meta.fields` and `content.sections`.
    - `tests/integration/cases/validate/50_pathTemplate_expr/*` - `pathTemplate.cases[].when` scenarios, including optional-field guards that permit `${meta.<field>}` reuse in `use`.
    - `tests/integration/cases/validate/60_entityRef_context/*` - scalar/array `entityRef`, `items.refTypes`, blank array item handling, `ref.*`, `ref.dirPath`.
    - `tests/integration/cases/validate/70_global_uniqueness/*` - global uniqueness checks.
    - `tests/integration/cases/query/10_basic/*` - basic `query`, including unsupported command-local `--help`.
    - `tests/integration/cases/query/20_select/*` - selector/projection scenarios, including `array.items.type=entityRef` under `refs.<field>`, scalar open-ref resolution when `refTypes` is omitted, and array open-ref resolution when `items.refTypes` is omitted.
    - `tests/integration/cases/query/30_where/*` - `--where` (JMESPath) happy/negative scenarios, including truthy `refs.<field>` filtering when an optional scalar ref is absent from frontmatter, nullable `content.sections.<name>` rejection inside `contains(...)` without a fallback, schema-aware rejection of unknown built-in `type` literals, scalar/array open-ref filtering by `refs.<field>.type` and `refs.<field>[].type` when schema omits `refTypes` / `items.refTypes`, conservative handling of interpolated `schema.const/schema.enum` (no false static reject), and preserved static `const/enum` rejection.
    - `tests/integration/cases/query/40_sort_pagination/*` - sort and pagination.
    - `tests/integration/cases/query/50_errors/*` - argument/global-option validation failures before compile.
    - `tests/integration/cases/query/60_infra/*` - schema/workspace infra failures plus strict shared-compiler blocking cases (`SCHEMA_*`), including malformed schema-type and const/enum mismatch classification.
    - `tests/integration/cases/get/10_contract/*` - `get` contract scenarios.
    - `tests/integration/cases/get/20_select/*` - `get` selector scenarios, including `array.items.type=entityRef` under `refs.<field>` and rejection of array-ref leaf selectors `refs.<field>.<leaf>`.
    - `tests/integration/cases/get/30_lookup/*` - `id` lookup scenarios.
    - `tests/integration/cases/get/40_blocking/*` - blocking read failures.
    - `tests/integration/cases/add/10_happy/*` - happy-path `add`, including expression-based `required` success scenarios and interpolated `schema.enum` acceptance via resolved refs.
    - `tests/integration/cases/add/20_args/*` - `add` CLI errors.
    - `tests/integration/cases/add/30_contract/*` - `add` write-contract failures.
    - `tests/integration/cases/add/40_validation/*` - `add` validation failures, including array constraints, array `entityRef` diagnostics, expression-based `required` failures, and interpolated `schema.const`/`schema.enum` mismatch/interpolation-failure coverage.
    - `tests/integration/cases/add/50_conflict/*` - `add` path conflicts.
    - `tests/integration/cases/add/60_infra/*` - `add` infrastructure and strict shared-compiler blocking failures (`SCHEMA_PARSE_ERROR`, `SCHEMA_INVALID`), including semantic compile errors in unused entity branches before mutation.
    - `tests/integration/cases/update/10_happy/*` - happy-path `update`, including expression-based `required` success scenarios.
    - `tests/integration/cases/update/20_noop/*` - `update` no-op scenarios, including array `entityRef` idempotent set.
    - `tests/integration/cases/update/30_args/*` - `update` argument/conflict failures.
    - `tests/integration/cases/update/40_contract/*` - `update` write-contract failures.
    - `tests/integration/cases/update/50_validation/*` - `update` validation failures, including array-ref failure with no partial writes and expression-based `required` failures.
    - `tests/integration/cases/update/60_lookup/*` - `update` lookup failures.
    - `tests/integration/cases/update/70_concurrency/*` - optimistic concurrency.
    - `tests/integration/cases/update/80_fs/*` - filesystem failures.
    - `tests/integration/cases/update/90_infra/*` - infrastructure failures, including strict global `SCHEMA_INVALID` blocking on unused-entity schema errors before update snapshot/write execution.
    - `tests/integration/cases/delete/10_happy/*` - happy-path `delete`.
    - `tests/integration/cases/delete/20_args/*` - `delete` argument failures.
    - `tests/integration/cases/delete/30_lookup/*` - `delete` lookup diagnostics.
    - `tests/integration/cases/delete/40_concurrency/*` - `delete` concurrency scenarios.
    - `tests/integration/cases/delete/50_refs/*` - reverse-ref blocking, including conservative raw `target.id` blocking from source-declared slots even when `refTypes` does not include the target type.
    - `tests/integration/cases/delete/60_infra/*` - strict shared-compiler failures (`SCHEMA_PARSE_ERROR`, `SCHEMA_NOT_FOUND`, `SCHEMA_INVALID`, `SCHEMA_READ_ERROR`) with diagnostics carried via top-level `schema.issues`, plus post-compile workspace read failure (`READ_FAILED`) that keeps top-level successful `schema` (`valid=true`, `issues=[]`).
    - `tests/integration/cases/delete/70_fs/*` - delete-time filesystem failures.
    - `tests/integration/cases/delete/80_help/*` - unsupported command-local `delete --help`.
    - `tests/integration/cases/version/10_happy/*` - happy-path `version`.
    - `tests/integration/cases/version/20_args/*` - `version` argument failures.
    - `tests/integration/cases/schema/10_happy/*` - happy-path `schema check`.
    - `tests/integration/cases/schema/20_errors/*` - schema source/semantic diagnostics (`read`, `parse`, `duplicate keys`, invalid semantics).
    - `tests/integration/cases/help/10_general/*` - general `help`, `help <command>`, and `help <command> --show-schema-projection`.
    - `tests/integration/cases/help/15_schema_recovery/*` - degraded-schema recovery contract for `help`, including command-help recovery with explicit projection flag.
    - `tests/integration/cases/help/20_errors/*` - non-zero `help` errors.
    - `tests/integration/cases/global_options/10_config/*` - data-first global `--config` contract scenarios (explicit config, auto-discovery, CLI precedence, INVALID_CONFIG failures).
    - `tests/integration/cases/workspace_lock/10_conflict/*` - dedicated lock-contention fixtures for `add/update/delete` (regular and `--dry-run`) used by dynamic helper-process checks.

## Current Command Status

- `help` - shared text-first discovery interface is implemented (`spec-cli help`, `spec-cli help <command>`, `spec-cli help <command> --show-schema-projection`), with schema projection and `--format json` capability gate.
- `schema` - shared schema compiler command is implemented (`spec-cli schema check`): deterministic compile diagnostics, top-level `schema` block (`valid/summary/issues`), no workspace scan, and in-process compile cache per command run.
- `validate` - support for JMESPath `${expr}` (cached compile/evaluate), unified `required`, interpolated `pathTemplate/schema.const/schema.enum`, `refs` runtime context (`dirPath` included), and updated schema/instance diagnostics is implemented.
- `query` - read-only pipeline is implemented on shared schema compile/read capability (`compile -> capability -> adapter index -> plan -> execute`), with global schema-validity blocking, top-level `schema` in all post-compile JSON responses, schema-aware JMESPath `--where`, projection, deterministic sort, and offset pagination.
- `get` - read-one pipeline is implemented on shared schema compile/read capability (`compile -> capability -> selector plan -> target read -> projection`) with global schema-validity blocking, top-level `schema` in all post-compile JSON responses, tolerant target lookup/read semantics, refs/content projection, and stable JSON contract.
- `add` - shared-compiler create pipeline is implemented (`lock -> compile -> write capability -> snapshot -> execute`) with global schema-validity blocking, top-level `schema` in every post-compile JSON response, full pre-write validation, deterministic `id/date/revision`, dry-run, atomic write, array writes in `meta.<field>`, array `entityRef` writes in `refs.<field>`, template-aware `schema.const/schema.enum` runtime resolution, compiler-owned provenance paths in runtime diagnostics, and fail-fast workspace-level writer lock.
- `delete` - baseline delete pipeline is implemented (exact-`id` lookup, `--expect-revision`, reverse-ref blocking, `dry-run`, filesystem delete, JSON contract) with fail-fast workspace-level writer lock.
- `update` - baseline update pipeline is implemented (`--set/--set-file/--unset`, whole-body operations, pre-commit full validation, `--expect-revision`, dry-run, atomic write/move, JSON contract), including JMESPath-based `required: "${expr}"` and `${expr}` path-template evaluation, full-replace array patch semantics, array `entityRef` writes in `refs.<field>`, and fail-fast workspace-level writer lock.
- `version` - baseline version command is implemented (single build-time source `Version` with fallback `dev`, JSON contract, contract error-path cases).
