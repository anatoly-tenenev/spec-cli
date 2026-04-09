# GET Null Alignment Task

## Purpose

Align `spec-cli get` with the read-side null materialization model already used by `spec-cli query` for selected leaf paths.

The current public behavior is asymmetric:

- `query --select meta.<field>` materializes `null` when the selected leaf is absent;
- `get --select meta.<field>` currently omits the missing field from the projected `meta` object;
- `get --select refs.<field>` and `get --select content.sections.<name>` already materialize `null` when absent.

This task removes that asymmetry for `get`.

## Problem Statement

For machine-first consumers, the current `query`/`get` difference is undesirable:

- the same selected logical leaf path can produce `null` in `query`;
- but disappear entirely in `get`.

That makes the read contract harder to reason about and forces agents to special-case the two read commands unnecessarily.

## Current Contract

### Query

For `query`, the projection layer materializes `null` for missing selected leaf paths during projection.

In practice this means selected leaf paths under:

- `meta.<field>`
- `refs.<field>`
- `content.sections.<name>`

can appear as `null` when absent in the selected entity view.

### Get

For `get`, the current behavior is mixed:

- selected `refs.<field>` materialize as `null` when absent;
- selected `content.sections.<name>` materialize as `null` when absent;
- selected `meta.<field>` do **not** materialize as `null` when absent and remain omitted from the projected `meta` object.

## Target Contract

`get` must align with `query` for selected leaf paths.

### Required behavior

For `spec-cli get`:

- selected `meta.<field>` materializes as `null` when absent;
- selected `refs.<field>` continues to materialize as `null` when absent;
- selected `content.sections.<name>` continues to materialize as `null` when absent.

### Aggregate selector behavior must stay unchanged

This task must **not** change aggregate selector semantics.

The following aggregate selectors remain sparse objects and must not synthesize unknown children:

- `meta`
- `refs`
- `content.sections`

Examples:

- `--select meta` should still return only actually present meta children;
- `--select refs` should still return only requested/known ref children according to current read-view construction;
- `--select content.sections` should still return only existing sections.

The rule is:

- selected leaf paths materialize `null` when absent;
- selected aggregate objects remain sparse.

## Scope

This task applies to:

- `spec-cli get`
- its help text
- its integration coverage

This task does not change:

- `query` null materialization semantics
- `where` behavior
- `sort` behavior
- write commands
- schema capability layout

## Implementation Direction

Update `get` projection logic so that selected `meta.<field>` behaves like the already-supported `refs.<field>` and `content.sections.<name>` null materialization flow.

The likely touch points are:

- selector planning for `get`
- `nullIfMissing` / equivalent missing-path materialization logic in get projection
- integration cases for `get --select meta.<field>`
- help text for `get`

The implementation should preserve deterministic projection order and keep the current aggregate-selector behavior unchanged.

## Help Contract Changes

After this change, `help` for `get` should explicitly describe the aligned rule:

- selected leaf paths under `meta.<field>`, `refs.<field>`, and `content.sections.<name>` materialize as `null` when absent;
- aggregate selectors `meta`, `refs`, and `content.sections` keep sparse objects and do not synthesize unknown children.

`query` and `get` help should remain aligned on this point.

## Test Expectations

Add or update direct integration coverage for:

1. `get --select meta.<field>` where the selected field is schema-known but absent in entity data
   Expected: the selected leaf path materializes as `null`

2. Existing `get --select refs.<field>` missing-ref behavior
   Expected: remains `null`

3. Existing `get --select content.sections.<name>` missing-section behavior
   Expected: remains `null`

4. `get --select meta`
   Expected: aggregate selector remains sparse and does not synthesize missing children as `null`

If an existing test currently expects omission for `get --select meta.<field>`, update it to the new contract.

## Acceptance Criteria

The task is complete when all of the following are true:

1. `get --select meta.<field>` returns `null` for absent selected schema-known leaf fields.
2. `get --select refs.<field>` still returns `null` when absent.
3. `get --select content.sections.<name>` still returns `null` when absent.
4. Aggregate selectors `meta`, `refs`, and `content.sections` remain sparse.
5. `query` and `get` help texts describe the same selected-leaf-vs-aggregate rule.
6. Integration tests directly cover the aligned contract.
7. `make vet` and `make test` pass after the change.

## Non-Goals

This task does not attempt to:

- redesign the whole read projection contract;
- change `query` projection semantics;
- materialize `null` for all absent descendants under aggregate selectors;
- change public error codes or result-state contracts.
