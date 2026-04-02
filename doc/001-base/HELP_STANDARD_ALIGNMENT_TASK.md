# `help` Alignment Task: Current Standard Schema Projection

## 1. Purpose

This document defines a focused implementation task for bringing `spec-cli help` and `spec-cli help <command>` into alignment with the current local standard for schema-derived help projection.

The immediate driver is the remaining legacy representation of conditional requiredness in help output:

- current help projection still accepts legacy `required_when`;
- current help projection emits canonical `required.when`;
- the current standard no longer uses either form for field or section requiredness.

Backward compatibility for `required_when` is explicitly not required.
This task treats `required_when` as legacy syntax that must be removed from the help path completely.

The normative target is the current local standard in `spec/SPEC_STANDARD_RU_REVISED_V3.md`, especially:

- section `11.5` for the unified `required` model;
- section `11.6` for `${expr}` and JMESPath truthiness;
- the examples where conditional requiredness is expressed as `required: ${expr}`.

## 2. Scope

In scope:

- `internal/application/help/helpschema/**`
- `internal/application/help/helptext/**` if rendering changes are required
- `tests/integration/cases/help/**`
- `tests/integration/help_cases_test.go`
- help-related documentation that describes the projected schema contract

Out of scope for this task:

- changing the mutating command semantics of `add` / `update`;
- changing `validate` semantics, except using `validate` as the semantic reference for `required: ${expr}`;
- redesigning the general text layout of help output outside the schema-projection contract;
- adding backward-compatibility shims for legacy help projection syntax.

## 3. Current Divergence

`help` currently diverges from the standard in both accepted schema syntax and emitted schema projection.

### 3.1. Legacy `required_when` Is Still Accepted by the Help Projector

The current standard no longer defines `required_when`.

Conditional requiredness must be represented only as:

```yaml
required: true | false | ${expr}
```

Current `help` projection still accepts:

```yaml
required_when:
  eq?: [meta.status, active]
```

This means the help path still understands obsolete schema syntax that other updated parts of the system already reject.

### 3.2. Help Emits Obsolete Canonical Form `required.when`

The current help projection rewrites conditional requiredness into:

```yaml
required:
  when:
    eq?: [meta.status, active]
```

This is no longer the standard contract.

The current standard requires help to show the same expression model users must actually write in schema files:

```yaml
required: ${meta.status == 'active'}
```

### 3.3. Help Does Not Support Standard `required: ${expr}`

The current projector treats `required` as boolean-only unless a separate legacy `required_when` key is present.

As a result, a schema that correctly uses:

```yaml
required: ${meta.status == 'active'}
```

is not currently part of the supported help projection contract and may degrade into `SCHEMA_PROJECTION_ERROR` instead of rendering a loaded schema section.

### 3.4. Help Fixtures and Snapshots Lock In the Legacy Contract

The current help integration fixtures and golden outputs still rely on:

- schemas using `required_when`;
- snapshots expecting `required.when` in the rendered `Schema` section.

This means the help test suite currently protects obsolete behavior instead of the current standard.

### 3.5. Help Documentation Is Internally Inconsistent

Current documentation for `internal/application/help/helpschema` still describes:

- normalization from `required|required_when -> required`;
- emission of canonical `required.when`.

This is inconsistent with the current standard and with the documented `add` / `update` schema model, which already states `required: boolean | "${expr}"` and rejection of legacy `required_when`.

## 4. Migration Rules

Backward compatibility is not required.
`required_when` must be fully removed from the help path.

After this task:

- `help` must not support `required_when` in any form;
- `help` must not emit canonical `required.when`;
- `help` must project conditional requiredness only as `required: ${expr}`;
- parsing, normalization, and rendering logic specific to `required_when` must be deleted from the help implementation;
- help fixtures, snapshots, and help-specific documentation must not rely on or describe `required_when`;
- help output must remain deterministic and text-first;
- degraded-mode behavior for schema problems must remain intact.

The task must not add warnings, compatibility notices, migration shims, or dual-format output.

## 5. Required Implementation Changes

### 5.1. Align Help Projection With the Standard `required` Model

For help schema projection:

- parse `required` as `boolean | ${expr}`;
- apply default `required = true` when the key is absent;
- delete `required_when` parsing and normalization branches from the help projector;
- preserve plain boolean output for unconditional requiredness;
- preserve scalar expression output for conditional requiredness.

Observable target:

```yaml
required: false
required: true
required: ${meta.status == 'active'}
```

and not:

```yaml
required:
  when:
    eq?: [meta.status, active]
```

