# Codebase Index (Agent Map)

Compact project map for fast entry into the code.

## Freshness Rule

- Any change to directory structure, entrypoint files, layer roles, or the supported command set must update this file in the same change.

## Fast CLI Execution Path

1. `internal/cli/app.go` - `NewApp`, `(*App).Run`.
2. `internal/application/commandbus/bus.go` - `(*Bus).Dispatch`.
3. `internal/application/commands/<command>/handler.go` - `(*Handler).Handle`.
4. For `help`: `options.Parse` -> `options.NormalizePaths` (canonical `ResolvedPath`) -> `helpschema.LoadReport` -> `helptext.RenderGeneral|RenderCommand`.
5. For `validate`: `options.Parse` -> `schema.Load` -> `workspace.BuildCandidateSet` -> `engine.RunValidation`.
6. For `query`: `options.Parse` -> `schema.Load` -> `schema.BuildIndex` -> `engine.BuildPlan` -> `workspace.LoadEntities` -> `engine.Execute`.
7. For `get`: `options.Parse` -> `schema.LoadReadModel` -> `engine.BuildSelectorPlan` -> `workspace.LocateByID` -> `workspace.ReadTarget` -> `engine.BuildEntityView` -> `engine.ProjectEntity`.
8. For `add`: `options.Parse` -> `options.NormalizePaths` -> `workspacelock.AcquireExclusive` -> `schema.Load` -> `workspace.BuildSnapshot` -> `engine.Execute`.
9. For `update`: `options.Parse` -> `options.NormalizePaths` -> `workspacelock.AcquireExclusive` -> `schema.Load` -> `workspace.BuildSnapshot` -> `engine.Execute`.
10. For `delete`: `options.Parse` -> `options.NormalizePaths` -> `workspacelock.AcquireExclusive` -> `schema.Load` -> `workspace.BuildSnapshot` -> `engine.Execute`.
11. For `version`: `options.Parse` -> `buildinfo.ResolveVersion` -> build payload `result_state/version`.

## Binary Entrypoint Status

- Binary entrypoint: `cmd/spec-cli/main.go` - `main`.
- `main` initializes `cli.NewApp(os.Stdout, os.Stderr, time.Now)` and exits through `os.Exit(app.Run(context.Background(), os.Args[1:]))`.
- `make build` builds `./cmd/spec-cli` into `bin/spec-cli` with `-ldflags` injection of `internal/buildinfo.Version` (default `VERSION=dev`); `make run` executes `./cmd/spec-cli`.

## Layer Map

- `internal/cli`
  - Entrypoint: `internal/cli/app.go` - `NewApp`, `(*App).Run`.
  - Responsibilities:
    - Build the application and register handlers (`help`, `query`, `get`, `add`, `update`, `delete`, `validate`, `version`) through `internal/cli/command_catalog.go`.
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

- `internal/application/commands/validate`
  - Entrypoints:
    - `internal/application/commands/validate/handler.go` - `NewHandler`, `(*Handler).Handle`
    - `internal/application/commands/validate/help.go` - `HelpSpec`
  - Responsibilities:
    - Orchestrate the `validate` pipeline: parse options -> normalize paths -> load schema -> scan workspace -> run engine.
    - Merge schema issues and instance issues into one result.
    - Build the contractual JSON response and `ExitCode`, including `--warnings-as-errors`.
    - Own command help for `validate` inside shared `help`.
  - Subpackages:
    - `validate/internal/options` - option parsing and path normalization.
    - `validate/internal/schema` - schema loading and compile-time validation.
    - `validate/internal/workspace` - markdown workspace scan and frontmatter/content parsing.
    - `validate/internal/expressions` - expression compiler/evaluator.
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

- `internal/application/commands/validate/internal/schema`
  - Entrypoint: `loader.go` - `Load`.
  - Responsibilities:
    - Read the schema file and parse YAML/JSON into AST.
    - Validate root mapping, duplicate keys, and top-level closed-world keys (`version/entity/description`).
    - Fail early on `SchemaError` with level `error` and return `SCHEMA_INVALID` before runtime validation.
    - Delegate `entity.<type>` parsing to the entity subpackage.
  - Subpackages:
    - `validate/internal/schema/internal/entity` - type-level schema parsing.

- `internal/application/commands/validate/internal/schema/internal/entity`
  - Entrypoint: `parser.go` - `ParseType`.
  - Responsibilities:
    - Validate closed-world keys at `entity.<type>` level (`id_prefix`, `path_pattern`, `meta`, `content`, `description`).
    - Parse `id_prefix` and enforce uniqueness across types.
    - Orchestrate parsing of `meta.fields`, `content.sections`, and `path_pattern` through specialized subpackages.
    - Build field maps and expression compile context, and aggregate schema issues from subpackages.
  - Subpackages:
    - `.../metafields` - parse and compile-time checks for `meta.fields` and `meta.fields[].schema`.
    - `.../sections` - parse and compile-time checks for `content.sections`.
    - `.../requiredconstraint` - shared parsing for `required` / `required_when`.
    - `.../expressioncontext` - expression compile context and compile-issue mapping.
    - `.../names` - schema-key validation for `meta.fields` and `content.sections`.
    - `.../pathpattern` - parsing and validation of `path_pattern`.
    - `.../schemachecks` - reusable schema check helpers.
    - `.../tests` - unit tests for schema parsing behavior of `meta.fields[].schema` (`items.type`, `items.refTypes`).

