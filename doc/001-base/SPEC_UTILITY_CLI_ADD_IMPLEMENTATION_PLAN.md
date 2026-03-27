# Detailed `add` Command Implementation Plan

## 1. Purpose of This Document

This plan describes the implementation of `spec-cli add` for the baseline CLI API together with the required integration-test set.
The document relies only on:

- `SPEC_UTILITY_CLI_API_BASELINE_RU.md`
- `SPEC_STANDARD_RU_REVISED_V3.md`

Goal: bring `add` to a predictable machine-first implementation with full pre-write validation, atomic writes, and black-box integration coverage.

## 2. What Must Be Implemented

The code must cover the following mandatory contract.

- The command is invoked as `spec-cli add [options]`.
- Required options: `--type <entity_type>` and `--slug <slug>`.
- Supported options: `--set`, `--set-file`, `--content-file`, `--content-stdin`, `--dry-run`.
- `--content-file` and `--content-stdin` are mutually exclusive.
- `id`, `createdDate`, and `updatedDate` are computed automatically.
- `add` does not use `schema.model` as the source of the write contract.
- Allowed write paths for `--set` and `--set-file` are derived directly from raw schema:
  - `meta.<field>` for fields from `meta.fields`;
  - `refs.<field>` for fields from `meta.fields` where `schema.type = entityRef`;
  - `content.sections.<name>` for sections from `content.sections`.
- `--set-file` is allowed only for `content.sections.<name>`.
- No separate `required_set_paths` contract is used; required data is enforced only through full validation of the final entity.
- Normatively forbidden write forms are determined by the baseline write namespace rules and raw schema, without consulting `schema.model`.
- The same write path may not be provided more than once across `--set` and `--set-file`.
- The whole candidate entity must be validated before writing.
- On validation failure, the command must return a top-level `error`, not a success payload.
- When structured diagnostics are available, they must be returned in `error.details.validation.issues[]`.
- Writing must be observably atomic: on failure, on-disk state must not change.
- All top-level JSON responses must contain `result_state`.
- Entity responses must contain `revision`, computed from the actual document state.
- The implementation must respect standard rules for `idPrefix`, `pathTemplate`, `required` / `required_when`, `entityRef`, `content.sections`, `slug`, `createdDate`, `updatedDate`, closed-world schema, and diagnostic classes.

## 3. Decisions to Fix Before Coding

The baseline and the standard define the `add` contract but leave several implementation gaps. These need to be fixed early; otherwise behavior becomes non-deterministic.

### 3.1. New `id` Generation

The standard requires only `"{idPrefix}-N"` format and uniqueness of `N` within a type. The algorithm for selecting a new `N` is not standardized.

Fixed decision:

- use the deterministic rule `max(existing N for type) + 1`;
- do not reuse gaps in the sequence;
- build the existing-id index once at command start;
- do not introduce filesystem/workspace locking for baseline `add`.

Reason: `max + 1` is simpler, more stable, and easier to test than "minimum free number".

### 3.2. Date Source for `createdDate` and `updatedDate`

The standard requires `YYYY-MM-DD` format but does not fix the source of "today's" date.

Fixed decision:

- introduce an injectable `Clock`;
- in production, use the UTC day from one explicit process time source;
- in tests, stub `Clock` so the date is deterministic;
- use the same value for both `createdDate` and `updatedDate` on creation.

### 3.3. Parsing `--set <path=value>`

The baseline defines `value_kind: "typed_scalar"` / `"target_entity_id"` / `"string"`, but does not standardize how a CLI string becomes a typed value.

Fixed decision:

- for `value_kind: "target_entity_id"`, treat the RHS as a plain string;
- for `value_kind: "string"`, treat the RHS as-is;
- for `value_kind: "typed_scalar"`, parse the RHS as a standalone YAML 1.2.2 value-node using the same typing profile as YAML frontmatter;
- string values that look like `boolean` / `integer` / `null` must be passed by the user in YAML-compatible quoted form;
- lock this rule in user help and tests.

Reason: this aligns best with the standard, which defines data typing through the YAML profile of the implementation.

