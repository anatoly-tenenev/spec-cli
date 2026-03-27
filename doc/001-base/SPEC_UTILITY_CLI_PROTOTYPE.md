# `spec-cli` Prototype (`validate`, `query`, `add`, `update` focus)

This document describes the CLI utility prototype based on the requirements in [SPEC_UTILITY_CLI_API_RU.md](./SPEC_UTILITY_CLI_API_RU.md).
Goal: lock down a minimally useful implementation with a machine-stable contract and an architecture ready for extension.

## 1. Prototype Scope

The prototype includes only these commands:

- `validate`
- `query`
- `add`
- `update`

Prototype constraints:

- the utility is machine-first (primary client: AI agent / CI);
- non-interactive by default;
- mandatory support for `--format json` and `--format ndjson` for all 4 commands;
- shared `result_state`, `error.code`, `error.exit_code`;
- mandatory support for `revision` in responses containing `entity`.

## 2. Core Contract Principles

### 2.1. Invocation Format

```bash
spec-cli [global options] <command> [command options]
```

Prototype global options:

- `--workspace <path>` (default: `.`)
- `--schema <path>` (default: `spec.schema.yaml`, resolved from `cwd`)
- `--format <json|ndjson>`
- `--config <path>`
- `--require-absolute-paths`
- `--verbose`

`--config` rules:

- `--config` points to a JSON file with defaults for global options;
- only `schema` and `workspace` keys are allowed in the MVP config;
- if `--config` is omitted, the CLI auto-loads `cwd/spec-cli.json` when that file exists;
- if `--config` is omitted and `cwd/spec-cli.json` is absent, the command continues without config;
- relative `schema`/`workspace` values from config are resolved from the active config file directory;
- value priority: explicit CLI flag > `--config` > built-in default;
- built-in defaults remain `workspace="."` and `schema="spec.schema.yaml"`;
- if `--config` is explicit, missing/unreadable/unparseable config and unknown keys fail with `INVALID_CONFIG`;
- if `--config` is omitted, unreadable/unparseable auto-found `cwd/spec-cli.json` and unknown keys fail with `INVALID_CONFIG`.

`--verbose` rules:

- `--verbose` does not change required fields or contract semantics;
- an optional `diagnostics` object may be added in `json/ndjson`;
- `diagnostics` is allowed in the top-level `result`, in `summary`, and in `error.details.debug`.

### 2.2. Mandatory Invariants

- In `json/ndjson`, data is printed to `stdout`, diagnostics to `stderr`.
- The CLI does not prompt when arguments are missing.
- Contracts do not expose internal entity filesystem paths.
- `query` uses deterministic default sorting: `type asc`, then `id asc`.
- Every `json` response and NDJSON `result/summary/error` record contains `result_state`.

Allowed `result_state` values:

- `valid`
- `invalid`
- `partially_valid`
- `not_found`
- `unsupported`
- `indeterminate`

### 2.3. NDJSON Pattern

- For single-response commands (`add`, `update`), the stream contains `record_type: "result"` and a final `record_type: "summary"`.
- For collections (`query`), data is emitted as `record_type: "item"`, then `summary`.
- For validation (`validate`), data is emitted as `record_type: "issue"`, then `summary`.
- On an error before the stream starts: one line with `record_type: "error"` and no `summary`.
- On a runtime error after a partial stream: a final line with `record_type: "error"` and `result_state: "indeterminate"` without `summary`.

### 2.4. Concurrency and `revision`

- `revision` is a computed opaque token of the document state (frontmatter + content).
- `revision` is returned in responses containing `entity`.
- `revision` is used as a machine version marker for later state comparison.

## 3. Prototype Command Contracts

### 3.1. `validate`

Purpose:

- schema validation;
- document validation against the schema;
- uniqueness checks for `slug`/`id`;
- validation of `meta` and `content.sections`.

Syntax:

```bash
spec-cli validate
```

Prototype command options:

- none (validation always runs in fixed full mode)

Key rules:

- the entire workspace is always validated (`validation_scope=full`);
- coverage is always strict (`coverage.mode=strict`);
- content validation is always enabled;
- there is no incremental/partial validation mode in the prototype.

Minimal JSON contract:

```json
{
  "result_state": "invalid",
  "validation_scope": "full",
  "summary": {
    "schema_valid": true,
    "validator_conformant": true,
    "entities_scanned": 42,
    "entities_valid": 39,
    "errors": 3,
    "warnings": 1,
    "coverage": {
      "mode": "strict",
      "complete": true
    }
  },
  "issues": []
}
```

### 3.2. `query`

Purpose:

- structured entity search by built-in fields, `meta.*`, and reference fields.

Syntax:

```bash
spec-cli query [options] --where-json <json>
```

Prototype options:

- `--type <entity_type>` (repeatable)
- `--where-json <json>` or `--where-file <path>` (exactly one source)
- `--fields <preset>`
- `--select <field>` (repeatable)
- `--limit`, `--offset` (offset pagination)
- `--sort <field[:asc|desc]>` (repeatable)
- `--count-only`
- `--validate-query`