- `internal/application/commands/validate/internal/schema/internal/entity/internal/metafields`
  - Entrypoint: `parser.go` - `Parse`.
  - Responsibilities:
    - Validate `meta`, `meta.fields[]`, and `meta.fields[].schema` structure and closed-world keys.
    - Parse field schema attributes (`type`, `enum`, `const`, `refTypes`, `items`, `uniqueItems`, `minItems`, `maxItems`) and validate links to entity types.
    - Validate `schema.items.refTypes` only for `schema.items.type=entity_ref` (non-empty, no duplicates, known entity types) and keep deterministic sorted `refTypes`.
    - Parse `required` / `required_when` through `requiredconstraint.Parse`.
    - Run static checks for strict `eq/in` operators on potentially-missing operands.
  - Subpackages: none.

- `internal/application/commands/validate/internal/schema/internal/entity/internal/sections`
  - Entrypoint: `parser.go` - `Parse`.
  - Responsibilities:
    - Validate `content.sections` structure and section-rule closed-world keys.
    - Parse section rules (`title`, `required`, `required_when`).
    - Parse section `required` / `required_when` through `requiredconstraint.Parse`.
    - Run static checks for strict `eq/in` operators on potentially-missing operands.
  - Subpackages: none.

- `internal/application/commands/validate/internal/schema/internal/entity/internal/requiredconstraint`
  - Entrypoint: `parser.go` - `Parse`.
  - Responsibilities:
    - Parse shared requiredness rules for fields and sections.
    - Compile object-form `required_when` and return compile issues as schema issues.
    - Enforce invariants `required=true` + `required_when` (error) and implicit `required=false` when only `required_when` is present (warning).
  - Subpackages: none.

- `internal/application/commands/validate/internal/schema/internal/entity/internal/expressioncontext`
  - Entrypoint: `context.go` - `Build`, `FromCompileIssue`, `IsBuiltinMetaField`.
  - Responsibilities:
    - Describe built-in meta fields for compile context (`type`, `id`, `slug`, `created_date`, `updated_date`).
    - Build `expressions.CompileContext` from built-ins and schema-defined `meta.fields`.
    - Map expression compiler issues into schema issue format.
  - Subpackages: none.

- `internal/application/commands/validate/internal/schema/internal/entity/internal/names`
  - Entrypoint: `validate.go` - `ValidateMetaFieldName`, `ValidateSectionName`.
  - Responsibilities:
    - Validate name formats for `meta.fields` and `content.sections`.
    - Forbid overriding built-in meta fields in `meta.fields`.
  - Subpackages: none.

- `internal/application/commands/validate/internal/schema/internal/entity/internal/pathpattern`
  - Entrypoint: `parser.go` - `Parse`.
  - Responsibilities:
    - Normalize `path_pattern` from `string | list | object`.
    - Validate closed-world keys of the `path_pattern` object and `cases[]`.
    - Compile `cases[].when` expressions.
    - Validate placeholders (`meta:*`, `ref:*`), strict operators, static `exists` guards, and the single-unconditional-case-at-end rule.
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

- `internal/application/commands/validate/internal/expressions`
  - Entrypoints:
    - `compiler.go` - `Compile`
    - `evaluator.go` - `Evaluate`
  - Responsibilities:
    - Compile expression AST (`eq`, `eq?`, `in`, `in?`, `all`, `any`, `not`, `exists`) with arity/type/reference checks.
    - Validate `meta/ref` references against compile context.
    - Evaluate strict/safe operators at runtime and report evaluation errors.
  - Subpackages: none.

- `internal/application/commands/validate/internal/engine`
  - Entrypoints:
    - `runner.go` - `RunValidation`
    - `issues.go` - `CountIssuesByLevel`
  - Responsibilities:
    - Run the full validation pipeline: parse candidates, built-in checks, schema-driven checks.
    - Resolve scalar `entity_ref` and `array.items.type=entity_ref` targets (including `refTypes`, `missing|ambiguous|type_mismatch`) and reject blank array `entity_ref` items as item-type mismatches.
    - Evaluate `required_when`, validate `path_pattern`, and keep deterministic issue ordering.
    - Aggregate entity/global issues and compute coverage, validity, and conformance metrics.
  - Subpackages: none.

- `internal/application/commands/query`
  - Entrypoints:
    - `handler.go` - `NewHandler`, `(*Handler).Handle`
    - `help.go` - `HelpSpec`
  - Responsibilities:
    - Orchestrate `query`: parse options -> normalize paths -> load schema -> build schema index -> build query plan -> load workspace views -> execute.
    - Build contractual JSON response (`items`, `matched`, `page`).
    - Keep namespace split in user contract/diagnostics: `projection-namespace` for `--select`, `filter-namespace` for `--sort` and `where-json.field`.
    - Own `query` help inside shared `help`.
    - Provide one place for mapping `INVALID_ARGS`, `INVALID_QUERY`, `ENTITY_TYPE_UNKNOWN`, schema errors, and read errors.
  - Subpackages:
    - `query/internal/options` - `--type`, `--where-json`, `--select`, `--sort`, `--limit`, `--offset`.
    - `query/internal/schema` - standard-schema loading and `QuerySchemaIndex`.
    - `query/internal/workspace` - full read-view building and `refs.<field>` resolution.
    - `query/internal/engine` - planner, `where-json` evaluator, sorting, pagination, projection.
    - `query/internal/model` - internal request/plan/index/AST/response types.
    - `query/internal/support` - pure helpers for YAML, collections, and literal/value operations.

