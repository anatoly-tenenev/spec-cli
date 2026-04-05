# Unified Schema Compiler Migration Plan

## 1. Purpose

This document describes the migration from the current per-command schema loaders to the unified schema compiler architecture defined in [SCHEMA_COMPILER_ARCHITECTURE.md](./SCHEMA_COMPILER_ARCHITECTURE.md).

The migration is intentionally phased. The goal is to reach the new mental model and response contract without attempting one high-risk all-at-once switch.

## 2. Fixed Decisions

This migration plan assumes the following decisions are already fixed:

1. `help` remains tolerant; all other schema-aware commands are strict.
2. Strict commands run one shared compile step before command execution.
3. Any schema error in any part of the schema blocks every strict schema-aware command.
4. Warnings do not block commands.
5. Strict commands return full schema diagnostics.
6. `schema check` is added as a separate command.
7. `validate` stops before workspace validation when compile produces schema errors.
8. `validate` exposes compile diagnostics in top-level `schema`, and runtime/workspace diagnostics only in `issues`.
9. All strict command responses gain one common top-level `schema` block.
10. Mutating commands keep the order `acquire workspace lock -> compile schema -> build capability -> execute`.
11. `ndjson` changes are out of scope in this migration.
12. `schema.type = null` is not carried forward into the shared compiler model; if it surfaces during migration, remove it from the migrated strict-command path instead of preserving legacy support.
13. For `schema.type = array`, `schema.items` is mandatory in the shared compiler model; legacy `validate` tolerance for arrays without `items` must not be preserved.

## 3. Migration Principles

The migration must follow these rules:

- do not introduce temporary architecture that violates the final boundaries more than the current code does;
- move schema semantics toward the compiler, not sideways into a new shared god-package;
- keep compile diagnostics deterministic from the first migrated phase;
- prefer adapters from new layers into old command handlers during transition;
- update contract/integration coverage as soon as public behavior changes.

## 4. Target End State

At the end of the migration:

- YAML parsing and semantic schema validation live in shared schema packages;
- command-local `internal/schema` loaders are removed or reduced to capability adapters only;
- strict commands consume shared capabilities;
- `schema check` exists as a public command;
- command responses include the top-level `schema` block consistently.

## 5. Phase 0. Preparation

### 5.1. Goals

- record architecture and migration documents;
- freeze scope and decisions;
- identify contract surfaces affected by response-shape changes.

### 5.2. Deliverables

- architecture document;
- migration plan;
- inventory of current command-local schema loaders and tests.

### 5.3. Acceptance Criteria

- design decisions are written down;
- migration phases are ordered and reviewable;
- no production code changes yet.

## 6. Phase 1. Shared Schema Source + Compiler Skeleton

### 6.1. Goals

Introduce the shared foundation without migrating public command behavior immediately beyond the new explicit command.

### 6.2. Work Items

1. Create shared schema packages for:
   - source loading;
   - canonical model;
   - diagnostics;
   - compile entrypoint.
2. Move common bootstrap logic into shared source/compiler packages:
   - file read;
   - YAML/JSON parse;
   - duplicate-key detection;
   - top-level root/entity extraction.
3. Implement deterministic diagnostics collection.
4. Implement canonical model for the minimum subset needed by the first compile consumers.
5. Add in-process compile cache for one command run.

### 6.3. Scope Rule

This phase does not yet require every command to consume the compiler. It creates the shared truth source first.

### 6.4. Acceptance Criteria

- compiler can read schema and emit deterministic diagnostics;
- compiler can represent canonical entity/meta/section/path semantics at least to the extent needed by `schema check`;
- no command is forced onto partial capability builders yet.

## 7. Phase 2. Add `schema check`

### 7.1. Goals

Ship the first public consumer of the new compiler with minimal dependency on existing command flows.

### 7.2. Work Items

1. Add CLI routing and help for `schema check`.
2. Define response shape:
   - same high-level contract family as `validate`;
   - schema diagnostics only;
   - no workspace scan.
3. Add integration cases for:
   - valid schema with zero issues;
   - valid schema with warnings;
   - invalid schema with multiple errors;
   - file-not-found / parse-error / duplicate-key cases.

### 7.3. Acceptance Criteria

- `spec-cli schema check` works end-to-end;
- it uses only the shared compiler for schema interpretation;
- diagnostics order and response shape are stable.

### 7.4. Phase-2 Convergence Checks Before Broad Command Migration

