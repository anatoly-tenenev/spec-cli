# `update` Command Implementation Plan

## 1. Sources and Boundaries

This plan is derived only from:

- `SPEC_UTILITY_CLI_API_BASELINE_RU.md`
- `SPEC_STANDARD_RU_REVISED_V3.md`

Goal: implement the baseline `spec-cli update` command that partially updates an existing entity, validates the final state, and writes the result atomically.

Out of scope:

- any documents other than the two listed above;
- CLI extensions outside the baseline profile;
- orchestration/streaming scenarios;
- exposing filesystem paths in the public command contract.

## 2. What `update` Must Support

### 2.1. External Contract

The command must support:

```bash
spec-cli update [options] --id <id>
```

Baseline options:

- `--id <id>`
- `--set <path=value>`
- `--set-file <path=filepath>`
- `--unset <path>`
- `--content-file <path>`
- `--content-stdin`
- `--clear-content`
- `--expect-revision <token>`
- `--dry-run`

Mandatory rules:

1. `--id` is required.
2. At least one patch operation must be provided.
3. `--content-file`, `--content-stdin`, and `--clear-content` are mutually exclusive.
4. `--set-file` is allowed only for `content.sections.<name>`; using `--set-file` for `meta.<field>` and `refs.<field>` must fail with `WRITE_CONTRACT_VIOLATION`.
5. `--set` / `--set-file` / `--unset` for `content.sections.<name>` may not be mixed with whole-body operations.
6. For types without a `content` block, `--content-file`, `--content-stdin`, and `--clear-content` must fail with `WRITE_CONTRACT_VIOLATION`.
7. `update` must not depend on `schema.model` as the runtime source of the write contract.
8. Allowed paths for `--set` are derived directly from raw schema of the entity type:
   - `meta.<field>` only for fields from `meta.fields` where `schema.type != entityRef`;
   - `refs.<field>` only for fields from `meta.fields` where `schema.type = entityRef`;
   - `content.sections.<name>` only for sections from `content.sections`.
9. Allowed paths for `--unset` are defined by the same writable namespace as `--set`.
10. Normatively forbidden write paths are determined by baseline write-namespace rules and the raw schema of the target type.
11. Each write path must pass path/value validation before the patch is applied.
12. If `--expect-revision` does not match, nothing is written.
13. After the patch is applied, the final entity must pass full validation.
14. If the post-patch state is invalid, nothing is written.
15. If there are no actual changes, the command must return successful `no-op` with `updated: false`, `noop: true`, `changes[]: []`.
16. On an actual change, `updatedDate` is updated automatically.
17. If the canonical path changes, the internal move must be part of the same atomic operation.
18. Successful `--dry-run` must execute the same pipeline as normal `update`, return the same success payload, but not write to disk and set `dry_run: true`.

### 2.2. Semantic Behavior

`update` must perform partial updates, not "read the file and fully regenerate it from the model" while ignoring the original document structure.

That means there are two distinct modes:

1. Whole-body mode.
   `--content-file`, `--content-stdin`, `--clear-content` replace the entire body as a literal string.

2. Path-based mode.
   `--set` and `--unset` change only concrete writable slots:
   - `meta.<field>`
   - `refs.<field>`
   - `content.sections.<name>`
   `--set-file` is allowed only for `content.sections.<name>` and sets the section body from file content as a literal string.

For `content.sections.<name>`, the value always means section body without heading.

### 2.3. What Counts as a Change

Separate:

1. User changes.
   Writable fields and/or body.

2. Derived changes.
   Recomputed `updatedDate`, `revision`, possible internal file move.

`changes[]` must describe actual user changes only. Do not include derived changes.

## 3. Dependencies and Internal Components

`update` needs the following minimal internal subsystems.

### 3.1. Schema Loader + Internal Write Model

`update` must not depend on `schema.model` as the runtime source of the write contract.
The implementation must derive the write contract directly from raw schema of the target type and keep an internal model sufficient for:

- describing writable paths and their `kind` / `value_kind`;
- deriving allowed paths for `--set`;
- deriving allowed paths for `--unset` from the same writable namespace;
- deriving normatively forbidden write forms from baseline write-namespace rules and raw schema.

Practical approach:

1. Load raw schema.
2. Build an internal schema model once.
3. Use the normalized internal write model for `update`.

