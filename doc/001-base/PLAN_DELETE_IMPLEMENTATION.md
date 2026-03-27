# `delete` Implementation Plan for the Baseline CLI API

## 1. Basis

This plan is based only on two sources:

- `SPEC_UTILITY_CLI_API_BASELINE_RU.md`
- `SPEC_STANDARD_RU_REVISED_V3.md`

Key normative anchors:

- baseline `§2`, `§5.2`, `§5.3`, `§6`, `§7.5`, `§7.8`, `§7.9`;
- standard `§5.3`, `§6.3`, `§7`, `§8`, `§11.1`, `§11.4`, `§14.2`.

## 2. What the Command Must Do

`delete` removes one entity by exact `id` and returns this successful JSON response:

- `result_state: "valid"`
- `dry_run`
- `deleted: true`
- `target.id`
- `target.revision`

The baseline directly defines only three mandatory argument rules:

- `--id` is required;
- `--expect-revision` works as an optimistic concurrency guard;
- `--dry-run` must be supported.

## 3. Fixed Product Decisions

These are not direct baseline quotes; they are the accepted decisions used to resolve `delete` ambiguities. They must be treated as part of the implementation plan.

1. If the entity is not found, `delete` must return `ENTITY_NOT_FOUND`.
Reason: `delete`, like `get`, addresses one entity by exact `id`; the baseline defines no alternate semantics.

2. `delete` must not require a fully valid workspace.
Reason: the baseline explicitly allows a tolerant mode for `get`; deletion needs the same property, otherwise damaged documents can make deletion impossible.

3. The target entity may be deleted even if it is data-invalid, as long as it can be parsed deterministically far enough to obtain `type`, `id`, filesystem path, and `revision`.
Reason: this follows from the tolerant approach used by `get` and from the practical meaning of a delete command.

4. Deletion must be blocked if it would leave broken `entityRef` links pointing to the deleted `id`.
Reason: standard `§6.3`, `§11.4`, `§12.3`, `§14.2` requires referential integrity and dataset conformance.

5. Deletion blocking uses a dedicated domain error code `DELETE_BLOCKED_BY_REFERENCES` with `result_state: "invalid"` and `error.exit_code = 1`.
Reason: the baseline defines `CONCURRENCY_CONFLICT`, but not a code for referential-integrity violations after deletion.

6. `dry-run` must go through the same pipeline as normal `delete` and return the same success payload with the only difference being `dry_run: true`.
Reason: this is how the baseline defines `dry-run` for `add` and `update`; it is the most consistent interpretation for `delete` as well.

7. `target.revision` must be computed by the same mechanism as in `get`/`update`, not by ad hoc hashing of file bytes.
Reason: baseline `§5.3` defines one shared `revision` contract.

## 4. Implementation Boundaries

Included in the first implementation:

- command CLI parsing and options;
- target lookup by `id`;
- `--expect-revision` check;
- incoming-reference checks against the entity being deleted;
- `dry-run`;
- physical file removal;
- baseline-form JSON response;
- text-first help for the command.

Not needed in the baseline implementation:

- tombstone/archive mechanics;
- soft-delete;
- exposing filesystem paths in the public response;
- cleaning up empty directories as part of the contract;
- full validation of the entire workspace as a required precondition.

## 5. Implementation Plan

### Step 1. Add the Command CLI Surface

- Register the `delete` command.
- Support `--id`, `--expect-revision`, `--dry-run`.
- Connect shared global options `--workspace`, `--schema`, `--format`, `--require-absolute-paths`.
- For missing `--id`, return `INVALID_ARGS` with `exit_code = 2`.
- For `--format json`, use the normative JSON envelope.
- For `--format text`, keep minimal human-readable output without extending the contract.

### Step 2. Build the Minimal Runtime Context

- Load and parse the schema file from `--schema`.
- Use raw schema as the runtime rule source rather than `schema.model`.
- Prepare the list of entity types, their `idPrefix`, and reference slots from the schema:
  - scalar `entityRef`;
  - `array` where `items.type = entityRef`.
- If the schema is missing, unreadable, or unparseable such that types and references cannot be resolved reliably, return a top-level schema error with `exit_code = 4`.

### Step 3. Build a Tolerant Workspace Entity Index