Phase 2 must not publish a public `schema check` command whose schema-validity model is materially weaker than the current strict schema loaders.

Before proceeding to later migration phases, explicitly verify and, if needed, fix the following:

1. `schema check` rejects schema constructs that current strict schema validation already rejects, even before `validate` is fully migrated to the shared compiler.
2. The shared compiler and current strict schema validation stay aligned at least for the schema-validity rules that define whether strict commands may continue.
3. This alignment check includes at minimum:
   - `required: "${expr}"` compilation and expression-context validation;
   - `pathTemplate.use` interpolation validation;
   - `pathTemplate.when` validation;
   - unconditional `pathTemplate` case rules, including count and position;
   - meta-field and section-name validation, including reserved-name handling.
4. Add explicit integration coverage proving that `schema check` rejects representative schemas from the current strict-schema error corpus, not only newly created compiler-only fixtures.
5. Add an explicit happy-path case for a fully valid schema with zero diagnostics and lock the response shape to `"schema.issues": []`, not `null`.

This checkpoint exists to prevent migration drift where `schema check` and the legacy strict loaders disagree about global schema validity during the transition.

## 8. Phase 3. Introduce Shared `schema` Response Block

### 8.1. Goals

Establish the public response contract needed by later command migrations.

### 8.2. Work Items

1. Define one common response fragment for strict commands:
   - `schema.valid`
   - `schema.summary`
   - `schema.issues`
2. Add contract helpers/builders if needed in `internal/output` or equivalent shared response package.
3. Apply the schema block first to `schema check`.
4. Add tests for:
   - success with warnings;
   - compile failure;
   - subsequent command error after successful compile.

### 8.3. Acceptance Criteria

- schema block shape is fixed before broad command migration;
- response builders are shared rather than reimplemented by each migrated command.

## 9. Phase 4. Migrate `validate`

### 9.1. Why `validate` First

`validate` is the natural first strict command because:

- schema diagnostics are already part of its current role;
- it makes the compile/runtime split explicit;
- it exercises the compiler more deeply than read-only commands.

### 9.2. Work Items

1. Replace current command-local schema interpretation with shared `schema.Compile(...)`.
2. Build a shared validation capability from the compiled schema.
3. Stop `validate` before workspace/entity validation when compile produces errors.
4. Move schema diagnostics out of the general `issues` list into top-level `schema`.
5. Keep runtime/workspace validation issues in `issues`.
6. Update integration snapshots and contract tests.

### 9.3. Temporary Adapter Allowance

During this phase it is acceptable to keep parts of the old runtime validation engine if they consume a capability adapter instead of raw command-local schema parsing.

What is not acceptable:

- a second schema parser hidden inside the new validation path;
- duplicated schema diagnostics between `schema` and `issues`.
- reintroducing legacy-only schema forms that are intentionally dropped by the migration, including `schema.type = null` and `schema.type = array` without mandatory `schema.items`.

### 9.4. Acceptance Criteria

- `validate` uses the shared compiler;
- invalid schema blocks workspace validation;
- response cleanly separates compile diagnostics from runtime diagnostics.

## 10. Phase 5. Migrate Read Capabilities: `query` and `get`

### 10.1. Goals

Move read-side commands off command-local schema loaders and onto one shared read capability builder.

### 10.2. Work Items

1. Implement shared read capability builder from compiled schema.
2. Encode:
   - selector namespace;
   - filter/sort namespace;
   - scalar/array ref exposure;
   - field kind and const/enum metadata needed by query planning.
3. Migrate `query` to:
   - compile schema;
   - build read capability;
   - validate `--select`, `--where`, `--sort` against the capability.
4. Migrate `get` to:
   - compile schema;
   - build read capability;
   - validate selectors against the capability.
5. Add integration coverage for:
   - schema error blocks query/get globally;
   - warnings returned in successful responses;
   - non-schema command errors still include top-level `schema`.

### 10.3. Acceptance Criteria

- `query` and `get` no longer parse schema semantics from raw YAML locally;
- shared read capability is the only source of read-side schema rules.

## 11. Phase 6. Migrate Write Capability: `add` and `update`

### 11.1. Goals

Replace near-duplicate write-side schema loaders with one shared write capability builder.

### 11.2. Work Items

1. Implement shared write capability builder from compiled schema.
2. Encode:
   - allowed set paths;
   - allowed unset paths;
   - allowed set-file paths;
   - metadata/ref/section value constraints;
   - path-template evaluation inputs.
