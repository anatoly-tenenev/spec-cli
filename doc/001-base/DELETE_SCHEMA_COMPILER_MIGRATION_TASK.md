# `delete` Migration Task: Shared Schema Compiler

## 1. Purpose

This document defines a focused implementation task for migrating `spec-cli delete` from its command-local schema loader to the shared schema compiler architecture described in:

- [../SCHEMA_COMPILER_ARCHITECTURE.md](../SCHEMA_COMPILER_ARCHITECTURE.md)
- [../SCHEMA_COMPILER_MIGRATION_PLAN.md](../SCHEMA_COMPILER_MIGRATION_PLAN.md)

The goal is to make `delete` follow the same strict compile-first model that is already used by `schema check`, `validate`, `query`, `get`, `add`, and `update`, while preserving the current delete-specific tolerant workspace behavior and the meaning of the existing integration suite.

## 2. Current State

Today `delete` still uses a command-local schema path:

1. `internal/application/commands/delete/handler.go`
2. `delete/internal/options`
3. `workspacelock.AcquireExclusive`
4. `delete/internal/schema.Load`
5. `delete/internal/workspace.BuildSnapshot`
6. `delete/internal/engine.Execute`

The command-local loader in `internal/application/commands/delete/internal/schema/**` still owns:

- raw schema file read / parse / duplicate-key checks;
- top-level schema shape validation;
- extraction of schema-declared reference slots for reverse-ref checks;
- command-local classification of `SCHEMA_NOT_FOUND | SCHEMA_PARSE_ERROR | SCHEMA_INVALID`.

At the same time, the repository already has the shared layers that must become the source of truth:

- `internal/application/schema/compile`
- `internal/application/schema/model`
- `internal/application/schema/capabilities/references`
- `internal/output/payload`

`delete` is therefore the remaining strict command that still derives schema meaning from raw YAML on the production path.

## 3. Required Outcome

After this task, `delete` must follow this execution model:

1. Parse CLI args.
2. Normalize paths.
3. Acquire workspace lock.
4. Run shared `schema.Compile(...)`.
5. Build the shared capability or capabilities needed by `delete`.
6. Build the tolerant workspace snapshot.
7. Execute delete-specific lookup / concurrency / reverse-ref / dry-run / commit logic only against shared compiler-owned schema semantics.
8. Return JSON response with top-level `schema` after compile has been attempted.

This migration is complete only if all schema semantics used by `delete` come from the shared compiler result and shared capabilities, not from a second command-local parser or a hidden raw-schema fallback.

## 4. Scope

### 4.1. In Scope

- `internal/application/commands/delete/**`
- `internal/application/schema/capabilities/references/**`
- any shared helper needed to let `delete` consume compiler-owned reference semantics;
- `tests/integration/cases/delete/**`
- `tests/integration/delete_multirun_test.go`
- command/unit tests directly touched by the migration;
- `doc/CODEBASE_INDEX.md` if command structure, entrypoints, or subpackage roles change.

### 4.2. Out of Scope

- migrating any other command in the same change;
- changing `ndjson` behavior;
- weakening the current tolerant workspace behavior of `delete`;
- changing the business meaning of existing `delete` scenarios;
- introducing a new compatibility layer for legacy schema syntax;
- preserving legacy `schema.type = null` support in the migrated strict path;
- preserving arrays without mandatory `schema.items` on the migrated strict path.

## 5. Fixed Migration Rules

### 5.1. Shared Compiler Is the Only Schema Truth

`delete` must stop calling `internal/application/commands/delete/internal/schema.Load` on the production path.

Not acceptable:

- keeping the local loader and merely adding `schema.Compile(...)` as a side-check;
- parsing raw YAML again after successful compile;
- rebuilding schema meaning from raw YAML inside `engine`, `workspace`, or a new helper package;
- reclassifying compiler diagnostics through command-local schema parsing.

Acceptable:

- a thin adapter from shared compiled schema / shared capabilities into the runtime shape needed by `delete`, if that adapter performs no raw-schema parsing and no second semantic interpretation.

### 5.2. Strict Global Schema Validity Applies to `delete`

`delete` must stop before workspace snapshot / target lookup / reverse-ref analysis when the shared compile step returns schema errors.

Any schema error anywhere in the schema blocks `delete`, even if the current invocation would only touch one entity and even if the broken schema fragment is unrelated to the current target.

Operational order is fixed:

1. acquire workspace lock;
2. compile schema;
3. only then build snapshot and run delete logic.