This removes divergence between raw schema as source of truth, schema discovery output, and runtime validation of `update`.

### 3.2. Repository / Index Layer

Workspace indexes are required:

- `id -> entity location`
- `(type, slug) -> entity`
- `path -> entity`
- entity index for `entityRef` resolution

Without these indexes, `update` cannot correctly handle:

- target lookup by `--id`;
- `entityRef` checks;
- `slug` uniqueness checks;
- path conflict checks during possible moves.

### 3.3. Markdown Entity Parser

The parser must be able to:

1. Extract YAML frontmatter.
2. Parse it under YAML 1.2.2 with duplicate-key rejection.
3. Separate built-in fields and schema-driven metadata.
4. Keep body in raw form.
5. Build a normalized section model:
   - `label`
   - `title`
   - heading span
   - section body span

Without this, section patching cannot be implemented without implicit whole-document rewrite.

### 3.4. Serializer

One deterministic serializer is required for:

- frontmatter field order;
- frontmatter delimiter format;
- newline policy;
- rebuilding the document after patching;
- computing `revision` from exact serialized bytes.

All write commands should use the same serializer. Otherwise `revision` and `--expect-revision` behavior diverge between commands.

### 3.5. Validator

Two validation stages are needed:

1. Preflight write validation.
   Validates patch arguments and their basic compatibility with the write contract.

2. Full post-patch validation.
   Validates the final entity under the standard and baseline, including:
   - built-in fields;
   - `meta.fields`;
   - `entityRef`;
   - `content.sections`;
   - `pathTemplate`;
   - `slug` uniqueness;
   - canonical path correctness;
   - path conflict.

## 4. Suggested Runtime Pipeline for `update`

### Step 1. Parse CLI Arguments

At this step:

1. Enforce required `--id`.
2. Enforce that at least one patch operation exists.
3. Validate mutually exclusive groups.
4. Gather operations into one normalized structure:
   - `set_ops[]`
   - `set_file_ops[]`
   - `unset_ops[]`
   - `body_op` (`replace_file`, `replace_stdin`, `clear`, or `none`)

Catch pure CLI syntax problems immediately:

- invalid `path=value` format;
- empty path;
- repeated path;
- the same path used in both `set` and `unset`.

This is the `INVALID_ARGS` layer, not domain write validation.

### Step 2. Load Schema and Build the Write Contract

Need to:

1. Read the effective schema.
2. Build or retrieve the internal schema model.
3. The final type-specific path validation happens only after reading the target entity, because allowed write paths depend on the actual target type.

### Step 3. Find the Target Entity by `--id`

Need to:

1. Build the workspace index.
2. Find the entity by `id`.
3. If not found, fail with `ENTITY_NOT_FOUND`.

The command may rely on the internal repository path, but that path must never appear in the public JSON response.

### Step 4. Parse the Current Document

Need an in-memory representation with:

- `type`
- `id`
- `slug`
- `createdDate`
- `updatedDate`
- metadata fields
- raw body
- normalized sections
- current internal path

Important nuance: `update` must be able to repair an invalid entity if the document can still be parsed deterministically and mapped to a type. So full validity of the current state must not be required before patching.

Fail fast only if it is impossible to:

- read the file;
- parse frontmatter;
- determine `type`;
- determine enough structure to apply the patch.

### Step 5. Compute Current `revision` and Check `--expect-revision`

Need to:

1. Use the current persisted document as the source of truth for the current `revision`, without canonical rebuild of the parsed model.
2. Apply only the serializer's standard newline policy to its content.
3. Compute `revision` from the exact bytes of that representation.
4. If `--expect-revision` is set and does not match, return `CONCURRENCY_CONFLICT` without writing.

The comparison must happen before any in-memory modification.

### Step 6. Path/Value Preflight Validation

After the entity `type` is known, load the type-specific write contract:

- `set_paths`
- `unset_paths`, matching the same writable namespace
- `path_specs`
- `forbidden_pathTemplates`, derived from baseline write-namespace rules and raw schema

For each patch element:

1. Validate that the path is allowed for this operation.
2. Validate that the path is not covered by forbidden patterns.
3. Validate that the operation matches the path kind:
   - `meta`
   - `ref`
   - `section`

Value parsing rules:

1. `--set meta.<field>=...`
   Parse RHS as YAML value-node with the same typing profile as frontmatter.
   All value forms allowed by the field's `schema.type` are accepted.
   For `schema.type: array`, YAML array is allowed.
   `type: object` is not supported by the standard and must not be accepted.

2. `--set refs.<field>=...`
   Treat RHS as literal string `id`.

3. `--set content.sections.<name>=...`
   Treat RHS as literal section body string without heading.

4. `--set-file`
   Allowed only for `content.sections.<name>`.
   For `meta.<field>` and `refs.<field>`, return `WRITE_CONTRACT_VIOLATION`.
   File content is read "as is", without trim and without escape interpretation.
   Then apply the same path-specific semantics as `--set`.

5. `--unset`
   Only validate that the path and the operation are allowed.

This step must not attempt full schema validation. It is enough to:

- validate value shape;
- ensure the value can be applied to the given path;
- reject obvious write-contract violations.

Full checks of `enum`, `const`, `required_when`, `entityRef`, `pathTemplate`, and sections must run only after the patch is applied to the full candidate.

### Step 7. Apply the Patch to the In-Memory Model

#### 7.1. Metadata Patch

For `meta.<field>`:

1. `set` replaces the field value.
2. `unset` removes the key from frontmatter.
3. If the field is absent, `unset` is a successful `no-op`.

#### 7.2. Ref Patch

For `refs.<field>`:

1. `set` replaces the string `id` in the frontmatter field corresponding to that `entityRef`.
2. `unset` is allowed for any `refs.<field>` inside the writable namespace of `update`.
3. If the field is absent, `unset` is a successful `no-op`.

In the internal model, `refs.<field>` must not be stored as a separate persisted object. The persisted form remains the raw frontmatter key with a string `id`.

#### 7.3. Section Patch

For `content.sections.<name>`:

1. `set` replaces the body of an existing section while preserving the heading.
2. `set` on an absent section creates the section.
3. `unset` removes the section entirely.
4. If the section is already absent, the operation is a successful `no-op`.

Creation of an absent section must use:

- canonical order from schema `content.sections`;
- heading `## <title> {#<label>}`;
- the first allowed schema `title` when `title` is a list;
- the `label` itself when `title` is absent.

Key implementation point: path-based section patching must preserve the rest of the body as much as possible.

Required insertion algorithm:

1. Find the nearest existing schema section before the new one in canonical order.
2. If found, insert after it.
3. Otherwise find the nearest following schema section and insert before it.
4. If no schema section exists, append to the end of the body using the serializer's spacing policy.

Unknown sections and other body content must remain untouched.

#### 7.4. Whole-Body Patch

If a one-shot body operation exists:

- `--content-file` -> replace body with file content;
- `--content-stdin` -> replace body with stdin content;
- `--clear-content` -> body becomes empty string.

In this mode, no section-level patch is allowed by CLI contract.
For types without `content`, such operations must already have been rejected during preflight as `WRITE_CONTRACT_VIOLATION`.

### Step 8. Compute Semantic No-Op

After patch application but before automatic `updatedDate` change, compare original and new user state:

- built-in writable semantic subset: `slug` is excluded because it is readonly for `update` in the write contract;
- metadata fields;
- raw body;
- persisted ref values.

If user state did not change:

1. Mark the candidate as `noop`.
2. Do not change `updatedDate`.
3. `changes[]` must be empty.

Important: do not return `noop` before full post-patch validation. Otherwise the command could "succeed" on an already-invalid entity just because the patch did not change it.

### Step 9. Automatically Update `updatedDate`

If the semantic diff is not empty:

1. Take the current calendar date from the injected clock source.
2. Write it into `updatedDate`.

Because date granularity is daily, `updatedDate` may remain the same string when the update happens on the same day. That is fine: the change is still reflected in `revision` and document content.

### Step 10. Run Full Post-Patch Candidate Validation

Run the same validation pipeline required by the standard for entity-level validation:

1. Built-ins:
   - `type`
   - `id`
   - `slug`
   - `createdDate`
   - `updatedDate`

2. `meta.fields`:
   - `required`
   - `required_when`
   - `schema.type`
   - `schema.const`
   - `schema.enum`
   - array constraints
   - `entityRef`

3. `content.sections`:
   - `required/required_when`
   - label presence
   - title validity

4. `pathTemplate`:
   - case selection
   - placeholder substitution
   - canonical path validation