- `internal/application/commands/query/internal/options`
  - Entrypoints:
    - `parse.go` - `Parse`
    - `paths.go` - `NormalizePaths`
  - Responsibilities:
    - Parse query options with defaults (`limit=100`, `offset=0`) and basic `--sort` syntax validation.
    - Return `INVALID_ARGS` centrally for unknown/incomplete arguments.
    - Normalize `workspace/schema` paths with `--require-absolute-paths`.
  - Subpackages: none.

- `internal/application/commands/query/internal/schema`
  - Entrypoints:
    - `loader.go` - `Load`
    - `index.go` - `BuildIndex`
  - Responsibilities:
    - Read schema, parse YAML/JSON, check duplicate keys, and validate minimal `entity` shape.
    - Parse metadata/read/content info for every entity type, including scalar `entity_ref` and `array.items.type=entity_ref` hints (`refTypes` single-type deterministic hint).
    - Build `QuerySchemaIndex` with namespace split: public projection selectors (`refs`, `refs.<name>`) and filter/sort leaf fields (`refs.<name>.resolved|type|id|slug`) used for where/sort and hidden projection compatibility.
    - Exclude scalar and array `entity_ref` from `meta.<name>` selector/filter/sort namespaces and keep type-conflict validation across entity types.
  - Subpackages: none.

- `internal/application/commands/query/internal/workspace`
  - Entrypoint: `loader.go` - `LoadEntities`.
  - Responsibilities:
    - Deterministically scan `.md` files and parse entity frontmatter/body.
    - Build full read-view (`type/id/slug/revision/created_date/updated_date/meta/refs/content.raw/content.sections`).
    - Exclude scalar and array `entity_ref` slots from projected `meta`.
    - Build global `id` index and resolve `refs.<field>` into `{id,resolved,type,slug}` for scalar refs and arrays of this shape for array refs, with `null`/unresolved semantics and deterministic fallback hints.
    - Mark `resolved=true` only when the resolved target is unique and compatible with `refTypes` hint; incompatible unique target keeps unresolved fallback shape.
    - Normalize YAML values (`time.Time` -> `YYYY-MM-DD`) and compute opaque `revision` (`sha256:<hex>`).
  - Subpackages: none.

- `internal/application/commands/query/internal/engine`
  - Entrypoint: `planner.go` - `BuildPlan`.
  - Responsibilities:
    - Validate type filters and build select-tree projector against `projection-namespace`, including hidden compatibility selectors `refs.<field>.id|resolved|type|slug`.
    - Apply default projection (`type`, `id`, `slug`, `meta`, `refs`) when `--select` is omitted.
    - Parse and bind typed `--where-json` AST with `field/op/value`, type, and enum checks against `filter-namespace`.
    - Build effective sort (default + hidden tail) and deterministic comparator for missing values, including `refs.<name>.type/slug` missing semantics when non-deterministic.
    - Execute filter/sort/paginate/project pipeline and build page metadata (`matched`, `returned`, `has_more`, `next_offset`, `effective_sort`).
  - Subpackages: none.

- `internal/application/commands/help`
  - Entrypoints:
    - `handler.go` - `NewHandler`, `(*Handler).Handle`
    - `help.go` - `HelpSpec`
  - Responsibilities:
    - Orchestrate `spec-cli help` and `spec-cli help <command>` through one ordered command catalog.
    - Reject explicit `--format json` for `help` with `CAPABILITY_UNSUPPORTED`.
    - Guarantee that `help` and `help <command>` still succeed on schema problems (`missing|invalid|error`) through degraded `Schema` contract instead of hard failure.
    - Render text-first help with exactly one `Schema` section where `ResolvedPath` and `Status` are always present.
    - Emit stable recovery block (`ReasonCode/Impact/RecoveryClass/RetryCommand`) whenever `Status != loaded`.
    - Return `INVALID_ARGS` for unknown `help <command>`.
  - Subpackages:
    - `help/internal/options` - positional parsing for `help`, canonical schema path, deterministic `ResolvedPath` through fixed-root injection.

- `internal/application/help/helpmodel`
  - Entrypoint: `model.go` - `NewCatalog`, `MustCatalog`, `(*Catalog).Ordered`, `(*Catalog).Find`, `(*Catalog).Names`, `(*Catalog).Has`.
  - Responsibilities:
    - Typed model for command help contract (`CommandSpec`, `PositionalSpec`, `OptionSpec`, `GlobalOptionSpec`).
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
    - Read the effective schema, parse YAML AST, and deterministically project raw schema into CLI-oriented schema view.
    - Classify schema unavailability into `loaded|missing|invalid|error` plus reason codes (`SCHEMA_NOT_FOUND`, `SCHEMA_NOT_READABLE`, `SCHEMA_PARSE_ERROR`, `SCHEMA_VALIDATION_ERROR`, `SCHEMA_PROJECTION_ERROR`).
    - Build degraded-mode recovery contract (`Impact`, `RecoveryClass`, `RetryCommand`) without partial heuristic schema-derived data.
    - Pass absolute `ResolvedPath` into the report.
    - Normalize `meta.fields -> meta|refs`, `required|required_when -> required` (implicit default is `required: true` when both keys are absent), `title -> string[]`.
    - Exclude storage-facing nodes (`path_pattern`, raw `meta.fields`) and hide empty blocks.
    - Emit canonical `required.when` expressions and canonical key order in schema subtree.
  - Subpackages:
    - `helpschema/internal/projector` - thin projection entrypoint and pipeline -> use-case mapping.
    - `helpschema/internal/projector/internal/pipeline` - full parse/project/render pipeline with ordered projection and deterministic YAML emission.
    - `helpschema/internal/projector/internal/pipeline/internal/yamlnodes` - isolated YAML-node primitives, key validation, and projection error mapping.

