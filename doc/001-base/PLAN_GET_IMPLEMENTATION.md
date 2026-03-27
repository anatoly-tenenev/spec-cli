# Detailed `get` Implementation Plan

Plan basis:

- `SPEC_UTILITY_CLI_API_BASELINE_RU.md`
- `SPEC_STANDARD_RU_REVISED_V3.md`

Plan limitation: it relies only on the two documents above and does not use any other sources.

## 1. Goal

Implement the baseline command:

```bash
spec-cli get [options] --id <id>
```

The command must:

- return one entity by exact `id`;
- use the canonical read namespace shared with `query`;
- not require the entire workspace to be fully valid;
- not hide an existing but invalid entity as `ENTITY_NOT_FOUND`;
- fail with a top-level error if the target entity cannot be parsed deterministically, its `type` cannot be determined, or the requested read fields cannot be computed;
- support the normative baseline JSON contract.

## 2. Normative `get` Contract

### Input

- required `--id <id>`;
- repeatable `--select <field>`;
- if `--select` is omitted, the default projection is:
  - `type`
  - `id`
  - `slug`
  - `revision`
  - `meta`

### Success Output

```json
{
  "result_state": "valid",
  "target": {
    "match_by": "id",
    "id": "FEAT-8"
  },
  "entity": {
    "type": "feature",
    "id": "FEAT-8",
    "slug": "retry-window",
    "revision": "sha256:def456",
    "meta": {
      "status": "active"
    }
  }
}
```

### Normative Rules

- unknown selector -> `INVALID_ARGS`;
- entity not found -> `ENTITY_NOT_FOUND`;
- if `content.sections.<name>` is selected and the section is missing, the field must be returned as `null`;
- `get` requires a valid and readable schema;
- schema errors must terminate the command with a top-level schema error;
- data-level violations in the target entity do not block `get` by themselves if the requested read fields are still computable;
- a missing value, including a missing required `meta.<name>`, is treated as an absent value;
- `get` must not return `ENTITY_NOT_FOUND` for an existing but invalid entity;
- if the target entity cannot be handled deterministically for the current request, partial success is not allowed.

## 3. Implementation Scope

The `get` implementation must include the following capabilities.

### 3.1. Schema Loading and Interpretation

- read the YAML schema by the effective path;
- validate that the schema can be used to build the read namespace;
- extract from the schema:
  - allowed `entity` types;
  - `meta.fields` definitions;
  - `content.sections` definitions;
  - `entityRef` field types;
  - read selectors available in the baseline model.

### 3.2. Canonical Read Namespace

Support these paths:

- built-ins:
  - `type`
  - `id`
  - `slug`
  - `createdDate`
  - `updatedDate`
  - `revision`
- meta:
  - `meta`
  - `meta.<name>`
- refs:
  - `refs`
  - `refs.<field>`
  - `refs.<field>.type`
  - `refs.<field>.id`
  - `refs.<field>.slug`
- content:
  - `content.raw`
  - `content.sections`
  - `content.sections.<name>`

### 3.3. Entity Lookup by `id`

- scan the workspace;
- parse candidate frontmatter;
- extract `id` before full document validation;
- match exact `id`;
- handle ambiguity when more than one document has the same `id`.

### 3.4. Deterministic Read of the Target Entity

For the target entity, `get` must be able to:

- parse YAML frontmatter;
- determine `type` from the `type` field;
- separate built-in fields from `meta.fields`;
- extract the raw document body;
- build a normalized section model under the standard rules;
- compute `refs` through successfully resolved `entityRef`;
- compute `revision` from the actual persisted document state.

### 3.5. Response Projection via `--select`

- support the default projection;
- validate every selector before reading the entity;
- materialize only the requested subgraph;
- merge overlapping selectors without duplication;
- return the whole object node for object-path selection;
- return `null` for missing `content.sections.<name>`.

## 4. Suggested Layer Decomposition

### Layer 1. Command CLI Wrapper

Responsibilities:

- parse `get` arguments;
- enforce required `--id`;
- collect `--select`;
- choose the JSON/text execution path;
- map errors to exit codes and top-level error payloads.

Result:

- a request structure with:
  - `id`
  - `selectors`
  - `output_format`
  - `workspace_path`
  - `schema_path`

### Layer 2. Schema Read Model

Responsibilities:

- load the schema;
- build read capabilities;
- determine allowed selectors;
- prepare the rules for computing `meta`, `refs`, and `content.sections`.

Result:

- a normalized read model suitable for both `get` and `query`.

### Layer 3. Entity Locator

Responsibilities:

- walk workspace files;
- extract frontmatter candidates;
- locate a document by `id`;
- distinguish:
  - target not found;
  - target found;
  - ambiguous target;
  - target physically found but not reliably parseable.

Result:

- descriptor of the located document or a domain lookup error.

### Layer 4. Entity Read Engine

Responsibilities:

- parse the target document;
- compute built-in fields;
- assemble `meta`;
- resolve `refs`;
- parse sections;
- compute `content.raw`;
- compute `revision`.

Result:

- internal read representation of the entity with absent-value support.

### Layer 5. Selector Projector

Responsibilities:

- apply selectors to the internal read representation;
- build the minimal correct JSON subgraph;
- merge overlaps correctly;
- respect the special case `content.sections.<name> = null`.

Result:

- `entity` payload for the JSON response.

### Layer 6. Error Mapping

Responsibilities:

- distinguish:
  - schema error;
  - invalid args;
  - entity not found;
  - blocking read error;
  - internal error;
- attach `error.details.validation.issues[]` when available.

## 5. Step-by-Step Implementation Plan

### Stage 1. Lock the Command Contract

Do:

- add/refine the command spec in CLI code;
- make `--id` required;
- make `--select` repeatable;
- lock the default projection.

Ready when:

- calling without `--id` yields `INVALID_ARGS`;
- calling with no selectors uses the default projection.

### Stage 2. Build a Shared Selector Validator for the Read Namespace

Do:

- implement selector normalization;
- validate selectors against the schema read model;
- distinguish object nodes from leaf nodes;
- prepare merge rules for overlaps.

Must support:

- `meta` and `meta.<name>`;
- `refs`, `refs.<field>`, `refs.<field>.<part>`;
- `content.raw`, `content.sections`, `content.sections.<name>`;
- built-in paths.

Ready when:

- unknown selector -> `INVALID_ARGS`;
- `meta` + `meta.status` does not break the result shape;
- `content.sections` + `content.sections.summary` merge correctly.

### Stage 3. Implement Target Lookup by `id`

Do:

- scan the workspace;
- extract `id` during a fast frontmatter read;
- do not require full document validity for candidate collection;
- handle:
  - `0` candidates;
  - `1` candidate;
  - `>1` candidates.

Ready when:

- missing `id` -> `ENTITY_NOT_FOUND`;
- duplicate `id` -> blocking error, not arbitrary file selection.

### Stage 4. Implement Tolerant Parsing of the Target Document

Do:

- parse YAML frontmatter under standard rules;
- require that `type` can be determined deterministically;
- allow missing optional or even required-for-full-validation fields when they are not needed for the current `get`;
- distinguish blocking parse failure from non-blocking validation failure.

Blocking cases for `get`:

- frontmatter cannot be parsed;
- `type` cannot be determined;
- at least one requested read field cannot be computed;
- the document structure does not allow deterministic construction of the requested read path.

Non-blocking cases for `get`:

- a required `meta.<name>` is missing but not needed or representable as absent;
- a required section is missing but requested fields remain computable;
- the entity has other full-validation violations that do not interfere with the current projection.

### Stage 5. Implement Built-ins and `meta`

Do:

- read `type`, `id`, `slug`, `createdDate`, `updatedDate` from frontmatter;
- assemble `meta` from non-built-in frontmatter fields declared in schema `meta.fields`;
- support absent values:
  - missing `meta.<name>` is not automatically a `get` error;
  - object `meta` contains only actually present and computable fields.

Ready when:

- `--select meta` returns only the allowed metadata subgraph;
- missing `meta.status` does not break `get`.

### Stage 6. Implement `refs`

Do:

- determine from the schema which metadata fields are `entityRef`;
- resolve them by `id` to target entities;
- return the expanded read-view:
  - `refs.<field>.id`
  - `refs.<field>.type`
  - `refs.<field>.slug`
- return `refs.<field>` as an object subgraph;
- if a link is missing or unresolved, treat it as absent unless that makes a requested field impossible to compute.

Important:

- the raw link value in frontmatter stays in `meta.<field>`;
- the expanded form is returned only under `refs`.

Ready when:

- `--select meta.container --select refs.container.id` returns the raw id and the expanded object in separate branches;
- unresolved links do not automatically break `get` when `refs` were not requested;
- if `refs.container.id` is requested and cannot be computed, the command fails.

### Stage 7. Implement `content.raw` and `content.sections`

Do:

- extract raw body after frontmatter;
- normalize sections according to the standard;
- support two label syntaxes:
  - `[<title>](#<label>)`
  - `<title> {#<label>}`
- extract section text by label;
- interpret `content.sections.<name>` as section body without heading;
- return `content.sections` as the object of all computable sections in the canonical namespace;
- for an explicitly requested missing section, return `null`.

Also verify:

- automatic label derivation from a heading without explicit labeling is not allowed;
- duplicate labels in the target are a blocking case if the request touches `content.sections`.

Ready when:

- `content.raw` returns the original body;
- `content.sections.summary` returns the section body;
- missing `summary` returns `null`;
- duplicate label on `content.sections.summary` request fails the command.

### Stage 8. Implement `revision`

Do:

- compute the string revision token from the actual document state;
- guarantee revision changes when frontmatter or body changes;
- connect `revision` to the shared read representation.

Ready when:

- two different states of the same document produce different `revision`;
- the same document produces a stable `revision`.

### Stage 9. Build the Response Projector

Do:

- materialize the subgraph by selectors;
- merge intersections;
- exclude unselected branches;
- return the full subgraph for object nodes;
- distinguish absent key from the `null` special-case for `content.sections.<name>`.

Ready when:

- `--select meta` returns the entire `meta`;
- `--select refs.container` returns the whole link object;
- `--select content.sections` returns the sections object;
- `--select content.sections.summary` returns only the requested branch.

### Stage 10. Implement Error Policy and Exit Codes

Do:

- `INVALID_ARGS` -> exit `2`;
- schema loading/schema shape failure -> exit `4`;
- `ENTITY_NOT_FOUND` -> exit `1`;
- blocking data-read error -> top-level domain error with exit `1`;
- unexpected internal error -> exit `5`.

Ready when:

- no blocking case returns partial `entity`;
- schema errors are not masked as `ENTITY_NOT_FOUND`;
- an existing but invalid entity is not masked as `ENTITY_NOT_FOUND`.

## 6. Suggested Internal Data Model

Minimal model to keep in code:

### Read Schema Model

- `entity_types`
- `meta_fields_by_type`
- `ref_fields_by_type`
- `content_sections_by_type`
- `allowed_selectors`

### Parsed Entity Descriptor

- `document_path`
- `raw_frontmatter`
- `raw_body`
- `frontmatter_map`
- `entity_type`
- `entity_id`

### Resolved Read Entity

- `type`
- `id`
- `slug`
- `createdDate`
- `updatedDate`
- `revision`
- `meta`
- `refs`
- `content.raw`
- `content.sections`

### Absent Value Policy

- a regular missing leaf value is not materialized in JSON unless required by a special case;
- `content.sections.<name>` is materialized as `null` when explicitly requested and missing;
- object nodes must not include non-computable child fields unless those children are requested or required to construct the object node itself.

## 7. What Should Be Shared with `query` from the Start

To avoid implementing the same logic twice, `get` should extract into shared modules:

- loading the read model from the schema;
- selector validation;
- read-namespace path parser;
- JSON subgraph projector;
- tolerant entity reader;
- `refs` resolver;
- `content.sections` parser.

Reason:

- the baseline explicitly defines one read namespace for both `query` and `get`;
- selector logic and projection should be identical;
- divergence between `get` and `query` response shapes will be a source of regressions.

## 8. Integration Tests

Below is the case set that covers baseline `get` behavior.

### Basic Success Scenarios

