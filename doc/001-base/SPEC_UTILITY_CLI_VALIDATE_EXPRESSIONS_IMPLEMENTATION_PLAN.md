# `expressions` Implementation Plan for `spec-cli validate`

## 1. Goal

This document defines a realistic transition plan from the MVP (`SPEC_UTILITY_CLI_VALIDATE_BASE_IMPLEMENTATION.md`) to full support for standard expressions in the `validate` command.

Target result:

- `required_when` is evaluated according to the standard for `meta.fields[]` and `content.sections[]`;
- `path_pattern.cases[].when` is evaluated using the same expression model;
- the semantics of `missing`, strict/safe operators, and the `meta/ref` context are respected;
- diagnostics land in the correct classes (`SchemaError` / `InstanceError` / `ProfileError`);
- `summary.validator_conformant=true` only when the profile is deterministic and the requirements are fully covered.

## 2. Normative Sources

The implementation must rely strictly on:

- `SPEC_STANDARD_RU_REVISED_V3.md`:
  - section `8` (`path_pattern`, `cases[].when`);
  - section `9` (placeholders and `ref:*`);
  - section `11.5` (requiredness model);
  - section `11.6` (expression operators, `missing`, typing);
  - section `12.3` (`entity_ref` resolution and `ref` context);
  - sections `14.3`/`14.4` (`Validator-conformant`, diagnostic classes).
- `SPEC_UTILITY_CLI_API_RU.md`:
  - section `5.1 validate` (`summary.validator_conformant`, CLI contract, and output formats).

## 3. Implementation Scope

### 3.1. Included

- Object-form expressions with exactly one operator:
  - `eq`, `eq?`, `in`, `in?`, `all`, `any`, `not`, `exists`.
- Boolean `required_when` (literal `true/false`) and object form.
- Left-to-right evaluation of `path_pattern.cases[].when`.
- Full expression context:
  - `meta.<field_name>` (including built-in fields);
  - `ref.<field_name>.<part>` (`id|type|slug|dir_path`).
- `meta.<entity_ref_field>` semantics as an alias for `ref.<field_name>.id` (after resolution).
- `eq`/`in` behavior on `missing` as `InstanceError`.

### 3.2. Excluded from This Plan

- New operators/DSL beyond the standard.
- Relaxation of strict typing.
- Changes to the `validate` `json/ndjson` output contract.

## 4. Architecture

### 4.1. New Layer: `Expression Engine`

Introduce a dedicated layer with two phases:

1. `compile` (once per schema):
   - parse and normalize expressions into AST;
   - perform structural validation (operator, arity, literal types, non-empty lists);
   - statically validate `meta.*`/`ref.*` references against the schema;
   - bind `standard_ref` for future diagnostics.
2. `evaluate` (for each entity):
   - evaluate the AST in the context of a concrete entity instance;
   - return a boolean result or a domain error (diagnostics).

Recommended modules:

- `validation/expressions/ast.*`
- `validation/expressions/compiler.*`
- `validation/expressions/evaluator.*`
- `validation/expressions/context.*`

### 4.2. Evaluation Context

The per-entity context must be built after:

1. parsing frontmatter;
2. validating built-in fields (`type/id/slug/date`);
3. resolving `entity_ref` (at minimum: target index + `refTypes` checks + `dir_path`).

Context structure:

- `meta`: map of literal frontmatter fields;
- `refs`: map of resolved links with `id/type/slug/dir_path` attributes;
- `presence`:
  - for `meta.<non_entity_ref>`: the key exists in YAML (`null` counts as present);
  - for `meta.<entity_ref>` and `ref.*`: present only when the link resolves successfully.

### 4.3. Evaluation Semantics (Core)

Introduce an internal operand result type:

- `Scalar(value)` (`string|number|boolean|null`);
- `Missing`.

Core rules:

- `eq` / `in`:
  - if any operand is `Missing` -> `InstanceError` (section `11.6`); the expression is treated as non-evaluable for that entity;
- `eq?` / `in?`:
  - if any operand is `Missing` -> result `false`, with no separate error;
- `exists`:
  - uses the presence rules from `11.6`;
- `all` / `any`:
  - short-circuit left to right;
- `not`:
  - boolean negation of the child expression result.

Strict comparison typing:

- only scalars are allowed;
- `integer` and `number` are compared by numeric value;
- `null` equals only `null`;
- any `array/object` literal in expressions -> `SchemaError` at compile time.

## 5. Integration into the Current `validate` Pipeline

### 5.1. Pipeline Changes

1. `Load schema`:
   - add `compileExpressions(schema)` and collect schema diagnostics.
2. `Parse documents`:
   - no contract changes, but preserve data for expressions in `EntityRuntimeContext`.