- `internal/application/help/helptext`
  - Entrypoint: `renderer.go` - `RenderGeneral`, `RenderCommand`.
  - Responsibilities:
    - Render text-first help sections in fixed order.
    - Render normalized options block (positionals before flags, sentinel `none`).
    - Render shared `Schema` section: always print `ResolvedPath`; if `Status=loaded`, print full projection; if degraded, print fixed recovery block.
    - Add an explicit rule in degraded `help <command>` saying concrete schema-derived values for `schema_derived` options are intentionally omitted.
    - Render command blocks without ANSI or table layout.
  - Subpackages: none.

- `internal/application/commands/get`
  - Entrypoints:
    - `handler.go` - `NewHandler`, `(*Handler).Handle`
    - `help.go` - `HelpSpec`
  - Responsibilities:
    - Orchestrate `get`: parse options -> normalize paths -> load schema read-model -> validate selectors -> locate target by `id` -> read target -> build read-view -> project JSON.
    - Build contractual JSON response (`result_state`, `target`, `entity`) for single-entity read.
    - Enforce projection contract for scalar and array `entity_ref`: `meta.<ref_field>` is not selectable; refs are projected via `refs|refs.<name>` and compatible leaf selectors `refs.<name>.id|resolved|type|slug`.
    - Own `get` help inside shared `help`.
    - Handle `INVALID_ARGS`, `SCHEMA_*`, `ENTITY_NOT_FOUND`, `TARGET_AMBIGUOUS`, `READ_FAILED`.
  - Subpackages:
    - `get/internal/options` - parse `--id` and repeatable `--select`, normalize paths.
    - `get/internal/schema` - schema loading and read-model building (`entity types`, `meta/ref/sections`, allowed selectors).
    - `get/internal/workspace` - deterministic scan, fast `id` locator, strict target parsing, section extraction, `revision`.
    - `get/internal/engine` - selector plan/tree, absent-value projection, special-case `content.sections.<name> = null`, read-view build, blocking policy.
    - `get/internal/model` - internal options, read-model, selector-plan, and parsed-target types.
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

- `internal/application/commands/get/internal/schema`
  - Entrypoint: `loader.go` - `LoadReadModel`.
  - Responsibilities:
    - Read schema, parse YAML/JSON, check duplicate keys, and validate minimal `entity` shape.
    - Build read-model from `schema.entity`: non-ref meta fields, scalar `entity_ref` fields, and `array.items.type=entity_ref` fields (with single-`refTypes` deterministic type hint), plus `content.sections` per type.
    - Build canonical selector allowlist (`built-in`, `meta.<non_ref>`, `refs`, `refs.<name>`, `content.raw`, `content.sections`, `content.sections.<name>`).
    - Return `SCHEMA_NOT_FOUND|SCHEMA_PARSE_ERROR|SCHEMA_INVALID` with `validation.issues`.
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
    - Validate selectors against schema read-model and build terminal select tree.
    - Apply default projection (`type`, `id`, `slug`, `meta`, `refs`) when `--select` is omitted.
    - Classify projection requirements (`refs`, `content.raw`, `content.sections`, requested ref/section fields) and apply null policy for `refs.<name>`/`refs.<name>.<leaf>` and `content.sections.<name>`.
    - Build read-view of target entity (`meta`, expanded `refs`, `content`) with refs shape `{id,resolved,type,slug}` for scalar refs and arrays of this shape for array refs, plus unresolved fallback semantics.
    - Mark `resolved=true` only for unique ref target compatible with `refTypes` hint; unique incompatible target stays unresolved with deterministic fallback.
    - Apply blocking policy only when a requested ref slot is structurally unreadable and deterministic `id` cannot be obtained.
    - Project only the selected response subtree.
  - Subpackages: none.

- `internal/application/commands/get/internal/model`
  - Entrypoint: `types.go` - package-visible model types.
  - Responsibilities:
    - Command options (`Options`).
    - Selector-plan types (`SelectNode`, `SelectorPlan`).
    - Schema read-model (`ReadModel`, `EntityTypeSpec`) and workspace read types (`LocateResult`, `ParsedTarget`, `EntityIdentity`).
  - Subpackages: none.