5. Workspace-level invariants:
   - global `id` uniqueness
   - `slug` uniqueness inside type
   - path conflict
   - successful link resolution

If the candidate is invalid:

1. Do not write to disk.
2. Return a top-level `error`.
3. Include diagnostics in `error.details.validation.issues[]` when possible.

If the candidate is valid and was previously marked as `noop`:

1. Do not write to disk.
2. Return success with `noop: true`.
3. Keep `changes[]` empty.

### Step 11. Compute Final Path and Select Move/Write Strategy

Canonical path must be computed only for a valid post-patch candidate.

Then there are two cases.

#### Case A. Path Unchanged

Need to:

1. Serialize the document.
2. Compute final `revision`.
3. If `dry-run`, do not write.
4. In normal mode, write through temp file + atomic rename over the original file.

#### Case B. Path Changed

Need to:

1. Check target path conflict.
2. Prepare parent directories.
3. Execute the internal move as part of one write transaction.

Practical transaction plan:

1. Serialize the new document into a temp file on the same filesystem.
2. If target path is occupied by another entity, return `PATH_CONFLICT`.
3. Prepare a rollback plan.
4. Move/replace files in a way that preserves the ability to restore the original state on failure.
5. After successful commit, remove the old path if it differs from the new one.

Minimum requirement here is not "perfect lock-free multiprocess transaction", but the baseline guarantee:

- on failure, the final disk state does not change;
- the public contract does not expose internal path operations.

### Step 12. Build `changes[]`

`changes[]` must be built from the semantic diff between pre- and post-patch user state.

Rules:

1. Do not include entries without an actual change.
2. For scalar/ref paths use:
   - `field`
   - `op`
   - `before`
   - `after`

3. For `content.sections.<name>` use:
   - `field`
   - `op`
   - `before_present`
   - `after_present`
   - `before_hash`
   - `after_hash`

4. Section hash is computed from the exact section body string without heading.
5. Section hash format must match `revision` token format.
6. Whole-body operations add a synthetic entry with `field`, `op`, `before_hash`, `after_hash`, where `field = "content.raw"`.
7. `content.raw` in `changes[]` is only a response diff representation and is not a write path.

For `refs.<field>` in `changes[]`, show the scalar `id`, not the expanded read-view.

### Step 13. Build Success Response

Success response must include:

- `result_state: "valid"`
- `dry_run`
- `updated`
- `noop`
- `changes[]`
- `entity`
- `validation`

`entity` must include:

- built-ins;
- `revision`;
- `meta`;
- `refs` only in short form `{ "id": "<target_id>" }`.

If the operation was `noop`:

- `updated` must be `false`;
- `noop` must be `true`;
- `changes[]` must be empty.

## 5. Serializer and Persisted-Shape Details

### 5.1. Frontmatter

For deterministic write behavior, `update` must use the same canonical frontmatter field order already fixed for `add`:

1. `type`
2. `id`
3. `slug`
4. `createdDate`
5. `updatedDate`
6. then `meta.fields` in the declaration order from raw schema

This order is mandatory for the `update` serializer.

### 5.2. Persisted Refs

In the persisted document, a reference field remains a regular frontmatter field with a string `id`.

The object form of `refs.<field>` must not leak into storage.

### 5.3. Body Serialization

Serializer spacing/newline policy must be fixed as follows:

- final Markdown uses platform-native newline policy;
- `\n` on Unix-like systems;
- `\r\n` on Windows;
- if body is non-empty, there must be exactly one blank line between the closing frontmatter delimiter and the body;
- if body is empty, do not add an extra empty block after frontmatter;
- when inserting a new schema section, separate it from adjacent content with exactly one blank line;
- the file must always end with a trailing newline;
- unknown sections and untouched body content must not be reformatted without need.

These exact bytes participate in `revision`.

## 6. Test Set

Tests should exist not only as unit tests but also as contract/integration tests at CLI level.

### 6.1. Arguments and Write Contract

Required cases:

1. Missing `--id`.
2. No patch operations.
3. Mixed mutually exclusive whole-body options.
4. Mixed whole-body and section patch.
5. Repeated path.
6. The same path in both `set` and `unset`.
7. Path absent from `set_paths`.
8. Path absent from writable namespace for `--unset`.
9. Path matches forbidden pattern.
10. `--set-file` used outside `content.sections.<name>`.
11. `--content-file` / `--content-stdin` / `--clear-content` used for a type without `content`.