### 5.2. Keep `meta.fields -> meta|refs` Projection Intact

This task must not regress the existing help-specific split where:

- non-`entityRef` metadata stays under `meta`;
- `entityRef` metadata is projected under `refs`.

Only the requiredness representation must change.

### 5.3. Keep Text Rendering Stable Outside the Requiredness Change

The `Schema` section of text help must remain stable except where the standard-alignment change necessarily affects the visible output.

This means:

- preserve section order;
- preserve deterministic key ordering;
- preserve `ResolvedPath` / `Status` behavior;
- preserve degraded-mode recovery blocks;
- avoid unrelated wording or formatting churn.

### 5.4. Keep Degraded Help Semantics Unchanged

The task must not weaken the contract that:

- `help` and `help <command>` still succeed on schema failures;
- schema projection failures surface as degraded help with `Status != loaded`;
- `ReasonCode`, `Impact`, `RecoveryClass`, and `RetryCommand` remain stable.

Only the classification of valid standard syntax versus invalid legacy syntax may change.

### 5.5. Use `validate` as the Semantic Reference

For the `required` expression model, `help` should align with the same standard contract already documented and implemented for `validate`:

- `required` accepts `boolean | ${expr}`;
- missing `required` defaults to `true`;
- expression form follows JMESPath syntax and truthiness semantics.

`help` does not need to execute instance-time validation logic, but it must accept and project the same schema-level syntax that the standard defines.

## 6. Test Migration for Help

The current help integration suite must be migrated away from legacy syntax.

### 6.1. Rewrite Existing Help Fixtures

Across `tests/integration/cases/help/**` and the shared `helpSchemaFixture`:

- replace `required_when` with `required: ${expr}` where conditional requiredness is intended;
- keep the projected schema meaning equivalent;
- update snapshots to the new visible help output.

### 6.2. Update Golden Text Outputs

Update affected `response.txt` files so that the rendered `Schema` section shows standard requiredness syntax.

The expected visible form is a scalar expression:

```yaml
required: ${meta.status == 'active'}
```

not the legacy nested object form.

### 6.3. Add Negative Coverage for Legacy Syntax

The help suite must explicitly verify that legacy `required_when` is rejected after its removal.

Minimum required addition:

- at least one help case where schema uses `required_when` and help does not render `Status: loaded`.
- the case may assert the existing degraded-schema contract if that is how the help path classifies the failure after removal.

This is necessary so the test suite stops silently preserving the old syntax.

### 6.4. Add Positive Coverage for Standard Expression Form

The help suite must explicitly verify loaded-schema rendering for:

- `required: false`
- `required: true`
- `required: ${expr}`

at minimum across:

- one `meta` field;
- one projected `refs` field;
- one `content.sections` entry.

## 7. Documentation Updates Required by the Implementation

When the implementation is done, update the relevant documentation in the same change:

- `doc/README.md` if any new document is added;
- `doc/CODEBASE_INDEX.md` for the `internal/application/help/helpschema` entry so it no longer describes legacy `required_when` / `required.when` behavior.

The acceptance state of the repository documentation after implementation must be:

- `help` docs describe only `required: boolean | "${expr}"`;
- no help-specific documentation claims canonical `required.when`;
- no help-specific documentation claims support for `required_when`.

## 8. Acceptance Criteria

The task is complete only if all of the following are true.

1. `help` no longer supports `required_when`.
2. `help` supports `required: boolean | ${expr}` with default `required = true`.
3. `help` renders conditional requiredness as scalar `${expr}`, not as `required.when`.
4. Existing help integration schemas are migrated to the standard syntax.
5. Existing help snapshots are updated to the standard rendered form.
6. The help suite contains explicit positive coverage for `required: ${expr}`.
7. The help suite contains explicit negative coverage for legacy `required_when`.
8. Degraded help behavior for unrelated schema failures remains unchanged.
9. `doc/CODEBASE_INDEX.md` is updated together with the implementation change.
10. No help implementation file, help fixture, help snapshot, or help-specific documentation depends on `required_when`.
11. `make vet` and `make test` pass after the migration.

## 9. Review Notes

Reviewers should explicitly verify the following.

- A valid schema using `required: ${expr}` keeps `help` in `Status: loaded`.
- The rendered `Schema` block shows the expression unchanged as a scalar.
- Legacy `required_when` no longer appears in help implementation code, help fixtures, help snapshots, or help-specific documentation.
- Help degraded-mode cases still behave exactly as before for missing, unreadable, malformed, and schema-invalid inputs not related to this migration.
- Documentation no longer describes the obsolete help-specific requiredness model.