- `internal/application/commands/get/internal/support`
  - Entrypoints:
    - `yaml.go` - `FirstContentNode`, `FindDuplicateMappingKey`, `ToStringMap`
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
    - Orchestrate `add`: parse options -> normalize paths -> acquire workspace lock -> load raw schema -> build workspace snapshot -> execute candidate build/validation/write.
    - Map `INVALID_ARGS`, `CONCURRENCY_CONFLICT`, `WRITE_CONTRACT_VIOLATION`, `VALIDATION_FAILED`, `PATH_CONFLICT`, schema errors, and read/write errors into one JSON envelope.
    - Inject `Clock` for deterministic `created_date`/`updated_date`.
    - Own `add` help inside shared `help`, including the whole-body `--content-file`/`--content-stdin` heading example `## <title> {#<section_name>}`.
  - Subpackages:
    - `add/internal/options` - parse/norm of `--type`, `--slug`, `--set`, `--set-file`, `--content-file`, `--content-stdin`, `--dry-run`.
    - `add/internal/schema` - raw schema loading and local write-contract building.
    - `add/internal/workspace` - deterministic workspace snapshot (`id`/`slug`/suffix indexes, existing paths, parseable identities, `dir_path` context).
    - `add/internal/engine` - apply writes, typed YAML parsing, canonical serialization, `path_pattern` / expression evaluation, full validation, `revision`, dry-run, atomic write.
    - `add/internal/model` - internal use-case types.
    - `add/internal/support` - YAML/deep-copy/literal-compare/stable-collection/error-detail helpers.

- `internal/application/commands/add/internal/options`
  - Entrypoints:
    - `parse.go` - `Parse`
    - `paths.go` - `NormalizePaths`
  - Responsibilities:
    - Parse and validate `add` arguments, including whole-body vs section conflicts, duplicate paths, and mutually exclusive flags.
    - Normalize `workspace/schema` and file-path arguments with `--require-absolute-paths`.
  - Subpackages: none.

- `internal/application/commands/add/internal/schema`
  - Entrypoint: `loader.go` - `Load`.
  - Responsibilities:
    - Read schema, parse YAML/JSON, check duplicate keys, and validate top level.
    - Orchestrate `entity.<type>` parsing and build `AddSchema`.
    - Parse write-contract projection for `meta/refs`, including `array.items.type=entity_ref` and `array.items.refTypes`.
    - Classify schema failures as `SCHEMA_NOT_FOUND|SCHEMA_PARSE_ERROR|SCHEMA_INVALID`.
  - Subpackages:
    - `add/internal/schema/internal/entity` - parse `id_prefix`, `path_pattern`, `meta.fields`, `content.sections`, and build local write contract.

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
    - Run create pipeline: apply writes -> build candidate -> resolve refs/path -> validate -> serialize -> write/dry-run response.
    - Route `WRITE_CONTRACT_VIOLATION`, `VALIDATION_FAILED`, `PATH_CONFLICT`, `WRITE_FAILED`, `INTERNAL_ERROR`.
    - Build contractual `entity` payload and final `result_state=valid`.
  - Subpackages:
    - `.../writes` - apply write ops, typed YAML parsing, write-contract guard rails, Markdown body build.
    - `.../refresolve` - resolve scalar `entity_ref` and `array.items.type=entity_ref` via workspace snapshot and diagnose deterministic `missing|ambiguous|type_mismatch` (indexed for arrays).
    - `.../pathcalc` - evaluate `path_pattern`, interpolate placeholders, normalize, and guard against escaping workspace.
    - `.../validation` - full instance validation and deterministic issue sorting.
    - `.../markdown` - canonical frontmatter/body serialization and `revision` computation (`sha256:*`).
    - `.../storage` - filesystem checks and atomic write.
    - `.../payload` - public `entity` payload (`meta`, `refs`, `content.sections`).
    - `.../expr` - schema-expression evaluator.
    - `.../lookup` - candidate-value lookup adapter for expressions/path evaluation.
    - `.../issues` - `domainvalidation.Issue` factory with entity context.

- `internal/application/commands/add/internal/model`
  - Entrypoint: `types.go` - package-visible command-state structs.
  - Responsibilities:
    - Option and write-operation types.
    - Schema/snapshot/candidate model types.
    - Shared structures passed between `options -> schema -> workspace -> engine`.
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
    - Orchestrate `delete`: parse options -> normalize paths -> acquire workspace lock -> load schema -> build workspace snapshot -> execute delete/checks.
    - Map `INVALID_ARGS`, `SCHEMA_*`, `CONCURRENCY_CONFLICT`, `ENTITY_NOT_FOUND`, `AMBIGUOUS_ENTITY_ID`, `REVISION_UNAVAILABLE`, `DELETE_BLOCKED_BY_REFERENCES`, `WRITE_FAILED`.
    - Own `delete` help inside shared `help`.
  - Subpackages:
    - `delete/internal/options` - parse/norm of `--id`, `--expect-revision`, `--dry-run`.
    - `delete/internal/schema` - raw schema loading and reference-slot extraction for reverse-ref checks.
    - `delete/internal/workspace` - deterministic scan, target lookup by `id`, tolerant frontmatter parse, `revision`.
    - `delete/internal/engine` - target lookup, optimistic concurrency, reverse-ref blocking, dry-run/commit payload.
    - `delete/internal/storage` - file deletion and filesystem error mapping.
    - `delete/internal/model` - internal use-case types.
    - `delete/internal/support` - YAML/map parsing helpers.