- Scan the workspace for entity implementations.
- For each candidate, try to read frontmatter and extract the minimum:
  - filesystem path for internal work;
  - `type`;
  - `id`;
  - `slug`, if available;
  - the raw frontmatter field values needed for reverse-ref checks;
  - `revision`.
- Determine entity type only from the `type` field in frontmatter, as required by standard `§5.3`.
- Treat `id` as globally unique across the dataset.
- Unparseable unrelated documents must not block `delete`; ignore them if they are not the target.
- The target entity may be deleted even if it is data-invalid, as long as the document deterministically yields `type`, `id`, and `revision`.

### Step 4. Find the Target Entity

- Perform exact match by `id`.
- If there is no match, return `ENTITY_NOT_FOUND`.
- If more than one document matches the same `id`, return top-level `AMBIGUOUS_ENTITY_ID` with `result_state: "invalid"` and `error.exit_code = 1`.
- If the target document exists but cannot be read far enough to compute `revision`, return top-level `REVISION_UNAVAILABLE` with `result_state: "invalid"` and `error.exit_code = 1`.

### Step 5. Check the Concurrency Guard

- If `--expect-revision` is provided, compare it with the actual `target.revision`.
- On mismatch, return `CONCURRENCY_CONFLICT` without deleting.
- The comparison must happen before any disk changes.

### Step 6. Run the Reverse-Reference Check

- Check all parseable workspace documents for references to `target.id`.
- At minimum, inspect:
  - `meta.<field>` where the schema declares `schema.type: entityRef`;
  - arrays where the schema declares `type: array` and `items.type: entityRef`.
- Any exact raw reference to `target.id` in a schema-declared reference slot of a parseable document is blocking, even if the document as a whole is invalid.
- If incoming references are found, return top-level `DELETE_BLOCKED_BY_REFERENCES`.
- The error response must return structured details about blocking entities without exposing their filesystem paths.
- `error.details` format:
  - `blocking_refs[].source_id`
  - `blocking_refs[].source_type`
  - `blocking_refs[].field`
- Full post-delete validation of the entire workspace is not required; for the baseline it is enough to guarantee that deletion does not create new broken `entityRef` links.

### Step 7. Execute the Deletion

- In normal mode, delete only one target file.
- Deletion must be observably atomic at the command-result level: on failure, the command must not report success.
- If the filesystem delete operation fails, return a top-level read/write error with `exit_code = 3`.
- Do not make cleanup of empty parent directories part of the baseline contract.

### Step 8. Implement `dry-run`

- `dry-run` must execute steps 2-6 completely.
- At step 7, do not physically delete the file.
- Successful `dry-run` must return:
  - `result_state: "valid"`
  - `dry_run: true`
  - `deleted: true`
  - `target.id`
  - `target.revision`

### Step 9. Build the Response and Errors

- JSON success must match the baseline example from `§7.8`.
- For a missing target, use:
  - `result_state: "not_found"`
  - `error.code: "ENTITY_NOT_FOUND"`
- For a missing target, use `error.exit_code = 1`.
- For a revision mismatch:
  - top-level `error.code: "CONCURRENCY_CONFLICT"`
- For a revision mismatch, use `result_state: "invalid"` and `error.exit_code = 1`.
- For an ambiguous target:
  - `result_state: "invalid"`
  - `error.code: "AMBIGUOUS_ENTITY_ID"`
  - `error.exit_code: 1`
- For unavailable revision:
  - `result_state: "invalid"`
  - `error.code: "REVISION_UNAVAILABLE"`
  - `error.exit_code: 1`
- For blocked deletion:
  - `result_state: "invalid"`
  - `error.code: "DELETE_BLOCKED_BY_REFERENCES"`
  - `error.exit_code: 1`
- Preserve baseline fields in all errors:
  - `error.code`
  - `error.message`
  - `error.exit_code`

### Step 10. Extend Help

- In the command text help, explicitly describe:
  - that deletion is by exact `id`;
  - the role of `--expect-revision`;
  - the behavior of `--dry-run`;
  - that deletion may be blocked by incoming references.

## 6. Suggested Internal Decomposition

Minimal internal pieces:

- `DeleteCommand` or equivalent handler;
- shared `SchemaLoader`;
- shared tolerant `WorkspaceEntityIndex`;
- shared `RevisionService`;
- `ReverseReferenceChecker`;
- `DeleteExecutor` for real delete and dry-run;
- shared `JsonErrorBuilder`.