### 3.4. Semantics of `--content-file` and `--content-stdin`

For `--set-file`, the baseline explicitly requires reading file content "as is". For whole-body input, that is not stated separately.

Fixed decision:

- read `--content-file` and `--content-stdin` as raw strings without trim and without escape interpretation;
- do not normalize trailing newline or line endings while reading;
- add no implicit normalization besides what the final Markdown serializer must already do.

### 3.5. Creating Sections via `content.sections.<name>`

The write namespace allows writing to `content.sections.<name>`, and the path value is the section body without a heading. To create valid Markdown, the serializer must generate the heading.

Fixed decision:

- canonically serialize a section as `## <title> {#<label>}`;
- if the schema defines `title`:
  - use the first allowed `title` from the canonical schema list;
- if `title` is absent:
  - use the `label` as heading text;
- insert the section body exactly as received from the CLI, without an extra heading;
- use the order from `schema.entity.<type>.content.sections` when creating/rebuilding sections.

This does not conflict with the standard: the validator must recognize `{#label}`, and `title` comparison happens only when `title` is defined in the schema.

### 3.6. Mixing Whole-Body and Section Writes in `add`

For `update`, the baseline explicitly forbids mixing `--content-file` / `--content-stdin` / `--clear-content` with `content.sections.<name>`. For `add`, the ban is not stated explicitly.

Fixed decision:

- also forbid mixing whole-body input and `content.sections.<name>` for `add`;
- treat as mutually exclusive:
  - `--content-file` and any `--set` / `--set-file` for `content.sections.<name>`;
  - `--content-stdin` and any `--set` / `--set-file` for `content.sections.<name>`;
- detect this conflict at CLI/request validation before candidate construction;
- return `INVALID_ARGS` and do not enter full entity validation.

This keeps `add` symmetric with `update` and removes ambiguity when whole-body and section-level input are provided together.

### 3.7. `--dry-run` Behavior

The baseline shows a `dry_run` field but does not specify exactly how `created` should behave.

Fixed decision:

- `--dry-run` writes nothing to disk;
- return `dry_run: true`;
- return `created: true` if the candidate passed all checks and would have been written without `--dry-run`;
- return the same success payload as regular `add`, i.e. `created`, `entity`, and `validation`;
- the only semantic difference of successful `dry-run` is `dry_run: true` and the absence of the actual write.

### 3.8. Source of the `add` Write Contract

Fixed decision:

- `add` does not read `schema.model` and does not accept a derived write contract from elsewhere;
- the only source of truth for the `add` write contract is the raw standard schema;
- allowed write paths are derived locally from raw schema:
  - `meta.<field>` for every field in `meta.fields`;
  - `refs.<field>` only for fields in `meta.fields` where `schema.type = entityRef`;
  - `content.sections.<name>` for every section in `content.sections`;
- built-in paths, aggregate paths, and expanded read paths are rejected by baseline rules:
  - `type`, `id`, `slug`, `createdDate`, `updatedDate`;
  - `content`, `content.raw`, `content.sections`;
  - `refs.*.id`, `refs.*.type`, `refs.*.slug`;
- `--set-file` is not treated as a general "string input" mechanism;
- `--set-file` is allowed only for `content.sections.<name>`;
- `--set-file` for `meta.<field>` and `refs.<field>` always fails with `WRITE_CONTRACT_VIOLATION`, even if the final value is logically a string;
- do not introduce a separate "user must provide path X" contract;
- if required data is missing after candidate assembly, catch it only in full validation and report it as validation failure, not `INVALID_ARGS`.

### 3.9. Classification of Request-Level `add` Errors

Fixed decision:

- syntactic and structural request errors return `INVALID_ARGS`;
- `INVALID_ARGS` includes:
  - missing required CLI arguments;
  - conflicting flags;
  - duplicate path;
  - invalid `path=value` syntax;
  - violation of `--require-absolute-paths`;
- write-contract violations return `WRITE_CONTRACT_VIOLATION`;
- write-contract violations include:
  - a path outside the locally derived write contract;
  - writes to a baseline-forbidden path;
  - use of `--set-file` for a path where the operation is not allowed;