- `internal/application/commands/delete/internal/options`
  - Entrypoints:
    - `parse.go` - `Parse`
    - `paths.go` - `NormalizePaths`
  - Responsibilities:
    - Parse and validate `delete` arguments, including `--flag=value`.
    - Enforce required `--id` and normalize boolean `--dry-run`.
    - Normalize `workspace/schema` paths with `--require-absolute-paths`.
  - Subpackages: none.

- `internal/application/commands/delete/internal/schema`
  - Entrypoint: `loader.go` - `Load`.
  - Responsibilities:
    - Read schema, parse YAML/JSON AST, and check duplicate keys.
    - Validate top-level shape (`version/entity/description`) and `schema.entity`.
    - Extract reference slots for `entity_ref` and `array.items.type=entity_ref`.
    - Classify schema failures as `SCHEMA_NOT_FOUND|SCHEMA_PARSE_ERROR|SCHEMA_INVALID`.
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
    - Command options and schema reference-slot model types.
    - Workspace snapshot types.
    - Blocking-reference diagnostics and related enums.
  - Subpackages: none.

- `internal/application/commands/delete/internal/support`
  - Entrypoints:
    - `yaml.go` - `FirstContentNode`, `FindDuplicateMappingKey`, `ToStringMap`
    - `collections.go` - `SortedMapKeys`
  - Responsibilities:
    - YAML AST helpers and duplicate-key checks.
    - Safe conversion of raw YAML values to `map[string]any`.
    - Stable map-key sorting for deterministic schema traversal.
  - Subpackages: none.

- `internal/application/commands/update`
  - Entrypoints:
    - `handler.go` - `NewHandler`, `(*Handler).Handle`
    - `help.go` - `HelpSpec`
  - Responsibilities:
    - Orchestrate `update`: parse options -> normalize paths -> acquire workspace lock -> load schema -> build workspace snapshot -> execute.
    - Support mutating patch (`--set`, `--set-file`, `--unset`, whole-body ops) plus optimistic concurrency (`--expect-revision`).
    - Return contractual JSON response (`updated/noop/changes/entity/validation`) with one domain-error mapping layer.
    - Own `update` help inside shared `help`, including the whole-body `--content-file`/`--content-stdin` heading example `## <title> {#<section_name>}`.
  - Subpackages:
    - `update/internal/options` - parse/norm of `update` options and conflict checks.
    - `update/internal/schema` - raw schema loading and read/write contract building.
    - `update/internal/workspace` - workspace snapshot, frontmatter parsing, section layout.
    - `update/internal/engine` - apply writes, resolve refs/path, full validation, serialize/revision, dry-run/commit.
    - `update/internal/model` - internal use-case types.
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

- `internal/application/commands/update/internal/schema`
  - Entrypoints:
    - `loader.go` - `Load`
    - `internal/entity/parser.go` - `ParseType`
  - Responsibilities:
    - Read schema, parse YAML/JSON, run deep duplicate-key checks, validate top-level shape.
    - Build `Schema.EntityTypes` for `update` (`meta/content/path_pattern/write contract`).
    - Parse `entity.<type>` including `id_prefix`, `path_pattern`, `meta.fields`, `content.sections`, allow-set/allow-unset/allow-set-file paths, required/enum/ref constraints, and `array.items.type=entity_ref` (`items.refTypes`) projection into `refs.<field>`.
    - Classify schema failures as `SCHEMA_NOT_FOUND|SCHEMA_PARSE_ERROR|SCHEMA_INVALID`.
  - Subpackages:
    - `update/internal/schema/internal/entity` - order-aware entity-type parser and write-contract builder.

- `internal/application/commands/update/internal/workspace`
  - Entrypoints:
    - `snapshot.go` - `BuildSnapshot`
    - `frontmatter.go` - `ParseFrontmatter`, `ReadStringField`, `BuildMeta`
    - `sections.go` - `BuildSectionLayout`, `ExtractSections`
  - Responsibilities:
    - Deterministically scan `.md` workspace and build snapshot indexes (`EntitiesByID`, `SlugsByType`, `ExistingPaths`, `TargetMatches`).
    - Locate exact-`id` target candidates with tolerant fallback.
    - Parse frontmatter/body and normalize meta fields for reference/validation context.
    - Extract labeled sections and detect duplicate labels.
  - Subpackages: none.

- `internal/application/commands/update/internal/engine`
  - Entrypoint: `execute.go` - `Execute`.
  - Responsibilities:
    - Resolve target and enforce optimistic concurrency on persisted `revision`.
    - Apply patch ops / whole-body changes, bump `updated_date`, resolve scalar/array `entity_ref`, evaluate `path_pattern`.
    - Run full post-update validation and map deterministic issues into `VALIDATION_FAILED`.
    - Serialize markdown/frontmatter, recompute `revision`, perform dry-run or commit (atomic write / move), build `changes` / `entity` payload.
  - Subpackages:
    - `.../writes` - preflight write-contract checks, typed YAML parsing, section/body patching, deterministic diff.
    - `.../refresolve` - scalar and array `entity_ref` resolution with deterministic `missing|ambiguous|type_mismatch` diagnostics.
    - `.../pathcalc` - `path_pattern` case selection, placeholder interpolation, workspace-boundary guard.
    - `.../validation` - built-in/meta/content/global rules, `required_when`, section-title checks.
    - `.../storage` - atomic write/move, rollback on rename failure, path conflict checks, test-only write-failure injection.
    - `.../markdown` - canonical frontmatter/body serialization and `revision` computation.
    - `.../payload` - public `entity` payload.
    - `.../expr` - expression evaluator (`exists`, `eq/eq?`, `in/in?`, `all`, `any`, `not`).
    - `.../lookup` - path lookup adapter for expressions/path evaluation.
    - `.../issues` - `domainvalidation.Issue` factory with entity context.