If `get` and `update` already exist in the codebase, `delete` should reuse first:

- target lookup by `id`;
- `revision` computation;
- the shared top-level error shape;
- shared handling of global CLI options.

## 7. Test Plan

### 7.1. Minimal Fixture Contract

`delete` needs not one workspace but a set of fixture bundles, because some scenarios require intentionally broken states:

- `fixtures/delete/base`
- `fixtures/delete/duplicate-id`
- `fixtures/delete/repairable-invalid-target`
- `fixtures/delete/blocking-invalid-source`
- `fixtures/delete/revision-unavailable`
- `fixtures/delete/broken-schema`
- `fixtures/delete/broken-workspace`
- `fixtures/delete/io-failure` or an equivalent runtime harness

Include in the test set only fixtures that are used by at least one case or by the shared harness.

When describing and implementing each individual test case, remove from its fixture scenario all documents, helper files, and fixture bundles that the case does not use. Otherwise the test plan starts masking extra dependencies and makes failure localization harder.

The minimal schema in `fixtures/delete/base` must contain at least three types:

- `service` without required outgoing links, usable as a safe deletion target;
- `feature` with a scalar `entityRef` to `service`;
- `release` or an equivalent type with an array of `entityRef` links to `feature`.

Additionally, the base workspace needs at least one document where `target.id` appears in a regular string field or in the body, but not in a schema-declared reference slot. This is a separate negative guard against a false-positive reverse-ref implementation based on text grep.

For scenarios with `--expect-revision`, the harness must precompute revision tokens from the exact persisted bytes of fixture documents and use them as constants, for example:

- `REV_SVC_1`
- `REV_FEAT_1`
- `REV_FEAT_2`
- `REV_INVALID_FEAT_9`

Filesystem-error cases need a separate scenario where target lookup, `revision` computation, `--expect-revision` checking, and reverse-ref analysis have already succeeded, but the filesystem delete operation of the target file itself fails. The technique used to inject the failure is not normative; the observable behavior is: the command must not return success and must not leave a partially changed state.

### 7.2. Shared Validation Rules

In every successful JSON case, additionally check:

- exit code `0`;
- `result_state = "valid"`;
- presence of `dry_run`, `deleted`, `target.id`, `target.revision`;
- `deleted = true`;
- no `error`;
- no filesystem path in the success payload;
- `target.revision` matches the revision computed from the same persisted bytes as in `get`/`update`.

In all successful cases without `--dry-run`, additionally check:

- exactly one expected file is physically removed;
- the other workspace documents are unchanged;
- for fixtures that are intended to stay fully valid after deletion, a follow-up `spec-cli validate --workspace <tmp> --schema <schema> --format json` succeeds;
- for tolerant fixtures with pre-existing unrelated invalidity, do not assert full validate success, only that `delete` did not create new broken `entityRef` links and did not worsen referential integrity relative to the initial state.

In all successful cases with `--dry-run`, additionally check:

- `dry_run = true`;
- the filesystem does not change;
- rerunning the same command without `--dry-run` on a clean fixture copy yields the same success payload except for the `dry_run` difference.

In all error cases, additionally check:

- the target file is not deleted;
- `error.code`, `error.message`, `error.exit_code` are present;
- filesystem paths do not leak into the public response;
- `ENTITY_NOT_FOUND` uses `result_state = "not_found"`;
- all other domain errors use `result_state = "invalid"`.

For `DELETE_BLOCKED_BY_REFERENCES`, additionally check:

- `error.code = "DELETE_BLOCKED_BY_REFERENCES"`;
- `error.details.blocking_refs[]` exists and is non-empty;
- every `blocking_refs[]` item contains only `source_id`, `source_type`, `field`;
- the list does not expose blocking document filesystem paths.

### 7.3. Mandatory Black-Box Cases

#### Happy Path

- `DLT-OK-01`: successful deletion of an existing entity with no incoming references and without `--expect-revision`.
- `DLT-OK-02`: successful deletion of an existing entity with correct `--expect-revision`; explicitly verify that the guard does not break the happy path.
- `DLT-OK-03`: successful `dry-run` for a deletable entity with no incoming references; the payload must match normal success except for `dry_run: true`.
- `DLT-OK-04`: deletion of a parseable but data-invalid target entity must be possible if its `revision` is computable and it has no incoming references.