- use `WRITE_CONTRACT_VIOLATION` consistently for all such cases.

### 3.10. Canonical Path Collision

Fixed decision:

- if the computed canonical path is already occupied by another document, the command must fail with `PATH_CONFLICT`;
- `PATH_CONFLICT` is neither `INVALID_ARGS` nor `WRITE_CONTRACT_VIOLATION` nor validation failure;
- path collision is treated as workspace-state conflict;
- on `PATH_CONFLICT`, no write happens, the existing file is unchanged, and no temporary artifacts remain.

### 3.11. `exit_code` for Domain-Level `add` Failures

Fixed decision:

- `WRITE_CONTRACT_VIOLATION` uses `error.exit_code = 1`;
- `PATH_CONFLICT` uses `error.exit_code = 1`;
- final-entity validation failure uses `error.exit_code = 1`;
- distinguish these cases by `error.code`, not numeric `exit_code`.

### 3.12. Shape of `refs` in Successful `add` Response

Fixed decision:

- in successful `entity` payload, `refs.<field>` is returned in short form;
- minimal and sufficient shape:
  - `id`
- do not return `type` and `slug` in `refs.<field>` for `add`;
- the `add` response contract for `refs` is `id`-only.

### 3.13. `revision` Algorithm

Fixed decision:

- compute `revision` from the exact bytes of the final serialized Markdown document;
- both YAML frontmatter and body are part of the hash, together with the serializer's newline/formatting policy;
- identical byte output must yield identical `revision`;
- any byte change in the final document must change `revision`;
- `dry-run` must compute the same `revision` that a real write would produce.

### 3.14. Newline Policy of Final Markdown

Fixed decision:

- serialize final Markdown using platform-native newline policy;
- use `\n` on Unix-like systems;
- use `\r\n` on Windows;
- compute `revision` after applying the platform-native newline policy;
- on the same platform, the same logical document must produce the same bytes and the same `revision`;
- cross-platform identity of `revision` is not guaranteed.

### 3.15. Body for Types Without `content`

Fixed decision:

- if a type has no `content` block in raw schema, any whole-body input is forbidden;
- `--content-file` and `--content-stdin` for that type must fail with `WRITE_CONTRACT_VIOLATION`;
- `content.sections.<name>` is also forbidden for such a type and fails with `WRITE_CONTRACT_VIOLATION`;
- such types are treated as frontmatter-only from the perspective of `add`.

### 3.16. Behavior Under Parallel `add`

Fixed decision:

- the baseline `add` implementation does not use filesystem/workspace locks;
- the command works by `snapshot -> build candidate -> final checks -> atomic write`;
- when parallel `add` operations race, the winner is not standardized;
- the loser must fail with one of the already defined domain conflicts if its candidate can no longer be written correctly;
- lack of locking must not violate the requirement of observable atomicity for each individual write.

### 3.17. Canonical Frontmatter Field Order

Fixed decision:

- YAML frontmatter is serialized in canonical field order;
- built-in fields always go first in fixed order:
  - `type`
  - `id`
  - `slug`
  - `createdDate`
  - `updatedDate`
- then go fields derived from `meta.fields`, in the order declared in raw schema;
- the serializer must not depend on arbitrary map/dict order;
- on the same platform, the same logical entity must produce the same field order and byte output.

### 3.18. Serialization of Whole-Body vs Section-Level Input

Fixed decision:

- `--content-file` and `--content-stdin` are treated as sources of ready-made body content;
- when body comes from whole-body input, `add` must not rebuild it by sections, reorder sections, or normalize headings;
- whole-body input is inserted into the final document together with canonical frontmatter and then passes full validation;
- when body is provided via `content.sections.<name>`, the final body is always built canonically from the schema;
- therefore `add` has two body-building modes:
  - whole-body: preserve user structure;
  - section-level: build canonical structure.

### 3.19. Creating Parent Directories

Fixed decision:

- if the canonical entity path points into non-existing parent directories, `add` must create them automatically;
- parent-directory creation is part of the same write operation;
- on write failure, no partially-created result may remain that breaks observable final-state atomicity;
- `dry-run` does not create directories on disk.

