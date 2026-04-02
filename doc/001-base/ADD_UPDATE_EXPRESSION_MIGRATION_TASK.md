# `add` / `update` Migration Task: Current Expression Logic

## 1. Purpose

This document defines a focused implementation task for bringing `spec-cli add` and `spec-cli update` to the current local standard in the areas where they still diverge from it:

- `required` must use the current `boolean | ${expr}` model;
- `pathTemplate.cases[].when` must use the same `boolean | ${expr}` model;
- `pathTemplate.use` must use `${expr}` interpolation instead of legacy `{path}` placeholders;
- integration tests for `add` and `update` must be migrated to the new schema syntax;
- `add` and `update` must gain explicit integration coverage for `required: ${expr}`.

The normative target is the current local standard in `spec/SPEC_STANDARD_RU_REVISED_V3.md`, especially:

- section `11.5` for the unified `required` model;
- section `11.6` for `${expr}` and JMESPath semantics;
- the `pathTemplate` rules that rely on the same expression model.

## 2. Scope

In scope:

- `internal/application/commands/add/**`
- `internal/application/commands/update/**`
- `tests/integration/cases/add/**`
- `tests/integration/cases/update/**`

Out of scope for this task:

- `validate` changes, except using it as the semantic reference implementation;
- `help` schema projection;
- renaming legacy `validate` case identifiers that still contain `required_when`.

## 3. Current Divergence

`add` and `update` still implement an obsolete expression model in three places.

### 3.1. `required_when` Instead of Unified `required`

The current standard no longer defines `required_when`.
The supported form is:

```yaml
required: true | false | ${expr}
```

Current `add` / `update` behavior still assumes:

- `required` is only `boolean`;
- `required_when` is a separate key;
- `required_when` uses a local object DSL instead of JMESPath.

### 3.2. Legacy Object DSL in `pathTemplate.cases[].when`

The current standard requires:

```yaml
when: true | false | ${expr}
```

Current `add` / `update` behavior still evaluates `when` through the local operator DSL:

- `exists`
- `eq`, `eq?`
- `in`, `in?`
- `all`
- `any`
- `not`

This is not equivalent to the standard JMESPath contract.

### 3.3. Legacy `{...}` Placeholders in `pathTemplate.use`

The current standard uses `${expr}` interpolation in string contexts.

Current `add` / `update` `pathTemplate.use` still uses legacy placeholders such as:

```yaml
use: "{refs.container.dirPath}/features/{slug}.md"
```

instead of:

```yaml
use: "${refs.container.dirPath}/features/${slug}.md"
```

## 4. Migration Rules

No backward compatibility is required.

After this task:

- `required_when` must not be supported in `add` / `update`;
- the legacy object DSL in `pathTemplate.cases[].when` must not be supported in `add` / `update`;
- legacy `{...}` placeholders in `pathTemplate.use` must not be supported in `add` / `update`;
- the commands must not emit compatibility warnings or deprecation notices for these legacy forms;
- old forms must simply fail as schema-invalid input.

## 5. Required Implementation Changes

### 5.1. Unify `required` Semantics with the Current Standard

For both `add` and `update`:

- parse `required` as `boolean | ${expr}`;
- apply default `required = true` when the key is absent;
- remove any parsing and runtime handling of `required_when`;
- evaluate `required: ${expr}` with JMESPath truthiness, not with a boolean-only interpretation;
- use the same runtime context model as the standard:
  - top-level built-ins: `type`, `id`, `slug`, `createdDate`, `updatedDate`;
  - `meta`;
  - `refs`.

Implementation note:

- `validate` already contains the correct semantic model for `required` and should be treated as the reference behavior;
- `add` / `update` should not keep a separate semantic fork once this migration is complete.

### 5.2. Replace Legacy `pathTemplate.when` Evaluation

For both `add` and `update`:

- parse `pathTemplate.cases[].when` as `boolean | ${expr}`;
- reject any non-boolean and non-`${expr}` value during schema load;
- evaluate `when` with JMESPath truthiness;
- stop using the local operator DSL for `pathTemplate.when`.

The observable behavior must match the standard model already used by `validate`.

### 5.3. Replace Legacy `pathTemplate.use` Placeholder Rendering

For both `add` and `update`:

- stop parsing `{placeholder}` syntax in `pathTemplate.use`;
- compile and render `${expr}` interpolation instead;
- use the same interpolation contract as the current standard;
- reject invalid interpolation during schema load when it can be determined statically;
- keep workspace-boundary and canonical-path checks after interpolation.

### 5.4. Remove the Legacy Local Expression DSL from `add` / `update`

The local `expr` packages under `add` and `update` are no longer the target behavior for schema expressions.

After migration, there must be no command-level semantic dependency on the old operator DSL for:

- `required`;
- `pathTemplate.cases[].when`.

If any reusable expression logic is extracted, it must be command-neutral and reflect the current standard, not the legacy DSL.