### 6.2. Patch Values

Required cases:

1. `meta.<field>` with valid YAML scalar.
2. `meta.<field>` with valid YAML array for `schema.type: array`.
3. `meta.<field>` with type mismatch.
4. `refs.<field>` with literal `id`.
5. `refs.<field>` with missing entity.
6. `content.sections.<name>` through `--set`.
7. `content.sections.<name>` through `--set-file`.
8. `--unset content.sections.<name>` for a missing section.

### 6.3. No-Op and Derived Fields

Required cases:

1. `set` to the same value.
2. `unset` of a missing optional metadata field.
3. `unset` of a missing `refs.<field>`.
4. `unset` of a missing section.
5. Verify no disk write on `noop`.
6. Verify `updatedDate` does not change on `noop`.
7. Verify successful `noop` returns `updated: false`, `noop: true`, `changes[]: []`.
8. Verify `noop` does not bypass post-patch validation on an already-invalid entity.

### 6.4. Post-Patch Validation

Required cases:

1. Patch removes required field.
2. Patch breaks `required_when`.
3. Patch breaks `entityRef`.
4. Patch breaks `content.sections`.
5. Patch creates `slug` conflict.
6. Patch changes path via `pathTemplate`.
7. Patch creates path conflict.

### 6.5. Whole-Body Mode

Required cases:

1. `--content-file` replaces the whole body.
2. `--content-stdin` replaces the whole body.
3. `--clear-content` clears the body.
4. Whole-body patch makes the entity invalid by section rules.
5. Whole-body operation for a type without `content` fails with `WRITE_CONTRACT_VIOLATION`.

### 6.6. Concurrency and Revision

Required cases:

1. Correct `--expect-revision`.
2. Incorrect `--expect-revision`.
3. `revision` changes when body changes.
4. `revision` changes when frontmatter changes.
5. `revision` does not change on `noop`.
6. Verify that `--expect-revision` uses the revision of the current persisted document, not a canonical rebuild of the parsed model.

### 6.7. Atomic Write and Move

Required cases:

1. Update without path change.
2. Update with path change.
3. Dry-run without write.
4. Simulated write failure and rollback verification.
5. Creation of missing parent directories during move.

## 7. Implementation Sequence

The implementation is best done in short complete slices, not as one large task.

### Stage 1. Infrastructure

1. Bring the internal schema model to the level needed by `update`.
2. Bring the repository/index layer to the required level.
3. Lock serializer and revision helper.

### Stage 2. Parse + Preflight

1. Add CLI argument parsing for `update`.
2. Add patch-operation normalization.
3. Add path/value preflight validation.

### Stage 3. In-Memory Patch Engine

1. Implement patching for `meta`.
2. Implement patching for `refs`.
3. Implement patching for section-level content.
4. Implement whole-body replace.
5. Add semantic diff and `changes[]`.

### Stage 4. Validation + Persistence

1. Enable full post-patch validation.
2. Enable automatic `updatedDate`.
3. Enable path recompute.
4. Enable atomic write / move transaction.
5. Enable `dry-run`.

### Stage 5. Contract Tests

1. Add unit tests for parser/patch/serializer.
2. Add CLI integration tests.
3. Add regression tests for `revision`, `noop`, `CONCURRENCY_CONFLICT`, and path move.

## 8. Fixed Clarifications

The following decisions are already part of this plan and require no further choice before coding.

### 8.1. `changes[]` for Whole-Body Operations

Whole-body replace adds a synthetic entry with `field: "content.raw"` and `before_hash` / `after_hash` computed from raw body.
Those hash fields must use the same token format as `revision`.
`content.raw` in this context is only a response diff representation, not a write path.

### 8.2. `--content-*` for Types Without `content`

For types without a `content` block, `--content-file`, `--content-stdin`, and `--clear-content` must fail with `WRITE_CONTRACT_VIOLATION`.

### 8.3. Successful `dry-run`

`update --dry-run` must go through the same pipeline as regular `update`, return the same success payload, and differ only by not writing to disk and returning `dry_run: true`.

### 8.4. `unset` on Missing Metadata/Ref Paths

`unset` must be idempotent for all allowed paths.
If `meta.<field>` or `refs.<field>` is missing, the operation is a successful `no-op` with no `changes[]` entry.

### 8.5. Current `revision` for `--expect-revision`