### 3.20. `slug` Conflict

Fixed decision:

- do not check `slug` conflict with another entity of the same type via a separate precheck rule;
- check uniqueness of `slug` within a type only during full candidate validation;
- report `slug` conflict as final-entity validation failure, not as `INVALID_ARGS` and not as a separate domain code.

## 4. Suggested Internal Architecture

The implementation is best built from five layers.

### 4.1. CLI Layer

Responsible for:

- argument parsing;
- validation of mutually exclusive options;
- validation of `--require-absolute-paths` for filesystem path arguments;
- normalization of repeatable `--set` / `--set-file`;
- generation of machine-readable `INVALID_ARGS` errors for syntactic and structural problems.

### 4.2. Raw Schema Extraction Layer

Responsible for extracting everything needed specifically by `add` from the schema:

- entity type description;
- `idPrefix`;
- `pathTemplate`;
- `meta.fields`;
- `content.sections`;
- locally derived allowlist of write paths;
- locally derived allowlist of paths allowed for `--set-file`, limited to `content.sections.<name>`.

The internal result of this step must be built directly from raw schema and not depend on `schema.model`.

### 4.3. Workspace Snapshot Layer

Needed for:

- finding existing entities;
- checking `id` uniqueness;
- checking `slug` uniqueness inside a type;
- resolving `entityRef`;
- computing `refs.*` for `pathTemplate` and validation;
- selecting the next `N` for `id`;
- early detection of final-path collision.

Minimal indexes:

- `entities_by_id`;
- `slugs_by_type`;
- `max_suffix_by_type`;
- `resolved_refs_index` or an equivalent fast target-lookup mechanism.

### 4.4. Candidate Builder Layer

Must build the in-memory write candidate:

- frontmatter;
- normalized body representation;
- computed relative POSIX path;
- serialized Markdown;
- `revision`.

At this layer the write namespace is translated into the shape of the actual entity:

- `meta.<field>` -> top-level frontmatter field;
- `refs.<field>` -> top-level frontmatter field with string `id`;
- `content.sections.<name>` -> normalized Markdown section.

### 4.5. Validator Reuse Layer

It is critical not to write a separate "lightweight" validator for `add`.

Reuse the same validation engine that covers the standard and is used by `validate`, so the same document:

- passes/fails the same way in `add`;
- passes/fails the same way in a subsequent `validate`;
- emits diagnostics in the same shape.

## 5. Step-by-Step Implementation Plan

### Stage 1. Parse and Validate CLI Arguments

Do:

- register the `add` command and its options;
- make `--type` and `--slug` mandatory;
- make `--content-file` and `--content-stdin` mutually exclusive;
- forbid mixing `--content-file` / `--content-stdin` with `content.sections.<name>` in `--set` / `--set-file`;
- forbid `--set-file` for paths other than `content.sections.<name>`;
- gather all `--set` and `--set-file` into one list of write operations;
- detect repeated paths across `--set` and `--set-file`;
- validate filesystem path arguments against `--require-absolute-paths`;
- ensure the normative error envelope for `--format json`.

Checks at this stage:

- missing required arguments;
- unknown CLI options;
- empty/invalid `path=value`;
- conflicting options;
- mixing whole-body input with `content.sections.<name>`;
- relative path under `--require-absolute-paths`.

Expected result:

- either a normalized `AddRequest`,
- or an immediate `INVALID_ARGS` with exit code `2`.

### Stage 2. Load Raw Schema and Extract the Type Write Contract

Do:

- load the schema file;
- extract `entity.<type>`;
- validate that `--type` exists;
- prepare the local mapping of allowed write paths directly from raw schema.

At this stage compute immediately:

- the set of allowed paths for `add`;
- the set of paths allowed specifically for `--set-file`;
- the set of normatively forbidden paths under baseline rules;
- the content model of the type;
- `pathTemplate` rules;
- `meta.fields` rules.

If the type has no `content`, mark whole-body input as invalid for `add`.

If the schema cannot yield the normative command result, return a top-level schema error with exit code `4`.

