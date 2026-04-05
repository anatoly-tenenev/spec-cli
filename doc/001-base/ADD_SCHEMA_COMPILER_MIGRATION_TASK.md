# `add` Migration Task: Shared Schema Compiler

## 1. Purpose

This document defines a focused implementation task for migrating `spec-cli add` from its command-local schema loader to the shared schema compiler architecture described in:

- [../SCHEMA_COMPILER_ARCHITECTURE.md](../SCHEMA_COMPILER_ARCHITECTURE.md)
- [../SCHEMA_COMPILER_MIGRATION_PLAN.md](../SCHEMA_COMPILER_MIGRATION_PLAN.md)

The goal is to make `add` a strict schema-aware command with the same compile-first mental model that already exists for `schema check`, `validate`, `query`, and `get`.

## 2. Current State

Today `add` still uses a command-local schema path:

1. `internal/application/commands/add/handler.go`
2. `add/internal/options`
3. `workspacelock.AcquireExclusive`
4. `add/internal/schema.Load`
5. `add/internal/workspace.BuildSnapshot`
6. `add/internal/engine.Execute`

The command-local loader in `internal/application/commands/add/internal/schema/**` still owns:

- raw schema file read / parse / duplicate-key checks;
- semantic interpretation of `idPrefix`, `pathTemplate`, `required`, `meta.fields`, `content.sections`;
- local write-contract projection;
- command-local expression/template compilation based on `internal/application/commands/internal/expressions`.

At the same time, the repository already has the shared layers that must become the source of truth:

- `internal/application/schema/compile`
- `internal/application/schema/model`
- `internal/application/schema/capabilities/write`
- `internal/application/schema/capabilities/validate`
- `internal/output/payload`

`add` is therefore one of the remaining mutating commands that still interprets schema semantics locally.

## 3. Required Outcome

After this task, `add` must follow this execution model:

1. Parse CLI args.
2. Normalize paths.
3. Acquire workspace lock.
4. Run shared `schema.Compile(...)`.
5. Build the shared capability or capabilities needed by `add`.
6. Validate request operations and execute the existing mutation pipeline only against those capabilities.
7. Return JSON response with top-level `schema` after compile has been attempted.

This migration is complete only if all schema semantics used by `add` come from the shared compiler result and shared capabilities, not from a second command-local parser.

## 4. Scope

### 4.1. In Scope

- `internal/application/commands/add/**`
- `internal/application/schema/capabilities/write/**`
- any shared helper needed to let `add` consume compiler-owned expressions/templates;
- `tests/integration/cases/add/**`
- command/unit tests directly touched by the migration;
- `doc/CODEBASE_INDEX.md` if command structure, entrypoints, or subpackage roles change.

### 4.2. Out of Scope

- migrating `update` in the same change;
- changing `ndjson` behavior;
- changing the business meaning of existing `add` scenarios;
- introducing a new compatibility layer for legacy schema syntax;
- preserving legacy `schema.type = null` support in the migrated strict path.

## 5. Fixed Migration Rules

### 5.1. Shared Compiler Is the Only Schema Truth

`add` must stop calling `internal/application/commands/add/internal/schema.Load` on the production path.

Not acceptable:

- keeping `add` on raw YAML parsing and only adding a compile side-check in parallel;
- rebuilding schema meaning from raw YAML after `schema.Compile(...)` already succeeded;
- translating compile diagnostics into command-local synthetic schema issues.

Acceptable:

- a thin adapter from shared compiled schema / shared capabilities into the runtime shape needed by `add`, if that adapter performs no raw-schema parsing and no second semantic interpretation.

### 5.2. Strict Global Schema Validity Applies to `add`

`add` must stop before any workspace snapshot / candidate build / write execution when the shared compile step returns schema errors.

Any schema error anywhere in the schema blocks `add`, even if the current invocation does not touch the broken part of the schema.

### 5.3. Top-Level `schema` Block Becomes Mandatory

After compile has been attempted, every `add` JSON response must include:

- `schema.valid`
- `schema.summary`
- `schema.issues`

This applies to:

- successful creation;
- `dry-run` success;
- compile failure;
- unknown entity type after successful compile;
- write-contract violations after successful compile;
- validation failures after successful compile;
- workspace read/write/path conflict errors after successful compile.

### 5.4. Schema Diagnostics Must Live Only in Top-Level `schema`

When the shared compiler reports schema issues:

- the top-level `schema.issues` array is the canonical place for them;
- `error.details.validation.issues` must not be used to duplicate schema diagnostics;
- legacy command-local `schema.invalid` issue wrapping must be removed from the migrated `add` path.

This is especially important for current schema-error integration cases such as `tests/integration/cases/add/60_infra/0002_err_add_schema_parse_error_json`.

### 5.5. Do Not Translate Shared Expression Objects Back to Command-Local Expression Types

The shared compiler returns expression/template objects from `internal/application/schema/expressions`.

`add` currently uses `internal/application/commands/internal/expressions`.

Do not introduce a converter that re-materializes shared compiler expressions into command-local compiled objects. Instead, migrate the `add` runtime path to consume shared expression/template types directly, or build a runtime adapter that references them directly.

Reason:

- the shared compiler must remain the owner of compiled schema expressions;
- re-compiling or cloning them into a second expression system recreates schema semantics locally.

## 6. Preferred Technical Direction

The migration should keep the `handler.go` layer thin and move detail into narrowly named subpackages.

Preferred direction:

1. Reuse `internal/application/schema/capabilities/write` for write-namespace and write-operation contract.
2. Reuse `internal/application/schema/capabilities/validate` for final entity validation semantics where possible.
3. Keep `add`-local packages only for command-specific runtime work:
   - option parsing;
   - workspace snapshot and indexes;
   - candidate assembly;
   - atomic write;
   - response payload assembly.

If additional shared write capability data is needed, extend `internal/application/schema/capabilities/write` rather than reviving `add/internal/schema`.

Do not create a new universal `common` package.

## 7. Detailed Work Packages

### 7.1. Handler Orchestration

Update `internal/application/commands/add/handler.go` so that it mirrors the shared-compiler flow used by other migrated strict commands.

Required changes:

- inject a compiler factory into `add.Handler`, similar to `validate/query/get`;
- after lock acquisition, run `compiler.Compile(schemaPath, request.Global.SchemaPath)`;
- build `schemaPayload := outputpayload.BuildSchemaPayload(compileResult)` once per request;
- on compile failure, return `error + schema` and stop before type lookup, snapshot, and engine execution;
- on successful compile, perform unknown-type checks against the compiled/capability entity set, not against a command-local loaded schema;
- for every later error path, return the top-level `schema` block together with the normal error payload;
- for success, preserve the existing `add` payload and add top-level `schema`.

Target success shape:

```json
{
  "result_state": "valid",
  "schema": { "...": "..." },
  "dry_run": false,
  "created": true,
  "entity": { "...": "..." },
  "validation": {
    "ok": true,
    "issues": []
  }
}
```

Target compile-failure shape:

```json
{
  "result_state": "invalid",
  "schema": { "...": "..." },
  "error": {
    "code": "SCHEMA_INVALID|SCHEMA_PARSE_ERROR|SCHEMA_NOT_FOUND|SCHEMA_READ_ERROR",
    "message": "...",
    "exit_code": 4|3|1
  }
}
```

The exact `result_state` / `exit_code` must come from the normal shared error mapping, not from new custom logic.

### 7.2. Shared Write Capability Expansion

`internal/application/schema/capabilities/write/builder.go` is currently too thin for `add`.

Extend it so that `add` can derive all write-specific semantics from compiler-owned data without returning to raw YAML.

Minimum data `add` still needs from shared schema semantics:

- entity existence by type;
- `idPrefix`;
- write-path namespace split:
  - `meta.<field>`
  - `refs.<field>`
  - `content.sections.<name>`;
- `SetPaths`;
- `SetFilePaths`;
- whether the type has `content`;
- deterministic meta-field order for serialization / validation loops;
- deterministic section order for body assembly;
- per-field value constraints needed by write parsing:
  - scalar kind;
  - array kind;
  - item kind;
  - scalar ref types;
  - array item ref types;
  - min/max/unique items;
  - const/enum if the write-path validation path still needs them before final validation;
