# Documentation Index

This page is the single entry point to the `spec-cli` project documentation.

## How to Use It

1. Start documentation lookup from this file.
2. Navigate to the required document through the sections below.
3. Whenever a new document is added to `doc/`, update this index in the same change.

## Documents

## Local Specification (from `spec/`)

- `../spec/SPEC_STANDARD_RU_REVISED_V3.md` - working specification from the `spec/` directory.
- Note: the `spec/` directory is in `.gitignore`; this is expected for a local working artifact.

### General Documents

- [CODEBASE_INDEX.md](./CODEBASE_INDEX.md) - a compact codebase map (agent map) in the `entrypoint + responsibilities + subpackages` format for each layer/package, including the current state of `help`/`add`/`update`/`delete`/`version`, the canonical `help` path contract (`ResolvedPath`), the degraded-schema recovery contract in `help`, the pre-dispatch contract of global CLI options and the integration runner (including data-first groups `cases/help/10_general` without a duplicate default case, `cases/help/15_schema_recovery`, `cases/help/20_errors`, optional `workspace.in` for help cases, `workspace.out` for mutating commands, the `workspace.in/.keep` marker for empty input workspaces, and the split between `run_cases_test.go` entrypoints and `tests/integration/internal/harness` runtime/assert helpers), current `query` namespace coverage for scalar `entityRef` (`meta.<ref_field>` forbidden in `--select`/`--where-json`, `refs.<field>` required), explicit `add`/`update` coverage for arrays and `array.items.type=entityRef` writes via `refs.<field>`, strict `camelCase` reserved-key handling without legacy aliases, plus current `validate` coverage for scalar/array `entityRef`, `schema.items.refTypes` constraints, blank array `entityRef` handling, and dedicated unit-test subpackages.

### 001 Base

- [001-base/SPEC_UTILITY_CLI_PROTOTYPE.md](./001-base/SPEC_UTILITY_CLI_PROTOTYPE.md) - specification of the CLI prototype (`validate`, `query`, `add`, `update`), contract invariants, architectural boundaries, DoD, and the global `--config` JSON contract (including auto-discovery of `cwd/spec-cli.json`).
- [001-base/SPEC_UTILITY_CLI_VERSION_IMPLEMENTATION_PLAN.md](./001-base/SPEC_UTILITY_CLI_VERSION_IMPLEMENTATION_PLAN.md) - implementation plan for the `spec-cli version` command (JSON contract, shared global `--format`, no command-specific `--format`, error codes, and contract-test coverage).
- [001-base/PLAN_GET_IMPLEMENTATION.md](./001-base/PLAN_GET_IMPLEMENTATION.md) - detailed implementation plan for the baseline `spec-cli get` command based on the baseline API and the local working specification.
- [001-base/SPEC_UTILITY_CLI_ADD_IMPLEMENTATION_PLAN.md](./001-base/SPEC_UTILITY_CLI_ADD_IMPLEMENTATION_PLAN.md) - detailed implementation plan for the `spec-cli add` command for the baseline CLI API: write contract from raw schema, pre-write validation, atomic write, and black-box integration tests.
- [001-base/QUERY_IMPLEMENTATION_PLAN.md](./001-base/QUERY_IMPLEMENTATION_PLAN.md) - detailed implementation plan for the `spec-cli query` command on the standard schema (`entity/meta.fields/content.sections`): CLI contract, read namespace, filtering (including `where-json` restrictions for `content.sections.*` and the ban on `content.raw`), sorting, pagination, JSON response, and test plan.
- [001-base/PLAN_UPDATE_IMPLEMENTATION.md](./001-base/PLAN_UPDATE_IMPLEMENTATION.md) - detailed implementation plan for the baseline `spec-cli update` command: patch operations (`--set/--set-file/--unset`), whole-body mode, pre/post validation, optimistic concurrency (`--expect-revision`), atomic write, contract black-box tests, and fixed implementation decisions (payload/errors/hash/path/output scope).
- [001-base/PLAN_DELETE_IMPLEMENTATION.md](./001-base/PLAN_DELETE_IMPLEMENTATION.md) - detailed implementation plan for the baseline `spec-cli delete` command: deletion by `id`, optimistic concurrency (`--expect-revision`), reverse-ref protection of referential integrity, `dry-run`, JSON/error contract, and black-box integration scenarios.
- [001-base/SPEC_UTILITY_CLI_VALIDATE_BASE_IMPLEMENTATION.md](./001-base/SPEC_UTILITY_CLI_VALIDATE_BASE_IMPLEMENTATION.md) - baseline implementation of `spec-cli validate` for the MVP (scope, pipeline, degradation around `expressions`/`entityRef`, `json` format).
- [001-base/SPEC_UTILITY_CLI_VALIDATE_EXPRESSIONS_IMPLEMENTATION_PLAN.md](./001-base/SPEC_UTILITY_CLI_VALIDATE_EXPRESSIONS_IMPLEMENTATION_PLAN.md) - staged plan for full `expressions` support in `spec-cli validate` (compiler/evaluator/context, diagnostics, test plan, completion criteria).
- [001-base/SPEC_UTILITY_CLI_VALIDATE_COMPLETION_PLAN_2026-03-09.md](./001-base/SPEC_UTILITY_CLI_VALIDATE_COMPLETION_PLAN_2026-03-09.md) - final completion plan for `spec-cli validate` (closing `expressions` and `entityRef`, static schema checks, `validator_conformant`, hardening, and testing stages).

### 002 Integration

- [002-integration/INTEGRATION_CASES_LAYOUT.md](./002-integration/INTEGRATION_CASES_LAYOUT.md) - data-first and black-box contract structure for integration cases, the `tests/integration/cases` layout, the case directory content contract (`case.json`, `spec.schema.yaml`, `workspace.in/out`, `response.json`), including grouped suites (`validate/<group>/<case>`, `global_options/<group>/<case>`), optional `runtime.cwd` placeholder support, the naming convention `NNNN_ok_*` / `NNNN_err_*` for `validate` cases, and the `_json` suffix rule for cases using `--format json`.