#### Arguments and Lookup

- `DLT-ARG-01`: missing `--id` returns `INVALID_ARGS` and `exit_code = 2`.
- `DLT-LOOKUP-01`: exact `id` not found and the command returns `ENTITY_NOT_FOUND` with `result_state: "not_found"`.
- `DLT-LOOKUP-02`: the dataset contains more than one document with the same `id`, and the command returns `AMBIGUOUS_ENTITY_ID`.
- `DLT-LOOKUP-03`: target found but `revision` cannot be computed reliably, and the command returns `REVISION_UNAVAILABLE`.

#### Concurrency

- `DLT-CONC-01`: `--expect-revision` does not match the actual `target.revision`, and the command returns `CONCURRENCY_CONFLICT` without deletion.
- `DLT-CONC-02`: successful `dry-run` with correct `--expect-revision` and a successful non-`dry-run` run on two identical clean copies of one fixture return the same `target.revision` in the success payload.

#### Reverse References

- `DLT-REF-01`: deletion is blocked if a scalar `entityRef` points to the target.
- `DLT-REF-02`: deletion is blocked if an `entityRef` array element points to the target.
- `DLT-REF-03`: a parseable but overall invalid document with a raw reference to `target.id` in a schema-declared reference slot also blocks deletion.
- `DLT-REF-04`: a plain textual occurrence of `target.id` in a regular string field or body does not block deletion if that slot is not declared as `entityRef`.
- `DLT-REF-05`: on `DELETE_BLOCKED_BY_REFERENCES`, the response returns a minimal `blocking_refs[]` without paths and with correct `source_id`, `source_type`, `field`.
- `DLT-REF-06`: `dry-run` does not bypass reverse-ref checks; when blocking references exist, it must also fail with `DELETE_BLOCKED_BY_REFERENCES`.

#### Tolerant Workspace and I/O

- `DLT-TOL-01`: unparseable unrelated documents that are not the target are ignored and do not block `delete`; parseable documents do not get this relaxation and participate in reverse-ref analysis even if they are otherwise invalid.
- `DLT-SCHEMA-01`: a schema error is returned if `--schema` is missing, unreadable, or cannot reliably define reference slots.
- `DLT-IO-01`: a filesystem error that occurs only after all domain checks succeed and exactly during target-file deletion returns a top-level I/O error with `exit_code = 3`; success must not be reported.

#### Public Contract

- `DLT-RESP-01`: success JSON does not expose filesystem paths.
- `DLT-RESP-02`: error JSON for `DELETE_BLOCKED_BY_REFERENCES` and other failures also does not expose filesystem paths.
- `DLT-HELP-01`: `delete` text help explicitly describes exact `id`, the role of `--expect-revision`, `--dry-run`, and blocking on incoming references.

### 7.4. Recommended Component Tests

In addition to black-box scenarios, it is useful to lock down several narrow component tests to avoid catching regressions only through the integration layer:

- extraction of reference slots from raw schema distinguishes scalar `entityRef`, `array<entityRef>`, and regular string/array fields;
- the tolerant workspace index ignores unparseable unrelated documents but does not hide duplicate `id`;
- the reverse-ref checker looks only at schema-declared reference slots and does not count arbitrary textual matches;
- the delete executor uses the same pipeline for normal and `dry-run` modes up to the commit phase;
- error mapping consistently distinguishes `ENTITY_NOT_FOUND`, `AMBIGUOUS_ENTITY_ID`, `REVISION_UNAVAILABLE`, `CONCURRENCY_CONFLICT`, `DELETE_BLOCKED_BY_REFERENCES`, and I/O failure.

## 8. Completion Criteria

The `delete` implementation can be considered complete if all of the following are true:

- all mandatory black-box cases from section 7.3 pass;
- the command reliably finds the target by `id`;
- `--expect-revision` really protects against stale delete;
- deletion does not leave new broken `entityRef` links;
- `dry-run` repeats the normal pipeline without writing to disk;
- JSON success and JSON errors match the baseline contract;
- filesystem paths do not leak into the user-facing contract;
- at least one dedicated test proves that reverse-ref checks do not produce false positives from ordinary text outside `entityRef` slots.