### 5.5. Keep `add` and `update` Aligned With Each Other

The migration must not be done for only one mutating command.

Acceptance condition:

- `add` and `update` must accept the same schema syntax for `required` and `pathTemplate`;
- `add` and `update` must reject the same invalid schema forms in these areas;
- `add` and `update` must produce the same semantic result as `validate` for equivalent schemas and entity states.

## 6. Test Migration for Existing `add` / `update` Suites

The existing integration suites for `add` and `update` still use legacy schema syntax extensively and must be migrated.

### 6.1. Bulk Fixture Rewrite

Across `tests/integration/cases/add/**` and `tests/integration/cases/update/**`:

- replace legacy `required_when` with `required: ${expr}` where conditional requiredness is intended;
- replace legacy object-DSL `pathTemplate.cases[].when` with `when: ${expr}` or boolean literals;
- replace legacy `{slug}` / `{meta.foo}` / `{refs.bar.slug}` placeholders with `${slug}` / `${meta.foo}` / `${refs.bar.slug}`;
- keep the resulting entity paths and command behavior equivalent wherever the old and new schemas express the same intent.

### 6.2. Golden and Fixture Updates

Update affected black-box artifacts as needed:

- `response.json`
- `response.txt`
- `workspace.out/**`

The migration must remain black-box:

- update only observable command behavior;
- keep existing `response.txt` files unchanged whenever the public text output can stay identical after fixture migration;
- change `response.txt` only where the old expected text depended on legacy schema syntax and the new standard syntax necessarily changes the visible command output;
- do not add assertions that depend on internal implementation details.

### 6.3. Case Naming

Renaming existing `add` / `update` cases is optional.

However:

- new cases added by this task must use the current terminology;
- no newly added `add` / `update` case id or description may contain `required_when`.

## 7. New Integration Coverage for `required: ${expr}`

Add dedicated `add` and `update` integration coverage for the new `required` expression model.

Minimum required addition:

- for `add`: `2` success cases and `2` failure cases;
- for `update`: `2` success cases and `2` failure cases.

This minimum is sufficient if it covers both metadata and content sections.

### 7.1. Required New Cases for `add`

Success:

1. `meta.fields.<name>.required: ${expr}` evaluates truth-like and the required field is provided, so `add` succeeds.
2. `content.sections.<name>.required: ${expr}` evaluates false-like and the section is omitted, so `add` succeeds.

Failure:

1. `meta.fields.<name>.required: ${expr}` evaluates truth-like and the field is missing, so `add` fails with validation.
2. `content.sections.<name>.required: ${expr}` evaluates truth-like and the section is missing, so `add` fails with validation.

### 7.2. Required New Cases for `update`

Success:

1. A patch produces a final state where `meta.fields.<name>.required: ${expr}` evaluates truth-like and the field is present, so `update` succeeds.
2. A patch produces a final state where `content.sections.<name>.required: ${expr}` evaluates false-like and the section is absent, so `update` succeeds.

Failure:

1. A patch produces a final state where `meta.fields.<name>.required: ${expr}` evaluates truth-like and the field is absent, so `update` fails with validation.
2. A patch produces a final state where `content.sections.<name>.required: ${expr}` evaluates truth-like and the section is absent, so `update` fails with validation.

### 7.3. Coverage Expectations

These new tests must verify the current expression model itself, not merely parser acceptance.

At minimum, the new cases must demonstrate:

- runtime evaluation of `${expr}`;
- truthy / falsy behavior, not only boolean literals;
- use of current `required` rather than legacy `required_when`;
- final validation behavior of mutating commands after candidate assembly / patch application.

## 8. Acceptance Criteria

The task is complete only if all of the following are true.

1. `add` and `update` no longer support `required_when`.
2. `add` and `update` support `required: boolean | ${expr}` with default `required = true`.
3. `add` and `update` no longer use the legacy object DSL for `pathTemplate.cases[].when`.
4. `add` and `update` use `${expr}` interpolation in `pathTemplate.use` instead of `{...}` placeholders.
5. Existing `add` / `update` integration fixtures are migrated to the new schema syntax.
6. `add` has at least `2` new success and `2` new failure integration cases for `required: ${expr}`.
7. `update` has at least `2` new success and `2` new failure integration cases for `required: ${expr}`.
8. The migrated commands do not add compatibility warnings for legacy syntax.
9. `make vet` and `make test` pass after the migration.

## 9. Notes for Implementation Review

During review, check these points explicitly:

- there is no remaining `required_when` handling under `internal/application/commands/add/**`;
- there is no remaining `required_when` handling under `internal/application/commands/update/**`;
- there is no remaining legacy object-DSL evaluator on the `required` / `pathTemplate.when` execution path for `add` / `update`;
- there is no remaining `{...}` placeholder renderer on the `pathTemplate.use` execution path for `add` / `update`;
- the new integration cases are direct black-box coverage of the migrated behavior, not indirect coverage through unrelated scenarios.