3. `Validate schema rules vs entity`:
   - evaluate `meta.fields.required_when` through the expression evaluator;
   - evaluate `content.sections.required_when` through the expression evaluator;
   - string `const/enum` with placeholders continues to use the shared context (`meta/ref`) and a single value resolver.
4. `Path checks`:
   - select the `path_pattern` case via `cases[].when`;
   - on `InstanceError` in a strict operator, do not automatically move to the next case;
   - record the problem as an entity violation.
5. `Finalize summary`:
   - `validator_conformant=false` if the `entity_ref/ref` resolution profile is not loaded or is non-deterministic.

### 5.2. Evaluation Order Inside One Entity

1. built-ins + basic frontmatter typing;
2. `entity_ref` type/resolution and `ref` context construction;
3. `required_when` for `meta.fields`;
4. schema checks of `meta.fields` values (`type/const/enum/...`);
5. `required_when` for `content.sections`;
6. `path_pattern.cases[].when` and path validation.

This order avoids false `missing` on `ref.*` and prevents cyclic dependencies.

## 6. Diagnostics

### 6.1. Classification (Mandatory)

- `SchemaError`:
  - invalid expression structure;
  - unknown operator;
  - invalid arity;
  - invalid `meta.*`/`ref.*` references;
  - invalid literal type constraints.
- `InstanceError`:
  - `eq`/`in` receives `missing`;
  - `exists`/links point to an unresolved `entity_ref` in entity data;
  - `path_pattern` selection fails due to a non-evaluable strict condition.
- `ProfileError`:
  - no deterministic `entity_ref/ref` resolution profile is available.

### 6.2. Recommended Minimal Set of Code IDs

- `schema.expression.invalid_operator`
- `schema.expression.invalid_arity`
- `schema.expression.invalid_operand_type`
- `schema.expression.invalid_reference`
- `instance.expression.missing_operand_strict`
- `instance.path_pattern.when_evaluation_failed`
- `profile.expression_context_unavailable`

The naming can be adapted to the current conventions, but the semantics and class must remain intact.

## 7. Rollout Plan by Stage

### Stage 1. Expression Compiler (Schema Time)

- AST + operator parser.
- Expression structure and schema reference validation.
- Feed schema diagnostics into the shared `issues[]`.

Completion criterion:

- invalid expressions are detected before document iteration begins.

### Stage 2. Context and Dependency Resolution

- Shared `EntityRuntimeContext`.
- `entity_ref` resolution with deterministic `dir_path`.
- Explicit implementation-profile check/flag.

Completion criterion:

- `meta`/`ref` sources are available to the evaluator.

### Stage 3. Evaluator + `required_when`

- Implement the semantics of `eq/eq?/in/in?/all/any/not/exists`.
- Integrate evaluation into requiredness for `meta.fields` and `content.sections`.

Completion criterion:

- conditional requiredness works per the standard, including `missing`.

### Stage 4. `path_pattern.cases[].when`

- Canonicalize `path_pattern` and evaluate cases.
- Correctly handle strict errors without silent fallback.

Completion criterion:

- the selected template is always deterministic and explainable by diagnostics.

### Stage 5. Output, Compatibility, Hardening

- Final calibration of `code` IDs and `standard_ref`.
- Verify the `json/ndjson` contract and exit codes.
- Optimization (AST cache, O(1) ref lookup, short-circuit).

Completion criterion:

- the regression suite passes and the output format is unchanged.

## 8. Test Plan (Minimum)

### 8.1. Unit: compiler/evaluator

- each operator in the happy path;
- invalid arity and literal types;
- `eq`/`in` + `missing` -> `InstanceError`;
- `eq?`/`in?` + `missing` -> `false`.

### 8.2. Integration: validate

- `meta.fields.required_when` on built-in and user-defined fields;
- `content.sections.required_when`;
- `path_pattern` with multiple `cases` and mixed strict/safe conditions;
- scenarios with `entity_ref` + `ref.<field>.dir_path`.

### 8.3. Contract

- `--format json` and `--format ndjson` (issue + summary);
- `summary.validator_conformant` when the profile is present/absent;
- `--fail-fast` and `--warnings-as-errors`.

## 9. Risks and Mitigations

- Risk: non-deterministic `entity_ref` resolution breaks the `ref` context.
  - Mitigation: explicit profile validation before the validate run, `ProfileError` + `validator_conformant=false`.
- Risk: cascading errors from strict operators.
  - Mitigation: one shared policy of "one primary evaluation error per expression + limit secondary messages".
- Risk: performance degradation on large workspaces.
  - Mitigation: compile expressions once, cache schema lookups, use short-circuit evaluation.

## 10. Completion Criteria

- All required expression operators are implemented and covered by tests.
- `required_when` and `path_pattern.cases[].when` work deterministically.
- Diagnostic classes match section `14.4`.
- `summary.validator_conformant` correctly reflects profile completeness.
- `validate` output remains compatible with the API contract (`json/ndjson`, exit codes).