Supported operators:

- `eq`, `neq`, `in`, `not_in`, `exists`, `not_exists`, `gt`, `gte`, `lt`, `lte`, `contains`
- logical nodes: `and`, `or`, `not`

Key rules:

- unknown operator/field -> `INVALID_QUERY`;
- incompatible comparison types -> `INVALID_QUERY`;
- `enum` validation is case-sensitive;
- custom `--sort` gets a stable hidden tail `type:asc,id:asc` if absent.
- with `--count-only`, `items` is returned as an empty array;
- with `--count-only`, `matched` is mandatory and reflects the number of matches before pagination;
- with `--count-only`, `page.returned` is `0`.

Minimal JSON contract:

```json
{
  "result_state": "valid",
  "items": [],
  "matched": 0,
  "page": {
    "mode": "offset",
    "limit": 100,
    "offset": 0,
    "returned": 0,
    "has_more": false,
    "next_offset": null,
    "effective_sort": ["type:asc", "id:asc"]
  }
}
```

### 3.3. `add`

Purpose:

- create a new entity/document.

Syntax:

```bash
spec-cli add [options]
```

Required options:

- `--type <entity_type>`
- `--slug <slug>`

Prototype core options:

- reference context: `--ref-field`, `--ref-id`
- metadata: `--meta`, `--meta-json`, `--meta-file`
- content: `--content-file`, `--content-stdin`
- mode: `--dry-run`

Key rules:

- `--content-file` and `--content-stdin` are mutually exclusive;
- `--meta-json` and `--meta-file` are mutually exclusive, `--meta` complements/overrides them;
- `id`, `createdDate`, `updatedDate` are always computed automatically;
- if the entity type requires a hierarchical link (`entityRef`) for placement/validation, reference context is mandatory;
- if a reference is provided (`--ref-id`), `--ref-field` is mandatory, otherwise `INVALID_ARGS`;
- `--ref-field` without `--ref-id` is not allowed (`INVALID_ARGS`);
- if the type requires reference context, `--ref-field` is always required, even if there is only one suitable field in the schema;
- if the type does not require reference context and `--ref-id` is not passed, creation without a reference is allowed;
- `--ref-id` is resolved globally (global `id` uniqueness across types is assumed);
- if the target path already exists, the command fails with `PATH_CONFLICT`;
- pre-validation and post-validation are always enabled;
- writes are atomic: if post-validation fails, disk state is rolled back.

Minimal JSON contract:

```json
{
  "result_state": "valid",
  "dry_run": false,
  "created": true,
  "file": {
    "existed_before": false
  },
  "entity": {
    "type": "feature",
    "id": "FEAT-8",
    "slug": "retry-window",
    "createdDate": "2026-02-25",
    "updatedDate": "2026-02-25",
    "revision": "sha256:abc123",
    "metadata": {}
  },
  "validation": {
    "before_write_ok": true,
    "after_write_ok": true,
    "issues": []
  }
}
```

### 3.4. `update`

Purpose:

- patch-update an existing entity without implicitly replacing the whole document.

Syntax:

```bash
spec-cli update [options]
```

Required identification:

- `--id <id>`

Prototype core options:

- patch metadata: `--set-meta`, `--set-meta-json`, `--set-meta-file`, `--unset-meta`
- patch content: `--content-file`, `--content-stdin`, `--clear-content`
- patch sections: `--set-section`, `--set-section-file`, `--clear-section`
- mode: `--dry-run`

Key rules:

- only explicitly specified fields are changed;
- conflicting patch sources -> `INVALID_ARGS`;
- if no patch operations are passed -> `INVALID_ARGS`;
- pre-validation and post-validation are always enabled;
- idempotence is mandatory: repeating the same patch may return a successful `noop`;
- writes are atomic: post-validation failure must not leave a partially written file.

Minimal JSON contract:

```json
{
  "result_state": "valid",
  "dry_run": false,
  "updated": true,
  "noop": false,
  "target": {
    "match_by": "id",
    "id": "FEAT-8"
  },
  "file": {
    "written": true
  },
  "changes": [
    {
      "field": "metadata.status",
      "op": "set",
      "before": "draft",
      "after": "active"
    }
  ],
  "entity": {
    "type": "feature",
    "id": "FEAT-8",
    "slug": "retry-window",
    "createdDate": "2026-02-25",
    "updatedDate": "2026-02-26",
    "revision": "sha256:def456",
    "metadata": {}
  },
  "validation": {
    "before_write_ok": true,
    "after_write_ok": true,
    "issues": []
  }
}
```

## 4. Errors and Exit Codes (Prototype)

Base exit codes:

- `0` success
- `1` command domain error
- `2` argument/query error
- `3` read/write error
- `4` schema error
- `5` internal error

Minimal JSON error format:

```json
{
  "result_state": "invalid",
  "error": {
    "code": "INVALID_QUERY",
    "message": "Invalid structured filter node",
    "exit_code": 2,
    "details": {
      "arg": "--where-json"
    }
  }
}
```

Codes that must already be implemented in the prototype:

- `INVALID_ARGS`
- `INVALID_CONFIG`
- `SCHEMA_NOT_FOUND`
- `SCHEMA_PARSE_ERROR`
- `SCHEMA_INVALID`
- `ENTITY_TYPE_UNKNOWN`
- `ENTITY_NOT_FOUND`
- `TARGET_AMBIGUOUS`
- `PATH_CONFLICT`
- `ID_CONFLICT`
- `SLUG_CONFLICT`
- `INVALID_QUERY`
- `WRITE_FAILED`
- `INTERNAL_ERROR`

## 5. Extensible Prototype Architecture

### 5.0 Technology Choice

- Prototype implementation language: `Go` (target baseline: `Go 1.24+`).
- Distribution format: one CLI binary `spec-cli`.
- Standard library preferred; external dependencies only for clear value (for example, CLI routing).

### 5.1 Architectural Style

`Hexagonal + Command Bus`:

- the CLI layer only parses arguments and builds `CommandRequest`;
- each use case (`validate/query/add/update`) is implemented as a separate command handler;
- filesystem, schema, serialization, index, and time access go through ports (interfaces);
- output (`json/ndjson`) is separated from business logic.

Benefits:

- new commands (`delete/move/rename/...`) are easy to add without core refactoring;
- the use case can be reused in API/daemon mode;
- contract testing is simplified.

### 5.2 Layers and Modules

Recommended structure:

```text
cmd/
  spec-cli/
    main.go
internal/
  cli/
    app.go
    global_options.go
    router.go
  application/
    commandbus/
      bus.go
    commands/
      validate/
        handler.go
      query/
        handler.go
      add/
        handler.go
      update/
        handler.go
  domain/
    entity/
      model.go
    query/
      ast.go
      validator.go
      evaluator.go
    validation/
      issue.go
    errors/
      codes.go
  infrastructure/
    fs/
    schema/
    markdown/
    index/
    clock/
  output/
    jsonwriter/
    ndjsonwriter/
    errormap/
  contracts/
    responses/
    capabilities/
```

### 5.3 Key Domain Contracts

- `Entity`: `type`, `id`, `slug`, `createdDate`, `updatedDate`, `revision`, `metadata`, `content`.
- `RevisionService`: computes opaque `revision`.
- `QueryAst` + `QueryValidator` + `QueryEvaluator`.
- `ValidationIssue`: `class`, `message`, `standard_ref`, `severity`.
- `MutationPlan` for `add/update`: describes computed changes before write.

### 5.4 Shared Pipeline for Write Commands

`add` and `update` share one scenario:

1. parse args -> `CommandRequest`;
2. preflight (schema/load/index/target resolve);
3. build `MutationPlan`;
4. pre-validation;
5. `dry-run` response or atomic write;
6. post-validation;
7. rollback on error;
8. build a stable JSON/NDJSON response.

This pipeline makes future write commands (`move/rename/retarget/delete/apply`) easier to add by reusing steps 2-8.

### 5.5 Extension Points

- command registry (`CommandRegistry`) with declarative registration;
- capability flag registry (`CapabilitiesProvider`) with auto-generated `capabilities`;
- extensible query operator catalog (new `op` values without breaking existing ones);
- validator plugins (additional rules/profiles);
- storage adapter abstraction (local FS now, remote backend later);
- new output writers for additional machine formats without changing command handlers.

## 6. Prototype Implementation Plan

1. CLI skeleton, global options, error mapping, `json/ndjson` writers.
2. Baseline infra modules: schema loader, Markdown parser/serializer, entity index.
3. `validate` with `summary/issues` and NDJSON `issue + summary`.
4. `query` with `where-json`, AST/validator/evaluator, paging/sort.
5. `add` with automatic built-in field computation, dry-run, atomic write, pre/post validation.
6. `update` with patch semantics, `changes[]`, no-op.
7. Contract tests and golden snapshots for JSON/NDJSON responses.

## 7. Definition of Done for the Prototype

- All 4 commands are implemented and work reliably in `--format json` and `--format ndjson`.
- All responses contain a correct `result_state`.
- `query` is deterministic when the user does not provide sorting.
- `add/update` support `dry-run`, pre/post validation, rollback, and atomicity.
- `update` returns `changes[]`.
- Errors match the unified JSON/NDJSON contract and the `error.code -> exit_code` mapping.
- There are tests for positive and negative scenarios of every command.

## 8. Data-First Integration Cases

The detailed convention for the structure of integration cases (the `tests/integration/cases` catalog, per-command structure, case directory contract, `case.json`/`response.json` format, `workspace.in/workspace.out` rules) is described in:

- [INTEGRATION_CASES_LAYOUT.md](../002-integration/INTEGRATION_CASES_LAYOUT.md)