#### 1. `get_default_projection_ok`

Given:

- valid schema;
- valid entity `FEAT-8`.

Command:

- `spec-cli get --format json --id FEAT-8`

Expected:

- exit code `0`;
- `result_state = "valid"`;
- `target.match_by = "id"`;
- `target.id = "FEAT-8"`;
- `entity` contains:
  - `type`
  - `id`
  - `slug`
  - `revision`
  - `meta`

#### 2. `get_custom_select_ok`

Command:

- `spec-cli get --format json --id FEAT-8 --select meta.status --select refs.container.id --select content.sections.summary`

Expected:

- exit code `0`;
- `entity` contains only the requested branches;
- `meta.status` exists;
- `refs.container.id` exists;
- `content.sections.summary` exists.

#### 3. `get_selector_merge_ok`

Command:

- `spec-cli get --format json --id FEAT-8 --select meta --select meta.status --select content.sections --select content.sections.summary`

Expected:

- exit code `0`;
- `meta` is returned once as an object;
- `content.sections` is returned as a correct object;
- the JSON shape contains no duplicate/conflicting branches.

#### 4. `get_select_object_node_meta_ok`

Command:

- `spec-cli get --format json --id FEAT-8 --select meta`

Expected:

- `entity.meta` contains the full allowed and computable metadata subgraph.

#### 5. `get_select_object_node_refs_ok`

Command:

- `spec-cli get --format json --id FEAT-8 --select refs`

Expected:

- `entity.refs` contains all computable expanded refs.

#### 6. `get_select_content_raw_ok`

Command:

- `spec-cli get --format json --id FEAT-8 --select content.raw`

Expected:

- the raw body is returned without frontmatter.

### Missing-Value Scenarios

#### 7. `get_missing_section_returns_null`

Given:

- the target has no `summary` section.

Command:

- `spec-cli get --format json --id FEAT-8 --select content.sections.summary`

Expected:

- exit code `0`;
- `entity.content.sections.summary = null`.

#### 8. `get_missing_meta_field_is_absent_not_error`

Given:

- the target is missing a metadata field that is required by full validation.

Command:

- `spec-cli get --format json --id FEAT-8 --select meta`

Expected:

- exit code `0`;
- `get` succeeds;
- the missing field does not cause a top-level error.

#### 9. `get_missing_ref_not_requested_does_not_block`

Given:

- the target has an unresolved link.

Command:

- `spec-cli get --format json --id FEAT-8 --select meta`

Expected:

- exit code `0`;
- the command does not fail only because `refs` is broken when `refs` is not requested.

### Argument and Schema Errors

#### 10. `get_missing_id_arg`

Command:

- `spec-cli get --format json`

Expected:

- exit code `2`;
- `error.code = "INVALID_ARGS"`.

#### 11. `get_invalid_selector`

Command:

- `spec-cli get --format json --id FEAT-8 --select meta.unknown`

Expected:

- exit code `2`;
- `error.code = "INVALID_ARGS"`.

#### 12. `get_schema_missing`

Command:

- invoke with a missing schema.

Expected:

- exit code `4`;
- top-level schema error;
- no `entity`.

#### 13. `get_schema_unparseable`

Command:

- invoke with a YAML schema that cannot be parsed.

Expected:

- exit code `4`;
- top-level schema error.

#### 14. `get_schema_cannot_build_read_namespace`

Given:

- the schema is formally loaded but cannot be used to build the read namespace correctly.

Expected:

- exit code `4`;
- top-level schema error.

### Target Lookup by `id`

#### 15. `get_not_found`

Command:

- `spec-cli get --format json --id FEAT-404`

Expected:

- exit code `1`;
- `result_state = "not_found"`;
- `error.code = "ENTITY_NOT_FOUND"`.

#### 16. `get_duplicate_id_conflict`

Given:

- the workspace contains two documents with the same `id`.

Command:

- `spec-cli get --format json --id FEAT-8`

Expected:

- the command does not choose a file arbitrarily;
- top-level error;
- no partial `entity`.

### Existing but Invalid Entity

#### 17. `get_target_invalid_but_readable`

Given:

- the target entity violates full validation, for example it is missing a required section.

