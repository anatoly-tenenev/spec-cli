# `spec-cli validate` Completion Plan (Finalization)

Date: 2026-03-09  
Status: working execution plan

## 1. Goal

Finish the `validate` implementation so it matches `SPEC_STANDARD_RU_REVISED_V3.md` and `SPEC_UTILITY_CLI_API_RU.md`, without degradations around `expressions` and `entityRef`.

Target state:

- `required_when` and `pathTemplate.cases[].when` work with the full expression model;
- `entityRef` resolves deterministically and builds the `ref` context;
- static schema consistency rules are enforced (including `pathTemplate` and strict/safe operators);
- `summary.validator_conformant` is set strictly according to the contract;
- the `json/ndjson` format and diagnostic classes (`SchemaError`/`InstanceError`/`ProfileError`) match the specification.

## 2. Baseline and Gap

### 2.1. What Is Already Defined

- There is an MVP document: `SPEC_UTILITY_CLI_VALIDATE_BASE_IMPLEMENTATION.md` (without full `expressions` and `entityRef` support).
- There is a plan for expressions: `SPEC_UTILITY_CLI_VALIDATE_EXPRESSIONS_IMPLEMENTATION_PLAN.md`.
- `V3` and the newer proposals added/clarified requirements for static schema analysis (`pathTemplate`, `exists` guards, forbidding strict operators for potentially-missing values).

### 2.2. What Must Be Closed in Addition to `entityRef`

- Static ban on `eq`/`in` for potentially-missing operands:
  - in `required_when`;
  - in `pathTemplate.cases[].when`.
- Static validation of `pathTemplate.cases[].use`:
  - `{meta:*}`/`{ref:*}` only when the value is statically safe;
  - `exists` in `when` counts as a guard only in allowed forms;
  - unnecessary `exists` for `use` placeholders is a schema error.
- Strict separation of schema-time and runtime errors according to `14.4`.
- Full synchronization of `validator_conformant` with the implementation profile (including deterministic `entityRef/ref` resolution).

## 3. Work Scope

### 3.1. `entityRef` and `ref` Context

- Schema level:
  - validate `schema.type: entityRef`;
  - validate `refTypes` (structure/typing);
  - validate `ref.<field>.<part>` references only for `entityRef` fields.
- Runtime level:
  - dataset entity index for link resolution;
  - deterministic target-selection algorithm;
  - `refTypes` validation against the actual target `type`;
  - populate `ref.<field>.id|type|slug|dirPath`.
- Expression semantics:
  - `meta.<entityRef_field>` is treated as `ref.<field>.id` (after resolution);
  - `exists` for `meta.<entityRef_field>` and `ref.*` depends on successful resolution.

### 3.2. Expressions + Static Analysis

- Compile expressions into AST (schema time).
- Validate operator structure: `eq`, `eq?`, `in`, `in?`, `all`, `any`, `not`, `exists`.
- Classify references: `ALWAYS_AVAILABLE` vs `POTENTIALLY_MISSING`.
- Check strict operators in two modes:
  - `REQUIRED_WHEN`;
  - `PATH_WHEN`.
- Analyze `pathTemplate.cases[].when` + `use` as a single construct:
  - extract the `guard-set`;
  - validate statically safe placeholder usage.

### 3.3. `validate` Pipeline

Order inside one entity:

1. Parse frontmatter + built-ins.
2. Resolve `entityRef` and build the runtime context.
3. `required_when` for `meta.fields`.
4. Validate `meta.fields` values (`type/const/enum` etc.).
5. `required_when` for `content.sections`.
6. `pathTemplate.cases[].when` + case selection + actual path validation.

Finalization:

- compute `issues[]`, `summary`, `result_state`, and exit code;
- `summary.validator_conformant=false` if the implementation profile is non-deterministic or requirement coverage is incomplete.

## 4. Implementation Stages

### Stage 0. Align Contracts (Short)

- Lock the target standard/CLI API sections as the source of truth.
- Approve a stable set of `code` values for new diagnostics.
- Agree on the rollout mode for static rules (`warn` -> `enforce` or enforce immediately).

Definition of Done:

- one unified requirement table and validator-check mapping;
- the `code`/`class`/`standard_ref` list is approved.

### Stage 1. Compiler + Static Schema Checks

- Implement AST and expression compilation.
- Implement static validation of `meta.*`/`ref.*` references.
- Implement `potentially-missing` classification.
- Add schema-time checks:
  - forbid strict `eq`/`in` on potentially-missing operands;
  - enforce `exists` guard rules for `pathTemplate`.

Definition of Done:

- invalid schemas are rejected before document iteration;
- violations derivable from the schema are classified as `SchemaError`.

### Stage 2. `entityRef` Runtime Core

- Entity registry and deterministic link resolution.
- `refTypes` and ambiguity checks.
- Build the `ref` context with `dirPath`.
- Make the resolution profile a mandatory dependency of a conformant mode.

Definition of Done:

- all `entityRef` values either resolve unambiguously or yield a correct `InstanceError`;
- if the profile is unavailable/non-deterministic, a `ProfileError` is produced.

### Stage 3. Evaluator Runtime Semantics

- Implement `Scalar|Missing`.
- Operator behavior:
  - `eq`/`in`: strict;
  - `eq?`/`in?`: safe (`missing -> false`);
  - `all/any/not/exists` with short-circuit.
- Support alias `meta.<entityRef>` -> `ref.<field>.id`.

Definition of Done:

- unit tests cover all operators and `missing` edge cases.

### Stage 4. Integrate into `validate`

- Plug the evaluator into `required_when` (`meta.fields`, `content.sections`).
- Plug evaluation into `pathTemplate.cases[].when`.
- Use one shared context for substitutions and expressions.

Definition of Done:

- validation result is deterministic for identical input;
- there is no silent fallback on strict errors.

### Stage 5. CLI Output and Contract Compatibility

- Verify `json/ndjson` formats (issue records + summary).
- Verify `result_state`, exit codes, `warnings-as-errors`, `fail-fast`.
- Strictly verify `summary.validator_conformant`.

Definition of Done:

- contract `5.1 validate` is satisfied for all supported modes.

### Stage 6. Testing and Hardening

- Unit:
  - compiler/evaluator;
  - `potentially-missing` classification;
  - `pathTemplate` guard logic.
- Integration:
  - `entityRef` + `ref.dirPath` scenarios;
  - `required_when`, `content.sections.required_when`;
  - complex `pathTemplate` with fallback.
- Contract:
  - golden tests for `json/ndjson`;
  - verify diagnostic classes and `standard_ref`.
- Performance:
  - compile expressions once per schema;
  - O(1) lookup for refs and schema fields.

Definition of Done:

- the regression suite passes;
- there is no SLA regression on typical workspaces.

## 5. Minimal New Diagnostic Set (Recommended)

- `schema.expression.invalid_operator`
- `schema.expression.invalid_arity`
- `schema.expression.invalid_operand_type`
- `schema.expression.invalid_reference`
- `schema.required_when.strict_potentially_missing`
- `schema.path_when.strict_potentially_missing`
- `schema.pathTemplate.placeholder_not_guarded`
- `schema.pathTemplate.unused_exists_guard`
- `instance.entityRef.unresolved`
- `instance.entityRef.ref_type_mismatch`
- `profile.entityRef_resolution_unavailable`

Important: the exact `code` strings can be adapted to current naming, but `class` and `standard_ref` must remain normatively correct.

## 6. Completion Criteria (Final)

- The implementation fully covers `expressions` and `entityRef` without MVP degradations.
- Static schema contradictions are caught as `SchemaError` before data iteration.
- Runtime data problems remain `InstanceError`.
- `summary.validator_conformant` reflects the actual completeness of the profile and checks.
- The `validate` format and behavior are compatible with `SPEC_UTILITY_CLI_API_RU.md`.

## 7. Recommended Execution Order

1. Stage 1 (compiler + static checks).
2. Stage 2 (`entityRef` runtime core).
3. Stage 3 (evaluator semantics).
4. Stage 4 (pipeline integration).
5. Stage 5 (contract/output hardening).
6. Stage 6 (full test sweep + performance tuning).