- section titles if body builder still needs the canonical heading label.

If some of this already belongs more naturally to `schema/capabilities/validate`, reuse that capability instead of duplicating the same projection into `write`.

Acceptance rule for this step:

- `write.Build(...)` and any related shared projection tests clearly show which write semantics `add` is allowed to depend on;
- the builder remains an adapter over compiled schema, not a second compiler.

### 7.3. `add` Runtime Model Refactor

Refactor `add` runtime packages so they consume shared capabilities or a thin compiler-backed adapter instead of `model.AddSchema` loaded from raw YAML.

Expected hotspots:

- `internal/application/commands/add/internal/model/types.go`
- `add/internal/engine/internal/writes`
- `add/internal/engine/internal/refresolve`
- `add/internal/engine/internal/pathcalc`
- `add/internal/engine/internal/validation`
- `add/internal/workspace/snapshot.go`

Specific requirements:

- remove the production dependency on `AddSchema` / `EntityTypeSpec` if they remain only as legacy schema-loader output;
- if a command-local runtime model is still needed, build it from compiled schema and shared capabilities only;
- stop importing command-local expression types into the schema-derived runtime shape;
- use shared expression/template types in required/pathTemplate evaluation;
- keep deterministic ordering and deterministic issue sorting exactly as today unless the public contract explicitly changes;
- do not keep dead `null`-type branches alive only because the old add-specific schema model had them.

### 7.4. Workspace Snapshot Input Cleanup

`add/internal/workspace.BuildSnapshot` currently accepts `model.AddSchema` mainly to access `idPrefix` metadata.

Reduce this dependency to the minimum schema-derived input actually required by snapshot building. For example:

- a shared capability map keyed by entity type;
- a thin compiler-backed runtime view containing only `idPrefix` and type presence;
- or the compiled schema itself if that remains the simplest dependency.

Do not leave snapshot coupled to a deprecated add-local schema shape if that shape no longer has an owner.

### 7.5. Validation and Path Evaluation Alignment

The migrated `add` path must continue to validate the fully assembled candidate entity before write, but the schema semantics used there must now come from shared compiler output.

Required behavior to preserve:

- required meta fields and required sections;
- expression-based required checks;
- `pathTemplate` case selection and interpolation;
- scalar `entityRef` and `array.items.type=entityRef` resolution;
- enum / const / type / array item / min / max / unique validation;
- slug/id/date validation;
- duplicate slug / duplicate id / path conflict behavior;
- deterministic validation issue ordering.

Important constraint:

- do not silently weaken validation rules just because the shared compiler now owns the schema;
- do not silently strengthen the meaning of existing non-schema `add` scenarios by converting them into schema failures unless the fixture schema itself is actually invalid under the new global-validity model.

### 7.6. Remove or Demote Command-Local Schema Loader

After migration, `internal/application/commands/add/internal/schema/**` must either:

- be deleted; or
- be reduced to a thin adapter layer that does not read or parse schema files.

What must disappear from `add/internal/schema`:

- filesystem schema read;
- YAML/JSON parse;
- duplicate-key detection;
- top-level schema validation;
- semantic parsing of `pathTemplate`, `required`, `meta.fields`, `content.sections`.

If an adapter remains, rename its role to reflect what it actually does. Do not keep a package named `schema` if it is no longer a schema parser/loader.

## 8. Integration-Test Migration Rules

### 8.1. Preserve Test Meaning

This is the most important migration rule for the test suite:

- existing `add` integration tests must keep testing the same user-visible behavior;
- if the shared compiler starts rejecting a fixture schema that previously happened to pass, fix the fixture schema first;
- do not change the intent of the test just to match the new compiler.

Concrete rule:

- if a pre-existing happy-path / args / contract / validation / conflict case starts failing with `SCHEMA_*`, inspect `spec.schema.yaml`;
- if the new failure is caused by schema invalidity unrelated to the scenario under test, minimally repair the schema fixture and keep the case in the same suite with the same business meaning;
- do not rewrite such a case into a schema-error case;
- do not change the case to assert a different domain error unless the original scenario itself was actually about schema invalidity.