Command:

- `spec-cli get --format json --id FEAT-8 --select type --select id --select slug --select meta`

Expected:

- exit code `0`;
- the command succeeds;
- the entity is not masked as `ENTITY_NOT_FOUND`.

#### 18. `get_unrelated_invalid_document_does_not_block`

Given:

- the workspace contains another invalid document.

Command:

- `spec-cli get --format json --id FEAT-8`

Expected:

- exit code `0`;
- `get` succeeds for the readable target.

### Blocking Read Errors

#### 19. `get_target_frontmatter_unparseable`

Given:

- the target file exists, but frontmatter cannot be parsed.

Command:

- `spec-cli get --format json --id FEAT-8`

Expected:

- top-level error;
- no partial `entity`;
- if diagnostics are available, they are included in `error.details.validation.issues[]`.

#### 20. `get_target_type_cannot_be_determined`

Given:

- the target is found by `id`, but `type` is missing or cannot be determined.

Expected:

- top-level error;
- no partial `entity`.

#### 21. `get_requested_ref_cannot_be_computed`

Given:

- `refs.container.id` is requested, but the link cannot be resolved.

Command:

- `spec-cli get --format json --id FEAT-8 --select refs.container.id`

Expected:

- top-level error;
- no partial `entity`.

#### 22. `get_requested_sections_cannot_be_determined`

Given:

- the target has a duplicate section label or the structure is broken in a way that makes the needed section ambiguous.

Command:

- `spec-cli get --format json --id FEAT-8 --select content.sections.summary`

Expected:

- top-level error;
- no partial `entity`.

### `refs` Checks

#### 23. `get_refs_resolution_ok`

Given:

- the target has a valid `entityRef` link.

Command:

- `spec-cli get --format json --id FEAT-8 --select refs.container`

Expected:

- `refs.container.id`, `refs.container.type`, and `refs.container.slug` are computed correctly.

#### 24. `get_meta_ref_and_expanded_ref_are_distinct`

Command:

- `spec-cli get --format json --id FEAT-8 --select meta.container --select refs.container.id`

Expected:

- `meta.container` contains the raw frontmatter id;
- `refs.container.id` contains the expanded read-view value;
- the values are returned in separate branches without mixing.

### `revision` Checks

#### 25. `get_revision_present_in_default_projection`

Command:

- `spec-cli get --format json --id FEAT-8`

Expected:

- `entity.revision` is mandatory.

#### 26. `get_revision_changes_on_document_change`

Given:

- the same document in two different states.

Expected:

- `revision` changes when frontmatter changes;
- `revision` changes when body changes.

## 9. Integration Test Fixtures

Minimal test-data set:

- one valid schema with a type that includes:
  - regular metadata fields;
  - at least one `entityRef`;
  - at least two `content.sections`;
- one valid target entity;
- one target entity without a required section;
- one target entity with a missing required metadata field;
- one target entity with an unresolved link;
- one document with duplicate `id`;
- one document with broken frontmatter;
- one document with duplicate section labels;
- one valid target-linked document for `refs` expansion.

## 10. Completion Criteria

The `get` implementation can be considered complete if all of the following are true:

- the JSON contract matches the baseline;
- the default projection is implemented;
- selector validation is implemented;
- `content.sections.<name> -> null` on missing section is implemented;
- an existing but invalid entity is readable when the projection is computable;
- `ENTITY_NOT_FOUND` is returned only when the target is actually missing;
- blocking read failures do not lead to partial success;
- integration tests cover success path, absent values, schema errors, not found, duplicate id, blocking parse/read failures, `refs`, and `revision`.

## 11. Recommended Development Order

Practical order:

1. CLI contract and `--id` / `--select`.
2. Schema read model.
3. Selector validator and projector.
4. Target lookup by `id`.
5. Tolerant target parsing.
6. Built-ins + `meta`.
7. `revision`.
8. `refs`.
9. `content.raw` + `content.sections`.
10. Error mapping.
11. Integration tests.

This order reduces risk because:

- the read-namespace shape is locked before business logic;
- `get` and the future `query` get a shared read layer;
- integration tests can grow on top of an already stable projector.