- `internal/application/commands/update/internal/model`
  - Entrypoint: `types.go` - package-visible command-state structs.
  - Responsibilities:
    - Patch-option types (`WriteOperation`, `BodyOperation`) and normalized request state.
    - Schema read/write model types (`EntityTypeSpec`, `MetaField`, `SectionSpec`, `PathPattern`, write-path specs).
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
    - Shared assembly of `validation.issues` payload for schema and runtime errors.
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
    - Declare supported commands (`help`, `query`, `get`, `add`, `update`, `delete`, `validate`, `version`).
    - Declare supported output formats (`json`, `text`).
  - Subpackages: none.

- `internal/domain/errors`
  - Entrypoint: `codes.go` - `New`, `ExitCodeFor`.
  - Responsibilities:
    - Store the unified catalog of domain error codes, including help-specific ones (`CAPABILITY_UNSUPPORTED`, `SCHEMA_READ_ERROR`, `SCHEMA_PROJECTION_ERROR`).
    - Build `AppError` with code/message/details/exit_code.
    - Map domain codes to process exit codes.
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

- `internal/output/errormap`
  - Entrypoint: `mapper.go` - `ResultStateForCode`.
  - Responsibilities:
    - Map domain error code to contractual `result_state`.
    - Special-case `CAPABILITY_UNSUPPORTED` / `NOT_IMPLEMENTED` -> `unsupported`.
    - Centralize error-display policy for output layer.
  - Subpackages: none.