### 5.3. Top-Level `schema` Block Becomes Mandatory After Compile

After compile has been attempted, every `delete` JSON response must include:

- `schema.valid`
- `schema.summary`
- `schema.issues`

This applies to:

- successful delete;
- successful `dry-run`;
- compile failure;
- `ENTITY_NOT_FOUND` after successful compile;
- `AMBIGUOUS_ENTITY_ID` after successful compile;
- `REVISION_UNAVAILABLE` after successful compile;
- `CONCURRENCY_CONFLICT` after successful compile;
- `DELETE_BLOCKED_BY_REFERENCES` after successful compile;
- filesystem delete failures after successful compile.

This does not apply to failures that happen before compile is attempted, for example:

- argument parsing failures;
- path normalization failures;
- workspace lock acquisition failures.

### 5.4. Schema Diagnostics Must Live Only in Top-Level `schema`

When the shared compiler reports schema issues:

- the top-level `schema.issues` array is the canonical place for them;
- `error.details` must not duplicate compiler diagnostics through a legacy delete-local schema wrapper;
- the migrated path must use shared compile failure classification and shared issue payloads.

This is especially important for `tests/integration/cases/delete/60_infra/**`.

### 5.5. Preserve Tolerant Workspace Semantics

The migration must not make `delete` depend on a fully valid workspace.

Required behavior to preserve:

- unrelated unparseable documents do not block delete;
- the target may still be deletable if it is data-invalid but parseable far enough to obtain `type`, `id`, and `revision`;
- reverse-ref checks still inspect parseable documents even if those documents are otherwise invalid;
- lookup by exact `id`, optimistic concurrency, `dry-run`, and filesystem delete semantics stay unchanged unless the public contract explicitly changes.

The shared compiler makes schema handling stricter; it must not silently make workspace handling stricter.

### 5.6. Preserve Conservative Reverse-Reference Blocking

This point is easy to regress and must be treated as a fixed contract.

The current `delete` implementation blocks deletion on any exact raw `target.id` match found in a schema-declared reference slot of a parseable source document:

- scalar `entityRef`;
- `array.items.type = entityRef`.

This blocking is currently driven by declared source slots and raw `id` matching, not by a re-validation of whether the target entity type satisfies `refTypes`.

Therefore the migration must not accidentally narrow blocking behavior by switching to logic such as:

- `referencesCapability.InboundByTargetType[target.Type]` only;
- target-type filtering that skips schema-declared source slots which were previously scanned;
- dependence on the target being schema-valid beyond the currently required `type/id/revision`.

If the existing shared references capability is too target-type-centric for `delete`, extend it with the capability shape `delete` actually needs, for example deterministic source-type slot views, rather than reviving the local schema loader.

### 5.7. No Hidden Schema Parsing May Survive in Delete Runtime Packages

After migration, no delete-local runtime package may still be parsing raw schema or reconstructing ref slot meaning from YAML.

Specifically review:

- `delete/internal/engine`
- `delete/internal/workspace`
- `delete/internal/model`
- any newly added adapter package

The only schema-derived inputs allowed there are compiled schema, shared capabilities, or thin compiler-backed adapters over them.

## 6. Preferred Technical Direction

The migration should keep `handler.go` thin and reuse the existing shared pieces already used by `add` and `update`.

Preferred direction:

1. Mirror the orchestration pattern from `internal/application/commands/add/handler.go` and `internal/application/commands/update/handler.go`.
2. Reuse `internal/output/payload.BuildSchemaPayload(...)` and the existing `error + schema` response assembly style.
3. Reuse `internal/application/schema/capabilities/references` for reverse-ref semantics.
4. If `references.Build(...)` is insufficient for `delete`, extend that shared capability instead of reviving `delete/internal/schema`.
5. Keep delete-local packages only for command-specific runtime work:
   - option parsing;
   - tolerant workspace snapshot;
   - target lookup;
   - optimistic concurrency;
   - raw frontmatter slot matching against `target.id`;
   - filesystem delete;
   - response payload assembly.

Do not create a new universal `common` package.

## 7. Detailed Work Packages

### 7.1. Handler Orchestration

Update `internal/application/commands/delete/handler.go` so that it mirrors the shared-compiler flow already used by migrated strict commands.

Required changes:

- inject a compiler factory into `delete.Handler`, similar to `add` / `update` / `validate`;
- after lock acquisition, run `compiler.Compile(schemaPath, request.Global.SchemaPath)`;
- build `schemaPayload := outputpayload.BuildSchemaPayload(compileResult)` once per request;
- on compile failure, return top-level `error + schema` and stop before snapshot and engine execution;
- on successful compile, build the shared references capability or compiler-backed delete adapter;
- on every later error path, return the top-level `schema` block together with the normal error payload;
- on success, preserve the existing `delete` payload and add top-level `schema`.