### 8.2. What May Change in Existing Fixtures

Allowed fixture changes:

- `spec.schema.yaml` repairs needed to satisfy the shared compiler's global-validity rules;
- `response.json` updates for the new top-level `schema` block;
- `workspace.out/**` only when the repaired schema still produces the same scenario but necessarily changes representation in a minimal, semantically equivalent way.

Not allowed:

- dropping assertions that express the original user-visible behavior;
- moving a case from validation/contract/happy path into schema-error expectations merely because the compiler became stricter;
- changing CLI args, workspace inputs, or expected output meaning unless the scenario genuinely needs a valid schema fix.

### 8.3. Cases That Should Intentionally Become Schema-Compiler-Driven

Schema-focused failure cases, especially under `tests/integration/cases/add/60_infra/**`, must be updated to the new shared-compiler response contract.

These cases should now verify:

- top-level `schema` payload;
- shared compile diagnostics in `schema.issues`;
- compile error classification via shared `SCHEMA_*` errors;
- absence of legacy duplicated schema issues under `error.details.validation.issues`.

### 8.4. Recommended Audit Loop

Use this loop while migrating the suite:

1. Switch the handler to the shared compiler path.
2. Run the `add` integration suite.
3. Separate failures into:
   - intended response-shape updates (`schema` block added);
   - unintended fixture-schema invalidity;
   - real implementation regressions.
4. Repair invalid fixture schemas minimally.
5. Re-run until all non-schema cases preserve their original meaning.

## 9. Test Plan

### 9.1. Unit / Package-Level Coverage

Add or update unit tests for:

- `add` handler compile-failure path;
- `add` handler post-compile error path with top-level `schema`;
- successful `add` response including top-level `schema`;
- expanded shared write capability builder;
- any new adapter that converts compiled schema/capabilities into an `add` runtime shape;
- runtime expression/path evaluation if it switches to shared schema-expression types.

### 9.2. Integration Coverage

Update existing `tests/integration/cases/add/**` snapshots to cover:

- top-level `schema` in all successful `add` responses;
- top-level `schema` in validation / contract / conflict / infra error responses after successful compile;
- schema parse / schema invalid failures through shared compiler diagnostics;
- unchanged meaning of existing non-schema scenarios.

Do not reduce integration coverage. If fixture schemas need repair, keep the original scenario intent intact.

### 9.3. Mandatory Validation Commands

After implementation:

- run `make vet`
- run `make test`

If the change touches codebase structure, also update `doc/CODEBASE_INDEX.md` in the same change and verify that the diff includes it.

## 10. Acceptance Criteria

The task is complete only if all of the following are true.

1. `add` no longer parses raw schema on the production path through `add/internal/schema.Load`.
2. `add` acquires workspace lock and then runs shared `schema.Compile(...)` before type lookup, snapshot, validation, and write execution.
3. Any compile error blocks `add` globally and returns top-level `error + schema`.
4. Every post-compile `add` JSON response contains top-level `schema`.
5. Schema diagnostics are returned only via top-level `schema.issues`, not duplicated inside `error.details.validation.issues`.
6. `add` runtime validation and path evaluation consume shared compiler-owned semantics, not a second schema parser.
7. Existing non-schema integration cases keep their original meaning; if shared compile strictness exposed invalid fixtures, those fixtures were repaired instead of changing the scenario intent.
8. Existing schema-error integration cases were migrated to the new shared-compiler response contract.
9. Any now-obsolete command-local schema-loader code was removed or demoted to a thin compiler-backed adapter with an explicit role.
10. `make vet` and `make test` pass.

## 11. Review Notes

During review, explicitly check the following:

- there is no hidden raw-schema parsing left under `add` after `schema.Compile(...)`;
- `handler.go` remains orchestration-only;
- if a modified directory root still contains multiple `.go` files, each one is still an entrypoint for a top-level role;
- response snapshots changed only for the intended `schema` block rollout or for minimal schema-fixture repairs that preserve scenario meaning;
- `doc/CODEBASE_INDEX.md` was updated if command roles, entrypoints, or subpackage responsibilities changed.