3. Migrate `add` and `update` handlers to:
   - acquire workspace lock;
   - compile schema;
   - build write capability;
   - validate operations against the capability;
   - continue existing mutation pipeline.
4. Remove command-local write schema loaders once the new builder covers required semantics.

### 11.3. Special Attention

`add` and `update` currently have the strongest schema-loader duplication. This is the largest deletion opportunity in the migration.

### 11.4. Acceptance Criteria

- `add` and `update` share one write capability source;
- command-specific differences remain at capability consumption level, not schema parsing level;
- lock ordering remains unchanged by decision.

## 12. Phase 7. Migrate Reference Capability: `delete`

### 12.1. Goals

Move `delete` to a shared reference capability built from compiled schema.

### 12.2. Work Items

1. Implement reference capability builder:
   - inbound ref slots by type;
   - scalar vs array cardinality.
2. Migrate `delete` to use:
   - workspace lock;
   - compile step;
   - reference capability;
   - reverse-reference scan.

### 12.3. Acceptance Criteria

- `delete` stops using its local schema parsing logic;
- reverse-reference behavior is driven by shared compiled semantics.

## 13. Phase 8. Reduce or Remove Legacy Command-Local Schema Packages

### 13.1. Goals

Delete obsolete per-command schema loaders and helpers once no command depends on them.

### 13.2. Work Items

1. Remove dead loaders and command-local YAML support duplicated only for schema loading.
2. Keep only command-local adapters that are still necessary at command execution level.
3. Update `doc/CODEBASE_INDEX.md` to reflect the new package map once code changes reach this phase.

### 13.3. Acceptance Criteria

- raw schema parsing is centralized;
- command-local schema packages are removed or reduced to thin adapters over shared capabilities.

## 14. Testing Migration Strategy

Each phase must update tests at the layer where new behavior appears.

### 14.1. Compiler-Level Tests

Add unit coverage for:

- duplicate keys;
- top-level closed-world errors;
- entity normalization;
- `idPrefix` duplicates;
- `refTypes` validation;
- expression compile errors;
- path-template errors;
- deterministic issue ordering.

### 14.2. Capability-Level Tests

Add unit coverage for:

- read selector derivation;
- write path derivation;
- reference slot derivation;
- validation-rule derivation.

### 14.3. Integration Tests

Add or update black-box contract cases for:

- `schema check`;
- strict commands blocked by unrelated schema errors;
- successful strict commands returning schema warnings;
- error responses after successful compile still including `schema`;
- `validate` split between top-level `schema` and runtime `issues`.

## 15. Documentation Migration Strategy

Documentation updates must happen in the same phase where code behavior changes.

Required updates by phase:

- Phase 2: add `schema check` documentation and index entry.
- Phase 3+: update command contract docs that describe response shapes.
- Phase 4+: update `validate` behavior docs.
- Phase 5-7: update command implementation plans if they become materially stale.
- Phase 8: update `doc/CODEBASE_INDEX.md` once command/package structure changes.

## 16. Risks and Controls

### Risk 1. Compiler Becomes a God Object

Control:

- keep compiler limited to canonical semantics and diagnostics;
- capabilities must live in separate builders.

### Risk 2. Hidden Second Parsers Survive in Commands

Control:

- require every migrated command to consume shared capabilities only;
- reject transitional code that re-parses YAML for schema meaning.

### Risk 3. Response Contract Drift During Migration

Control:

- introduce shared schema response block early;
- centralize response block construction before broad command migration.

### Risk 4. Migration Stalls After `schema check`

Control:

- treat Phase 2 as foundation, not completion;
- review follow-up phases against explicit deletion targets in command-local loaders.

## 17. Recommended Delivery Order

Recommended implementation order:

1. shared source/compiler foundation;
2. `schema check`;
3. shared schema response block;
4. `validate`;
5. `query` and `get`;
6. `add` and `update`;
7. `delete`;
8. cleanup of legacy schema loaders.

This order delivers the new mental model early while keeping migration risk manageable.

## 18. Completion Criteria

The migration is complete when all of the following are true:

1. Every strict schema-aware command uses shared `schema.Compile(...)`.
2. Command-local schema loaders no longer define schema semantics independently.
3. Capabilities are built from the compiled canonical schema.
4. `schema check` exists and is fully covered by contract tests.
5. Strict command responses consistently include top-level `schema`.
6. `validate` no longer mixes schema compile diagnostics into runtime `issues`.
7. Documentation and codebase index reflect the new structure.