Target success shape:

```json
{
  "result_state": "valid",
  "schema": { "...": "..." },
  "dry_run": false,
  "deleted": true,
  "target": {
    "id": "SVC-2",
    "revision": "sha256:..."
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

The exact `result_state` and `exit_code` must come from the normal shared error mapping, not from new custom logic.

### 7.2. Shared References Capability Alignment

`internal/application/schema/capabilities/references/builder.go` already exists, but it must be evaluated against the real needs of `delete`, not only against an architectural ideal.

Minimum requirements for the delete migration:

- deterministic visibility of all schema-declared scalar `entityRef` source slots;
- deterministic visibility of all schema-declared `array.items.type = entityRef` source slots;
- enough capability data to let `delete` scan parseable source documents by source type without returning to raw YAML;
- preservation of existing conservative blocking semantics described in section 5.6.

If the current `InboundByTargetType` view is not sufficient by itself, extend the capability with an additional deterministic view such as:

- `SlotsBySourceType`;
- or another explicit source-driven projection.

Acceptance rule for this step:

- the shared references capability clearly documents which source-slot semantics `delete` is allowed to consume;
- the builder remains an adapter over compiled schema, not a second compiler;
- tests lock the scalar/array coverage and deterministic ordering.

### 7.3. `delete` Runtime Model Refactor

Refactor delete runtime packages so they consume shared capabilities or a thin compiler-backed adapter instead of `model.Schema` loaded from raw YAML.

Expected hotspots:

- `internal/application/commands/delete/internal/model/types.go`
- `internal/application/commands/delete/internal/engine/execute.go`
- `internal/application/commands/delete/internal/schema/**`

Specific requirements:

- remove the production dependency on `model.Schema{ReferenceSlotsByType: ...}` if it remains only as local loader output;
- if a delete-local runtime shape is still needed, build it from compiled schema and shared capabilities only;
- keep stable sorting of `blocking_refs`;
- keep exact domain codes and payload shapes for non-schema delete errors;
- keep `slotMatchesTarget` semantics unchanged unless an explicit public contract change is requested.

### 7.4. Snapshot and Target Resolution Boundaries

`delete/internal/workspace.BuildSnapshot` should remain schema-agnostic and tolerant.

Required behavior to preserve:

- deterministic scan order;
- tolerant `TargetMatches` discovery;
- `revision` computation from persisted bytes;
- ignoring unrelated unparseable documents;
- target recoverability rules for `REVISION_UNAVAILABLE`.

Required boundary change:

- compile failure must stop `delete` before snapshot build;
- snapshot code must not start reading or inferring schema semantics as a side effect of the migration.

### 7.5. Remove or Demote Command-Local Schema Loader

After migration, `internal/application/commands/delete/internal/schema/**` must either:

- be deleted; or
- be reduced to a thin adapter layer that does not read or parse schema files.

What must disappear from `delete/internal/schema`:

- filesystem schema read;
- YAML/JSON parse;
- duplicate-key detection;
- top-level schema validation;
- semantic parsing of reference slots from raw schema nodes.

If an adapter remains, rename its role to reflect what it actually does. Do not keep a package named `schema` if it is no longer a schema parser/loader.

## 8. Integration-Test Migration Rules

### 8.1. Preserve Test Meaning

This is the most important migration rule for the suite:

- existing `delete` integration tests must keep testing the same user-visible behavior;
- if the shared compiler starts rejecting a fixture schema that previously happened to pass, fix the fixture schema first;
- do not change the intent of the test just to match the new compiler.

Concrete rule:

- if a pre-existing happy-path / lookup / concurrency / refs / filesystem case starts failing with `SCHEMA_*`, inspect `spec.schema.yaml`;
- if the new failure is caused by schema invalidity unrelated to the scenario under test, minimally repair the schema fixture and keep the case in the same suite with the same business meaning;
- do not rewrite such a case into a schema-error case;
- do not change the expected domain error merely because the compiler became stricter.

This rule applies directly to the current delete suite, where most cases intentionally test non-schema behavior and should stay that way.

### 8.2. What May Change in Existing Fixtures

Allowed fixture changes:

- `spec.schema.yaml` repairs needed to satisfy the shared compiler's global-validity rules;
- `response.json` updates for the new top-level `schema` block;
- `workspace.out/**` only when the repaired schema still produces the same scenario but necessarily changes representation in a minimal, semantically equivalent way.

Not allowed:

- dropping assertions that express the original user-visible behavior;
- moving a happy / lookup / concurrency / refs / fs case into schema-error expectations merely because the compiler became stricter;
- changing CLI args, workspace inputs, or expected output meaning unless the scenario genuinely needs a valid-schema repair.

### 8.3. Cases That Should Intentionally Become Schema-Compiler-Driven

Schema-focused failure cases under `tests/integration/cases/delete/60_infra/**` must be updated to the new shared-compiler response contract.

These cases should now verify:

- top-level `schema` payload;
- shared compile diagnostics in `schema.issues`;
- compile error classification via shared `SCHEMA_*` errors;
- absence of any legacy duplicate schema diagnostics elsewhere in the payload.

### 8.4. Delete-Specific Audit Guidance

Use the following audit expectations while migrating the current delete suite:

- `tests/integration/cases/delete/20_args/**` is pre-compile and should keep testing argument failures without a top-level `schema` block;
- post-compile suites such as `10_happy`, `30_lookup`, `40_concurrency`, `50_refs`, and `70_fs` should gain top-level `schema` but keep their original scenario meaning;
- `tests/integration/delete_multirun_test.go` should continue to compare dry-run and real-run payloads for equivalence except `dry_run`, which now also implicitly covers the shared `schema` block;
- if any currently valid delete fixture becomes compiler-invalid, repair the schema fixture instead of changing the scenario category.

### 8.5. Recommended Audit Loop

Use this loop while migrating the suite:

1. Switch the handler to the shared compiler path.
2. Run the `delete` integration suite.
3. Separate failures into:
   - intended response-shape updates (`schema` block added);
   - unintended fixture-schema invalidity;
   - real implementation regressions.
4. Repair invalid fixture schemas minimally.
5. Re-run until all non-schema cases preserve their original meaning.

## 9. Test Plan

### 9.1. Unit / Package-Level Coverage

Add or update unit tests for:

- `delete` handler compile-failure path;
- `delete` handler post-compile error path with top-level `schema`;
- successful `delete` response including top-level `schema`;
- expanded shared references capability builder;
- any new adapter that converts compiled schema / capabilities into a delete runtime shape;
- engine-level reverse-ref behavior after the runtime model switch;
- conservative blocking behavior so migration does not accidentally narrow checks through target-type filtering;
- removal or demotion of the local schema loader.

### 9.2. Integration Coverage

Update existing `tests/integration/cases/delete/**` snapshots to cover:

- top-level `schema` in all successful post-compile `delete` responses;
- top-level `schema` in post-compile non-schema error responses;
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

1. `delete` no longer parses raw schema on the production path through `delete/internal/schema.Load`.
2. `delete` acquires workspace lock and then runs shared `schema.Compile(...)` before snapshot build and delete execution.
3. Any compile error blocks `delete` globally and returns top-level `error + schema`.
4. Every post-compile `delete` JSON response contains top-level `schema`.
5. Failures that happen before compile remains pre-compile responses and do not gain an accidental `schema` block.
6. Schema diagnostics are returned only via top-level `schema.issues`, not duplicated elsewhere in the payload.
7. Reverse-ref checks consume shared compiler-owned semantics, not a second raw-schema parser.
8. Reverse-ref blocking semantics are not silently narrowed by target-type filtering or `refTypes` filtering compared with the current command behavior.
9. Existing non-schema integration cases keep their original meaning; if shared compile strictness exposed invalid fixtures, those fixtures were repaired instead of changing the scenario intent.
10. Existing schema-error integration cases were migrated to the new shared-compiler response contract.
11. Any now-obsolete command-local schema-loader code was removed or demoted to a thin compiler-backed adapter with an explicit role.
12. `make vet` and `make test` pass.

## 11. Review Notes

During review, explicitly check the following:

- there is no hidden raw-schema parsing left under `delete` after `schema.Compile(...)`;
- the reverse-ref implementation still scans the same effective set of schema-declared source slots as before unless an explicit contract change was requested;
- `handler.go` remains orchestration-only;
- if a modified directory root still contains multiple `.go` files, each one is still an entrypoint for a top-level role;
- response snapshots changed only for the intended `schema` block rollout or for minimal schema-fixture repairs that preserve scenario meaning;
- `doc/CODEBASE_INDEX.md` was updated if command roles, entrypoints, or subpackage responsibilities changed.