The current `revision` before checking `--expect-revision` must be computed from the current persisted document as the source of truth, without canonical rebuild of the parsed model.

### 8.6. Allowed Forms for `meta.<field>`

`meta.<field>` must accept any YAML value-node allowed by the target field's `schema.type`.
For `schema.type: array`, YAML array is allowed.
`type: object` is not supported by the standard and must not be accepted by `update`.

### 8.7. Scope of `--set-file`

`--set-file` in `update` is allowed only for `content.sections.<name>`.
Using `--set-file` for `meta.<field>` and `refs.<field>` must fail with `WRITE_CONTRACT_VIOLATION`.

### 8.8. Frontmatter Order

The `update` serializer must use the mandatory frontmatter field order:
`type`, `id`, `slug`, `createdDate`, `updatedDate`, then `meta.fields` in the raw-schema declaration order.

### 8.9. Serializer Spacing/Newline Policy

The `update` serializer must use a minimal-normalizing policy:
platform-native newline policy, exactly one blank line between frontmatter and non-empty body, no extra empty block for empty body, trailing newline, and preservation of untouched body without unnecessary reformatting.

### 8.10. Success Payload for `noop`

Successful `noop` must return `updated: false`, `noop: true`, `changes[]: []`.

### 8.11. Source of the `update` Write Contract

`update` must derive the write contract directly from raw schema of the entity type and must not depend on `schema.model` as the runtime source of write semantics.
Allowed paths for `--unset` match the same writable namespace as `--set`.

### 8.12. Source of Truth for This Repository

For `update` implementation in this repository, the source of truth is this plan.
If another document conflicts with it, this plan takes priority for `update`.

### 8.13. Shape of `validation` in Success Response

Successful `update` response must return:

```json
"validation": {
  "ok": true,
  "issues": []
}
```

Validation error details are returned only inside the error envelope (`error.details.validation.issues[]`) on `VALIDATION_FAILED`.

### 8.14. Success Payload Composition

The `update` success payload must not include `target` or `file`.
The required success payload is limited to the fields defined in section 4, step 13.

### 8.15. Optimistic Concurrency Error

For `--expect-revision` mismatch, introduce domain code `CONCURRENCY_CONFLICT`.
Use `exit_code: 1` and `result_state: "invalid"`.

### 8.16. Compatibility of Whole-Body Operations with Patch Operations

Whole-body operations (`--content-file`, `--content-stdin`, `--clear-content`) may be combined with patch operations on `meta.*` and `refs.*`.
Only mixing whole-body operations with `content.sections.*` patch operations is forbidden.

### 8.17. Hash Rule for `changes[]`

For `before_hash` / `after_hash` in `changes[]`, use token format `sha256:<hex>`.
Compute the hash from the raw string value of the patch slot "as is" (without platform newline normalization).

### 8.18. Handling `--expect-revision` by Error Type

If the token in `--expect-revision` is syntactically invalid, the command fails with `INVALID_ARGS`.
If the token is syntactically valid but does not match the current revision, the command fails with `CONCURRENCY_CONFLICT`.

### 8.19. Path Resolution for File-Based Options

Paths in `--set-file` and `--content-file` are resolved relative to process `cwd` (as in `add`), not relative to `--workspace`.

### 8.20. Output Format in This Task

Within the scope of this task, `update` is implemented only for `--format json`.
Support for `ndjson` and corresponding contract tests is outside this task.

### 8.21. Shape of `entity.refs` in Success Response

In success payload, `entity.refs` uses short form only:
`{ "<field>": { "id": "<target_id>" } }`.
Expanded ref data (`type`, `slug`, `dirPath`, `meta`) must not appear in the public `update` response.

## 9. Summary

Correct baseline `update` implementation reduces to one connected pipeline:

1. Normalize CLI patch.
2. Find the entity by `id`.
3. Obtain the type-specific write contract.
4. Check `--expect-revision`.
5. Run preflight write validation.
6. Apply the patch to the in-memory document model.
7. Determine `noop` or actual change.
8. Update `updatedDate` when required.
9. Fully validate the post-patch candidate.
10. Recompute path/revision.
11. Execute atomic write or atomic move+write.
12. Return machine-first success payload with `changes[]`.

If implementation follows this order, the baseline and standard requirements fit into one deterministic pipeline without hidden side effects.