### Stage 3. Take a Workspace Snapshot and Build Indexes

Do:

- read the current workspace;
- identify existing entities by the standard rules;
- build the `id` index;
- build the `slug` index per type;
- compute `max_suffix_by_type`;
- build the index used for `entityRef` resolution.

This must happen before candidate construction because otherwise it is impossible to:

- generate `id`;
- validate referential integrity;
- compute `refs.*` for `pathTemplate`;
- catch duplicates early.

### Stage 4. Validate and Type Write Operations

For each operation:

- verify that the path belongs to the write contract derived locally from raw schema;
- verify that the path does not belong to built-in paths, aggregate paths, or expanded read paths forbidden by baseline rules;
- verify that `--set-file` is used only for `content.sections.<name>`;
- verify that `--content-file` and `--content-stdin` are allowed for the type;
- parse the value according to raw schema and the path kind.

If any of these checks fail, the command must return `WRITE_CONTRACT_VIOLATION` with `error.exit_code = 1`, not `INVALID_ARGS`.

Transformation rules:

- `meta.<field>`:
  - for `typed_scalar`, parse the RHS as YAML 1.2.2 value-node with the same typing profile as YAML frontmatter;
- `refs.<field>`:
  - string `id` of the target entity;
- `content.sections.<name>`:
  - section body string without heading.

Important: this stage does not replace full validation. It only guarantees that the patch input is syntactically valid for `add`.

### Stage 5. Generate Candidate Built-In Fields

Do:

- take `type` from `--type`;
- take `slug` from `--slug`;
- validate `slug` against the standard regex;
- compute a new `id` using the rule from section 3.1;
- set `createdDate` and `updatedDate` from one `Clock` value interpreted as a UTC day;
- ensure that the generated `id` does not conflict with an existing one.

At this step it is also convenient to build the base frontmatter scaffold of the future entity.

### Stage 6. Translate Write Operations into the Entity Model

Do:

- copy `meta.<field>` to top-level YAML fields;
- copy `refs.<field>` to a top-level YAML field with string `id`;
- prepare the whole-body content:
  - from `--content-file`;
  - or from `--content-stdin`;
  - or as an empty string;
- if whole-body input is used, preserve the user body structure without rebuilding sections;
- if `content.sections.<name>` is used, build body only from section-level input and canonical schema order;
- order sections canonically according to the schema;
- serialize frontmatter in the canonical order defined in section 3.17;
- serialize the final document body.

If the type has no `content.sections` but the user tries to write `content.sections.<name>`, this is a write-contract error.
If the type has no `content`, the user may not use `--content-file` or `--content-stdin`.

### Stage 7. Compute the Canonical Path of the New Entity

Do:

- normalize `pathTemplate` into `cases` form;
- evaluate `when` left to right;
- select the first matching `use`;
- substitute built-ins, `meta.*`, and `refs.*`;
- obtain the relative POSIX path;
- check for collision with an existing document.

It is critical to compute the path from the fully assembled candidate, not from partial data. Otherwise the serialized document and write location may diverge.

If the final path is already occupied by another document, fail with `PATH_CONFLICT` and `error.exit_code = 1` before any write begins.

### Stage 8. Run Full Candidate Validation

Validate not the operations, but the complete final entity.

Check:

- built-in fields;
- `idPrefix`;
- uniqueness of `id`;
- uniqueness of `slug` within type;
- `meta.fields` by `required`, `required_when`, and `schema`;
- referential integrity and `refTypes`;
- `pathTemplate`;
- placeholder validity and computability;
- `content.sections`, including required sections and title validity;
- closed-world frontmatter;
- overall document consistency against the standard.

The output of this stage must be either:

- a valid candidate with `validation.ok = true`,
- or a diagnostics set in the same minimal shape as `validate`.

If validation fails:

- return a top-level `error`;
- use exit code `1`;
- include `error.details.validation.issues[]` when possible;
- do not create or move any files.

### Stage 9. Compute `revision` and Prepare the Response Payload

After the serialized Markdown has been fully stabilized:

- compute `revision` from the exact bytes of the final serialized Markdown document;
- apply the serializer's platform-native newline policy before computing `revision`;
- build `entity` in the baseline read shape:
  - built-in fields;
  - `meta`;
  - `refs` as object-view;
  - any additional fields required by the normative command payload.

For `refs.<field>`, the response must return object-view with `id` only.

### Stage 10. Implement `--dry-run`

If `--dry-run` is enabled:

- run all stages completely, including full validation and `revision` computation;
- write nothing to disk;
- return the same success payload as regular `add`, but with `dry_run: true`.

`dry-run` checks must be exactly as strict as real writes. Otherwise `dry-run` stops being a reliable predictor of the real result.

### Stage 11. Implement Atomic Write

For real writes:

- create missing parent directories of the target path;
- create a temporary file next to the target or on the same filesystem;
- write the serialized Markdown there;
- perform `fsync` if needed;
- atomically move the temp file to the target path;
- clean up the temp artifact on failure;
- guarantee absence of partially written results.

### Stage 12. Build the Final Response Contract

Successful JSON response must contain:

- `result_state`;
- `dry_run`;
- `created`;
- `entity`;
- `validation`.

For errors:

- top-level `error`;
- `error.code`;
- `error.message`;
- `error.exit_code`;
- and `error.details.validation.issues[]` when available.

Text output can exist as a convenience mode, but all contract integration tests must use `--format json`.

## 6. Integration Test Plan

### 6.1. General Approach

Integration tests must be black-box:

- each test creates a temporary workspace;
- the workspace receives the minimal schema and required fixture documents;
- the command is started as a real CLI process;
- the test checks:
  - JSON response;
  - exit code;
  - filesystem state after the command;
  - when useful, a follow-up `validate` run or direct reading of the created Markdown.

All normative checks must use `--format json`. Relaxed text output should be covered only by a smoke test if the project supports it for `add`.

### 6.2. Baseline Fixture Set

For `add`, one reference schema with two types is enough, for example:

- `service`:
  - simple type without mandatory references;
- `feature`:
  - `idPrefix`;
  - `pathTemplate` that uses `slug`, and in one scenario `refs.container.dirPath`;
  - `meta.fields.status` with string `enum`;
  - `meta.fields.container` with `schema.type: entityRef` and `refTypes: [service]`;
  - `content.sections.summary`;
  - `content.sections.implementation`.

This single fixture set lets the tests cover:

- `id` generation;
- write-path derivation from raw schema;
- `--set-file` restriction to section paths;
- referential integrity;
- `pathTemplate`;
- section handling;
- required/optional fields.

### 6.3. Mandatory Success Scenarios

#### T1. Minimal Creation of a Valid Entity

Setup:

- the workspace already contains one `service` that can be referenced.

Command:

- `spec-cli add --format json --type feature --slug retry-window --set refs.container=SVC-1`

Verify:

- exit code `0`;
- `result_state = "valid"`;
- `created = true`;
- `dry_run = false`;
- `entity.type = "feature"`;
- `entity.slug = "retry-window"`;
- generated `id` has the form `FEAT-N`;
- `createdDate` and `updatedDate` are present and equal;
- the date matches the UTC value of the test `Clock`;
- `entity.refs.container.id = "SVC-1"`;
- `type` and `slug` are absent from `entity.refs.container`;
- exactly one new file appeared on disk;
- the created document passes subsequent `validate`.

#### T2. Creation with Typed `meta` via `--set`

Command:

- `spec-cli add --format json --type feature --slug retry-window --set refs.container=SVC-1 --set meta.status=draft`

Verify:

- `entity.meta.status = "draft"`;
- the value really ends up in frontmatter;
- built-in frontmatter field order is canonical;
- the final document is valid.

#### T2a. YAML Typing for `typed_scalar` in `--set`

Setup:

- the fixture schema contains `meta` fields of types `integer`, `boolean`, `null`, and `string`.

Commands:

- `--set meta.priority=1`
- `--set meta.enabled=true`
- `--set meta.optional_note=null`
- `--set meta.literal_code='"001"'`
- `--set meta.literal_word='"true"'`

Verify:

- `1` is interpreted as integer;
- `true` as boolean;
- `null` as `null`;
- quoted values remain strings;
- typing matches the YAML frontmatter profile;
- the final document is valid.

#### T3. Whole-Body Creation via `--content-file`

Setup:

- a body file with known Markdown content.

Command:

- `spec-cli add --format json --type feature --slug retry-window --set refs.container=SVC-1 --content-file /abs/path/body.md`

Verify:

- the body is written without trimming;
- whole-body structure is preserved without section rebuild;
- `revision` matches the hash of the exact bytes of the final serialized document;
- final-file newline policy matches the current platform;
- section validation passes or fails strictly according to the file content.

#### T4. Body Creation via `--content-stdin`

Verify the same things as in `T3`, but for stdin.

#### T5. Section Creation via `content.sections.<name>`

Command:

- `spec-cli add --format json --type feature --slug retry-window --set refs.container=SVC-1 --set content.sections.summary='short text'`

Verify:

- the section is actually created in Markdown;
- the heading is generated by the serializer, not expected from the user;
- the section appears in the canonical place relative to the schema;
- the body is built canonically from section-level input;
- `validate` passes.

#### T5a. Section with `title` from Schema

Setup:

- `content.sections.summary` has `title`, for example `["Summary", "Overview"]`.

Command:

- `spec-cli add --format json --type feature --slug retry-window --set refs.container=SVC-1 --set content.sections.summary='short text'`

Verify:

- the serializer generates heading `## Summary {#summary}`;
- the first allowed `title` is chosen deterministically from schema;
- the user does not need to provide a heading in the CLI;
- the final document passes `validate`.

#### T6. Whole-Body and Section-Path Conflict

Command:

- whole-body via `--content-file`;
- plus `--set content.sections.summary=...`.

Verify:

- the command fails with `INVALID_ARGS`;
- exit code `2`;
- no write occurs;
- full candidate validation does not run.

#### T7. `--set-file` for a String Section Path

Command:

- `--set-file content.sections.summary=/abs/path/summary.md`

Verify:

- file content is read as-is;
- the section is created without losing line breaks;
- final validation succeeds.

#### T8. `--dry-run`

Verify:

- exit code `0`;
- `dry_run = true`;
- `created = true`;
- JSON payload contains the same `entity` as a real `add` would produce;
- JSON payload contains the same `validation` as a real `add`;
- no new files appear in the filesystem;
- rerunning without `--dry-run` creates the same document.

### 6.4. Mandatory Negative CLI and Contract Scenarios

#### T9. Missing Required `--type`

Verify `INVALID_ARGS`, exit code `2`, and no write.

#### T10. Missing Required `--slug`

Verify `INVALID_ARGS`, exit code `2`, and no write.

#### T11. Unknown `entity_type`

Verify `ENTITY_TYPE_UNKNOWN` and no write.

#### T12. `--content-file` and `--content-stdin` Provided Together

Verify `INVALID_ARGS`, exit code `2`.

#### T13. Same Write Path Passed Twice

Variants:

- twice via `--set`;
- once via `--set` and once via `--set-file`.

Verify `INVALID_ARGS`, exit code `2`.

#### T14. Attempt to Write a Path Outside the Locally Derived Write Contract

Example:

- `--set meta.unknown=value`

Verify:

- command fails with `WRITE_CONTRACT_VIOLATION`;
- `error.exit_code = 1`;
- no write occurs.

#### T15. Attempt to Write a Normatively Forbidden Path

Examples:

- `--set id=FEAT-999`;
- `--set content.raw=text`.

Verify:

- command fails with `WRITE_CONTRACT_VIOLATION`;
- `error.exit_code = 1`;
- no write occurs.

#### T16. `--set-file` Outside `content.sections.<name>`

Variants:

- `--set-file meta.status=/abs/path/value.txt`
- `--set-file refs.container=/abs/path/value.txt`

Verify:

- command fails with `WRITE_CONTRACT_VIOLATION`;
- `error.exit_code = 1`;
- no write occurs.

#### T16a. Whole-Body Input for a Type Without `content`

Setup:

- choose an entity type without `content` in raw schema.

Variants:

- `--content-file /abs/path/body.md`
- `--content-stdin`

Verify:

- command fails with `WRITE_CONTRACT_VIOLATION`;
- `error.exit_code = 1`;
- no write occurs.

#### T17. `--require-absolute-paths` Violation

Example:

- `--content-file relative.md --require-absolute-paths`

Verify `INVALID_ARGS`.

### 6.5. Mandatory Full-Validation Failure Scenarios

#### T18. Required Entity Data Not Provided

Example:

- `feature` requires `refs.container`, but the command does not provide it.

Verify:

- top-level `error`;
- exit code `1`;
- no write;
- if `issues[]` are available, they include the missing-required-field diagnostic;
- the error occurs during full candidate validation, not during CLI precheck.

#### T19. Invalid `slug`

Example:

- `Retry Window`

Verify validation failure and no write.

#### T20. Duplicate `slug` Within Type

Setup:

- the workspace already contains a `feature` with the same `slug`.

Verify validation failure and no write.

#### T21. Unresolvable or Wrong-Type `entityRef`

Variants:

- link to missing `id`;
- link to an entity not allowed by `refTypes`.

Verify validation failure and no write.

#### T22. Invalid Content Section

Example:

- schema requires `summary`, but neither whole-body nor section-path input creates it.

Verify validation failure and no write.

#### T23. `pathTemplate` Cannot Be Computed

Example:

- the selected case requires a placeholder from `refs.container.dirPath`, but the link cannot be resolved.

Verify validation failure and no write.

#### T24. Final Path Collision

Setup:

- another document already exists at the target path.

Verify:

- command fails with `PATH_CONFLICT`;
- `error.exit_code = 1`;
- existing file is unchanged;
- no new file appears.

### 6.6. Determinism and Atomicity Scenarios

#### T25. Deterministic `id` Generation

Setup:

- the workspace already contains `FEAT-0`, `FEAT-1`, `FEAT-4`.

Verify:

- new `id` is `FEAT-5`;
- the gap `FEAT-2` is not reused;
- behavior matches `max(existing N) + 1`.

#### T26. Atomicity on Write Failure

Artificially trigger a file-write failure after validation has already succeeded.

Verify:

- the target file is absent;
- no temp file remains;
- workspace after the command is identical to the state before the command.

#### T26a. Automatic Creation of Parent Directories

Setup:

- the canonical path of the new entity points into a nested directory that does not yet exist.

Verify:

- `add` creates missing parent directories successfully;
- the final file appears at the canonical path;
- the document passes `validate`.

#### T27. `revision` Changes with Actual Content

Create two entities with different final serialized Markdown and verify that `revision` differs.

#### T27a. `revision` Is Stable Within a Platform

On the same platform, two identically serialized documents must produce identical bytes and identical `revision`.

#### T27b. Frontmatter Order Is Deterministic

Verify:

- built-in fields are serialized in order `type`, `id`, `slug`, `createdDate`, `updatedDate`;
- `meta.fields` come after built-ins;
- order of `meta` fields matches their declaration order in raw schema;
- rerunning the same scenario on the same platform yields the same field order.

## 7. Recommended Work Order

Recommended implementation sequence:

1. Introduce `AddRequest` and CLI parsing.
2. Implement write-contract extraction directly from raw schema.
3. Implement workspace snapshot and indexes.
4. Implement write-path parsing and type conversion.
5. Implement candidate builder and content-section serialization.
6. Implement full candidate validation through the shared validator.
7. Implement `dry-run`.
8. Implement atomic write.
9. Build JSON success/error envelopes.
10. Add integration tests in blocks: happy path -> CLI errors -> validation failures -> atomicity.

## 8. Completion Criteria

The `add` implementation can be considered complete when all of the following are true:

- all mandatory scenarios from section 6 pass;
- successful `add` creates a document that passes `validate`;
- any validation failure returns a top-level `error` and leaves the filesystem unchanged;
- `dry-run` yields the same candidate as the real run, but without writing;
- `id`, dates, path, body, and `revision` are computed deterministically;
- behavior for every decision in section 3 is fixed in code and tests.