- `tests/integration`
  - Entrypoints:
    - `run_cases_test.go` - `TestValidateCases`, `TestQueryCases`, `TestGetCases`, `TestAddCases`, `TestUpdateCases`, `TestDeleteCases`, `TestVersionCases`
    - `help_cases_test.go` - `TestHelpGeneralCases`, `TestHelpSchemaRecoveryCases`, `TestHelpErrorCases`, `TestHelpSelectedCases`
    - `global_options_cases_test.go` - `TestGlobalOptionsCases`
    - `delete_multirun_test.go` - `TestDeleteHappy02DryRunMatchesRealRevision`
    - `workspace_lock_test.go` - `TestMutatingCommandsLockConflict`, `TestMutatingCommandsDryRunRespectsWorkspaceLock`
  - Responsibilities:
    - Run data-first integration cases for `validate`, `query`, `get`, `add`, `update`, `delete`, `version` from `tests/integration/cases/<command>/<group>/<NNNN_outcome_case-id>`.
    - Run black-box `help` cases for groups `cases/help/10_general`, `cases/help/15_schema_recovery`, `cases/help/20_errors`.
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
    - Compare text-help stdout directly and stabilize `ResolvedPath` through runtime fixed-root injection.
    - Compare `workspace.out` for mutating scenarios against actual post-command workspace state (ignoring internal `.spec-cli/workspace.lock` service file).
    - Use marker `workspace.in/.keep` for empty input workspaces so they stay in Git and CI.
    - Run extra dynamic black-box test that compares `delete` dry-run and real-run by `target.revision` on clean workspace copies.
    - Run dynamic black-box lock-contention checks for `add`, `update`, `delete` (regular and `--dry-run`) using a dedicated helper process that holds workspace lock.
    - Cover `refs` namespace boundaries and optional-leaf missing semantics: object-level `--select refs` is covered for both `query` and `get`, `refs.<field>` and `refs.<field>.<leaf>` are valid in projection (leaf support is intentionally hidden in help), and `refs.<field>.type|slug=null` behaves as missing in where/sort.
    - Cover scalar and array `entity_ref` namespace split in `query`: `meta.<ref_field>` is rejected in both `--select` and `--where-json`, while ref filters/selectors must use `refs.<field>` / `refs.<field>.<leaf>`.
    - Cover `add`/`update` array-write contract: `meta.<array_field>` set/replace/unset, `refs.<field>` for `array.items.type=entity_ref`, deterministic array-ref diagnostics (`missing|ambiguous|type_mismatch`), and no-partial-write behavior on post-validation failure.
    - Cover explicit projection of built-in `revision` for both `query --select revision` and `get --select revision` with stable opaque tokens in JSON responses.
  - Subpackages:
    - `tests/integration/internal/harness` - shared integration test harness (`case.json` loading, subprocess execution, placeholder/path utilities, stderr/response/workspace assertions, permission setup).
    - `tests/integration/internal/runner` - response normalization and workspace permission adapter used by harness and selected tests.
    - `tests/integration/cases/validate/10_contract/*` - contract scenarios.
    - `tests/integration/cases/validate/20_schema/*` - schema-level scenarios, including `schema.items.refTypes` constraints for arrays.
    - `tests/integration/cases/validate/30_instance_builtin/*` - built-in entity checks.
    - `tests/integration/cases/validate/40_instance_meta_content/*` - `meta.fields` and `content.sections`.
    - `tests/integration/cases/validate/50_path_pattern_expr/*` - `path_pattern.cases[].when` scenarios.
    - `tests/integration/cases/validate/60_entity_ref_context/*` - scalar/array `entity_ref`, `items.refTypes`, blank array item handling, `ref.*`, `ref.dir_path`.
    - `tests/integration/cases/validate/70_global_uniqueness/*` - global uniqueness checks.
    - `tests/integration/cases/query/10_basic/*` - basic `query`, including unsupported command-local `--help`.
    - `tests/integration/cases/query/20_select/*` - selector/projection scenarios, including `array.items.type=entity_ref` under `refs.<field>`.
    - `tests/integration/cases/query/30_where/*` - `--where-json` happy/negative scenarios.
    - `tests/integration/cases/query/40_sort_pagination/*` - sort and pagination.
    - `tests/integration/cases/get/10_contract/*` - `get` contract scenarios.
    - `tests/integration/cases/get/20_select/*` - `get` selector scenarios, including `array.items.type=entity_ref` under `refs.<field>`.
    - `tests/integration/cases/get/30_lookup/*` - `id` lookup scenarios.
    - `tests/integration/cases/get/40_blocking/*` - blocking read failures.
    - `tests/integration/cases/add/10_happy/*` - happy-path `add`.
    - `tests/integration/cases/add/20_args/*` - `add` CLI errors.
    - `tests/integration/cases/add/30_contract/*` - `add` write-contract failures.
    - `tests/integration/cases/add/40_validation/*` - `add` validation failures, including array constraints and array `entity_ref` diagnostics.
    - `tests/integration/cases/add/50_conflict/*` - `add` path conflicts.
    - `tests/integration/cases/update/10_happy/*` - happy-path `update`.
    - `tests/integration/cases/update/20_noop/*` - `update` no-op scenarios, including array `entity_ref` idempotent set.
    - `tests/integration/cases/update/30_args/*` - `update` argument/conflict failures.
    - `tests/integration/cases/update/40_contract/*` - `update` write-contract failures.
    - `tests/integration/cases/update/50_validation/*` - `update` validation failures, including array-ref failure with no partial writes.
    - `tests/integration/cases/update/60_lookup/*` - `update` lookup failures.
    - `tests/integration/cases/update/70_concurrency/*` - optimistic concurrency.
    - `tests/integration/cases/update/80_fs/*` - filesystem failures.
    - `tests/integration/cases/update/90_infra/*` - infrastructure failures.
    - `tests/integration/cases/delete/10_happy/*` - happy-path `delete`.
    - `tests/integration/cases/delete/20_args/*` - `delete` argument failures.
    - `tests/integration/cases/delete/30_lookup/*` - `delete` lookup diagnostics.
    - `tests/integration/cases/delete/40_concurrency/*` - `delete` concurrency scenarios.
    - `tests/integration/cases/delete/50_refs/*` - reverse-ref blocking.
    - `tests/integration/cases/delete/60_infra/*` - schema/load failures.
    - `tests/integration/cases/delete/70_fs/*` - delete-time filesystem failures.
    - `tests/integration/cases/delete/80_help/*` - unsupported command-local `delete --help`.
    - `tests/integration/cases/version/10_happy/*` - happy-path `version`.
    - `tests/integration/cases/version/20_args/*` - `version` argument failures.
    - `tests/integration/cases/help/10_general/*` - general `help` and `help <command>`.
    - `tests/integration/cases/help/15_schema_recovery/*` - degraded-schema recovery contract for `help`.
    - `tests/integration/cases/help/20_errors/*` - non-zero `help` errors.
    - `tests/integration/cases/global_options/10_config/*` - data-first global `--config` contract scenarios (explicit config, auto-discovery, CLI precedence, INVALID_CONFIG failures).
    - `tests/integration/cases/workspace_lock/10_conflict/*` - dedicated lock-contention fixtures for `add/update/delete` (regular and `--dry-run`) used by dynamic helper-process checks.

## Current Command Status

- `help` - shared text-first discovery interface is implemented (`spec-cli help`, `spec-cli help <command>`), with schema projection and `--format json` capability gate.
- `validate` - support for `expressions`, `entity_ref`, and `path_pattern.cases[].when` is implemented.
- `query` - read-only pipeline is implemented (schema index, `where-json`, projection, deterministic sort, offset pagination, JSON contract).
- `get` - baseline read-one pipeline is implemented (`id` lookup, schema-driven selectors, tolerant read, refs/content projection, JSON contract).
- `add` - baseline create pipeline is implemented (raw-schema write contract, full pre-write validation, deterministic `id/date/revision`, dry-run, atomic write, JSON contract), including explicit support for array writes in `meta.<field>`, array `entity_ref` writes in `refs.<field>`, and fail-fast workspace-level writer lock.
- `delete` - baseline delete pipeline is implemented (exact-`id` lookup, `--expect-revision`, reverse-ref blocking, `dry-run`, filesystem delete, JSON contract) with fail-fast workspace-level writer lock.
- `update` - baseline update pipeline is implemented (`--set/--set-file/--unset`, whole-body operations, pre-commit full validation, `--expect-revision`, dry-run, atomic write/move, JSON contract), including full-replace array patch semantics, array `entity_ref` writes in `refs.<field>`, and fail-fast workspace-level writer lock.
- `version` - baseline version command is implemented (single build-time source `Version` with fallback `dev`, JSON contract, contract error-path cases).
