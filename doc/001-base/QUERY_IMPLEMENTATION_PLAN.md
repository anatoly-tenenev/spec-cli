# `query` Command Implementation Plan

This document is self-contained. To implement `spec-cli query`, the requirements, examples, and decisions defined here are sufficient. No other specification is required.

## 1. Purpose and Boundaries

`query` is the only CLI command for reading multiple entities.

The command must:

- read entities from the workspace;
- support early narrowing by `--type`;
- support the structured `--where-json` filter;
- support projection via `--select`;
- support deterministic sorting via `--sort`;
- support offset pagination only through `--limit` and `--offset`;
- return the normative machine response in JSON.

The command must not:

- return internal entity filesystem paths;
- use cursor pagination;
- depend on the write contract of `add/update/delete`.

## 2. Normative CLI Contract

### 2.1 Syntax

```bash
spec-cli query [options]
```

### 2.2 Command Options

- `--type <entity_type>`: early type narrowing, repeatable.
- `--where-json <json>`: structured JSON filter.
- `--select <field>`: include a field from the read namespace, repeatable.
- `--sort <field[:asc|desc]>`: sort order, repeatable.
- `--limit <n>`: page size, default `100`.
- `--offset <n>`: page offset, default `0`.

### 2.3 Relevant Shared CLI Rules

- Relevant global options:
  - `--workspace <path>`: workspace root, default `.`
  - `--schema <path>`: schema file path, default `spec.schema.yaml`
  - `--format <json|text>`: output format, default `text`
  - `--require-absolute-paths`: if set, any explicitly passed relative filesystem path must fail with `INVALID_ARGS`
- The CLI is non-interactive.
- For data commands, the normative machine mode is `--format json`.
- In `--format json`, data goes to `stdout`, diagnostics to `stderr`.
- Any unrecognized command argument must fail with `INVALID_ARGS`.
- `--limit` and `--offset` must be integers `>= 0`.
- The value `-` for stdin/stdout is not considered a relative path, but this rule is practically irrelevant for `query`.

### 2.4 Exit Codes

- `0`: success.
- `1`: expected domain failure of the command.
- `2`: invalid CLI arguments or invalid query.
- `3`: file read/write error.
- `4`: schema error.
- `5`: internal utility error.

For `query`, the practical branches are:

- `0` for valid execution, including `matched = 0`;
- `2` for `INVALID_ARGS`, `INVALID_QUERY`, `ENTITY_TYPE_UNKNOWN`;
- `3` for workspace read errors;
- `4` for invalid/incomplete schema;
- `5` for implementation defects.

### 2.5 JSON Error Shape

Any error in `--format json` must have this shape:

```json
{
  "result_state": "invalid",
  "error": {
    "code": "INVALID_ARGS",
    "message": "Unknown selector 'meta.unknown'",
    "exit_code": 2,
    "details": {}
  }
}
```

Mandatory minimum fields:

- `result_state`
- `error.code`
- `error.message`
- `error.exit_code`

## 3. Normative Read Namespace

`query` and `get` use the same read namespace.

### 3.1 Allowed Paths for `--select`

Built-in fields:

- `type`
- `id`
- `slug`
- `createdDate`
- `updatedDate`
- `revision`

Meta paths:

- `meta`
- `meta.<name>`

Refs paths:

- `refs`
- `refs.<field>`
- `refs.<field>.type`
- `refs.<field>.id`
- `refs.<field>.slug`

Content paths:

- `content.raw`
- `content.sections`
- `content.sections.<name>`

### 3.2 Shared Namespace Rules

- The same path is used in `--select`, `--sort`, `where-json.field`, and the JSON response.
- An object-node selector must return the whole corresponding subgraph.
- Overlapping selectors must merge without duplication.
- If the requested `content.sections.<name>` section is missing, the response must contain that field with value `null`.
- In the read contract, `refs.<field>` is returned as an expanded object, not as a scalar id.

### 3.3 Allowed Paths for `--sort`

Only leaf paths with orderable values are allowed for `--sort`:

- built-ins: `type`, `id`, `slug`, `createdDate`, `updatedDate`, `revision`
- `meta.<name>`
- `refs.<field>.type`
- `refs.<field>.id`
- `refs.<field>.slug`
- `content.raw`
- `content.sections.<name>`

Object paths forbidden for `--sort`:

- `meta`
- `refs`
- `refs.<field>`
- `content.sections`

If `--sort` contains a path outside this set, the command must fail with `INVALID_ARGS`.

### 3.4 Allowed Paths in `where-json`

Allowed filter leaf fields:

- `type`
- `id`
- `slug`
- `revision`
- `createdDate`
- `updatedDate`
- `meta.<name>`
- `refs.<field>.type`
- `refs.<field>.id`
- `refs.<field>.slug`
- `content.sections.<name>`

Additional rules:

- `content.raw` is forbidden in `where-json.field`;
- `content.sections.<name>` allows only `contains`, `exists`, `not_exists`;
- filtering on `content.sections.<name>` is lexical discovery / coarse prefilter, not semantic matching.

Object paths in `where-json.field` are forbidden.

## 4. Minimal Schema Contract Required by `query`

The source of truth for allowed types, selectors, and field typing is the loaded standard schema (`schema.entity`).

Minimal sufficient schema shape:

```yaml
version: "0.0.4"
entity:
  feature:
    idPrefix: FEAT
    pathTemplate: "features/{slug}.md"
    meta:
      fields:
        status:
          schema:
            type: string
            enum: [draft, active, deprecated]
        owner:
          schema:
            type: entityRef
            refTypes: [service]
    content:
      sections:
        summary: {}
        implementation: {}
```

From this model `query` must derive:

- the list of allowed `entity_type`;
- the list of allowed selectors for `--select`;
- the list of allowed leaf fields for `--sort`;
- the list of allowed leaf fields for `where-json`;
- field types for operator compatibility checks;
- enum values for `meta.<name>`, if declared;
- allowed `refs.<field>` from metadata fields of type `entityRef`;
- allowed `content.sections.<name>` from `content.sections`.

Rule:

- allowed fields are determined only from `schema.entity` plus the built-in read contract;
- actual workspace documents must not extend the contract.

## 5. Normative `--where-json` Language

### 5.1 General Shape

`--where-json` accepts a JSON object. Logical and leaf nodes are supported.

Logical nodes:

```json
{ "op": "and", "filters": [ ... ] }
{ "op": "or", "filters": [ ... ] }
{ "op": "not", "filter": { ... } }
```

Leaf node:

```json
{ "field": "meta.status", "op": "eq", "value": "active" }
```

For `exists` and `not_exists`, `value` is forbidden:

```json
{ "field": "content.sections.summary", "op": "exists" }
```

### 5.2 Supported Operators

- `eq`
- `neq`
- `in`
- `not_in`
- `exists`
- `not_exists`
- `gt`
- `gte`
- `lt`
- `lte`
- `contains`

### 5.3 Syntax Validation Rules

- `--where-json` must parse as valid JSON.
- A node must contain either a logical form or a leaf form, but not both.
- In `and` and `or`, `filters` is required and must be a non-empty array.
- In `not`, `filter` is required and must be exactly one nested node.
- In a leaf node, `field` and `op` are required.
- For `exists` and `not_exists`, `value` is forbidden.
- For all other leaf operators, `value` is required.
- For `in` and `not_in`, `value` must be a JSON array.

Any violation of these rules must fail with `INVALID_QUERY`.

### 5.4 Operator Semantics

- `eq`: literal equality.
- `neq`: literal inequality.
- `in`: the current field value must equal one of the array items in `value`.
- `not_in`: the current field value must not equal any array item in `value`.
- `exists`: true if the value is present.
- `not_exists`: true if the value is absent.
- `gt`, `gte`, `lt`, `lte`: regular value comparison.
- `contains`:
  - for strings: substring search;
  - for arrays: literal element containment.

### 5.5 Type Compatibility

Validate not only JSON shape but also compatibility of `field`, `op`, and `value`.

Rules:

- `gt/gte/lt/lte` are allowed only for numbers and dates formatted as `YYYY-MM-DD`.
- Dates are compared lexicographically as strings.
- `contains` is allowed for strings and arrays.
- If an operator is applied to an incompatible field type, that is `INVALID_QUERY`, not `false`.
- For enum fields, `eq`, `neq`, `in`, `not_in` must accept only enum values, with case sensitivity.
- `content.sections.<name>` allows only `contains`, `exists`, `not_exists`.
- `content.raw` is not allowed in `where-json.field`.

Practical compatibility table:

- strings (except `content.sections.<name>`): `eq`, `neq`, `in`, `not_in`, `contains`, `exists`, `not_exists`
- `YYYY-MM-DD` dates: `eq`, `neq`, `in`, `not_in`, `gt`, `gte`, `lt`, `lte`, `contains`, `exists`, `not_exists`
- numbers: `eq`, `neq`, `in`, `not_in`, `gt`, `gte`, `lt`, `lte`, `exists`, `not_exists`
- arrays: `eq`, `neq`, `in`, `not_in`, `contains`, `exists`, `not_exists`
- `content.sections.<name>`: `contains`, `exists`, `not_exists`

### 5.6 Missing-Value Semantics

- `exists` returns `true` if the value is present.
- `not_exists` returns `true` if the value is absent.
- All other leaf operators return `false` when the value is absent.

### 5.7 Valid Filter Examples

By type and status:

```json
{
  "op": "and",
  "filters": [
    { "field": "type", "op": "eq", "value": "feature" },
    { "field": "meta.status", "op": "eq", "value": "active" }
  ]
}
```

By date:

```json
{
  "field": "updatedDate",
  "op": "gte",
  "value": "2026-03-01"
}
```

Section presence:

```json
{
  "field": "content.sections.summary",
  "op": "exists"
}
```

Lexical section prefilter:

```json
{
  "field": "content.sections.summary",
  "op": "contains",
  "value": "retry"
}
```

## 6. Normative `query` Response

### 6.1 Successful JSON Response

```json
{
  "result_state": "valid",
  "items": [
    {
      "type": "feature",
      "id": "FEAT-8",
      "slug": "retry-window",
      "meta": {
        "status": "active"
      }
    }
  ],
  "matched": 1,
  "page": {
    "mode": "offset",
    "limit": 100,
    "offset": 0,
    "returned": 1,
    "has_more": false,
    "next_offset": null,
    "effective_sort": ["type:asc", "id:asc"]
  }
}
```

Mandatory success fields:

- `result_state`
- `items`
- `matched`
- `page.mode`
- `page.limit`
- `page.offset`
- `page.returned`
- `page.has_more`
- `page.next_offset`
- `page.effective_sort`

### 6.2 Response Rules

- On success, `result_state` is always `valid`.
- No matches is still success: `result_state = "valid"`, `matched = 0`, `items = []`.
- If `--select` is omitted, default projection is `type`, `id`, `slug`.
- If `--sort` is omitted, default sort is `type:asc`, then `id:asc`.
- If `--limit 0`, return page aggregates without items.

### 6.3 Fixed Decisions for Ambiguous Areas

Additional rules fixed to make this document self-sufficient:

- With `--limit 0`, `items` is mandatory and equals `[]`.
- If `offset >= matched`, then:
  - `items = []`
  - `page.returned = 0`
  - `page.has_more = false`
  - `page.next_offset = null`
- When the user provides `--sort`, the implementation must append a hidden tail `type:asc`, `id:asc` if the resulting sort list does not already end with exactly that tail.
- If the user already specified `type` or `id` earlier with a different direction, that is still allowed; the hidden tail is added only as a tie-breaker at the end.
- For sorting, missing values are less than present ones in `asc` and greater than present ones in `desc`.
- Sorting must be deterministic and stable for equal keys.

## 7. Command Error Map

### 7.1. `INVALID_ARGS`

Use in cases of:

- unknown selector in `--select`;
- unknown or object-path in `--sort`;
- invalid `--sort` syntax;
- `--limit` or `--offset` is not an integer `>= 0`;
- unknown CLI arguments.

### 7.2. `INVALID_QUERY`

Use in cases of:

- `--where-json` is not valid JSON;
- invalid logical filter structure;
- empty `filters` in `and` or `or`;
- unknown operator;
- unknown field in `where-json`;
- object-path in `where-json.field`;
- invalid combination of `field/op/value`;
- type mismatch;
- invalid enum value;
- `exists/not_exists` contains `value`;
- `in/not_in` did not receive an array;
- `gt/gte/lt/lte` applied to something that is neither a number nor a date;
- `content.raw` used in `where-json.field`;
- an operator other than `contains|exists|not_exists` used for `content.sections.<name>`.

Recommended machine diagnostics in `error.details` for policy bans:

- `arg = "--where-json"`
- for `content.raw`: `reason = "forbidden_field"`, `field = "content.raw"`
- for a forbidden operator on `content.sections.<name>`: `reason = "forbidden_operator_for_field"`, `field`, `operator`

### 7.3. `ENTITY_TYPE_UNKNOWN`

Use if any `--type` value does not exist in `schema.entity`.

## 8. What Must Exist at Runtime

### 8.1. Schema Loading

There must be a loader that returns the effective standard schema and validates the minimum structure needed by `query`.

Mandatory minimum:

- `entity`
- `entity.<type>.meta.fields`
- `entity.<type>.meta.fields.<name>.schema.type`
- `entity.<type>.meta.fields.<name>.schema.enum` (if present)
- `entity.<type>.content.sections`, if the type supports sections

### 8.2. Workspace Reading

There must be a workspace entity enumerator that can retrieve for each entity:

- built-in fields;
- metadata;
- links for the expanded read-view;
- raw content;
- parsed `content.sections`.

### 8.3. Building the Full Read-View

Each entity needs a canonical full read-view used later for:

- filtering;
- sorting;
- final projection.

Recommended full read-view:

```json
{
  "type": "feature",
  "id": "FEAT-8",
  "slug": "retry-window",
  "revision": "sha256:def456",
  "createdDate": "2026-03-10",
  "updatedDate": "2026-03-10",
  "meta": {
    "status": "active",
    "owner": "platform"
  },
  "refs": {
    "container": {
      "type": "service",
      "id": "SVC-2",
      "slug": "billing-api"
    }
  },
  "content": {
    "raw": "## Summary\nRetry window...",
    "sections": {
      "summary": "Retry window...",
      "implementation": "Use backoff..."
    }
  }
}
```

### 8.4. Shared JSON Writer

A shared writer is needed for:

- successful JSON responses;
- error responses with `result_state`, `error.code`, `error.message`, `error.exit_code`.

## 9. Implementation Plan

### Stage 1. Normalize Command Input

Build `QueryRequest`:

- `types[]`
- `where_json_raw`
- `selects[]`
- `sorts[]`
- `limit`
- `offset`

Immediately validate:

- defaults for `limit` and `offset`;
- integer/non-negative constraints for `limit/offset`;
- basic `field[:asc|desc]` syntax in `--sort`.

Result:

- one place that emits `INVALID_ARGS` for argument problems.

### Stage 2. Build `QuerySchemaIndex`

Build from the standard schema (`schema.entity`) an index known to all later steps:

- allowed `entity_type`;
- allowed selectors;
- allowed sort fields;
- allowed filter fields;
- type of each leaf field;
- enum restrictions;
- mapping from metadata `entityRef` to `refs.<field>.type|id|slug`;
- mapping of `content.sections.<name>`.

Recommended structure:

```text
QuerySchemaIndex
  entityTypes
  selectorSpecs
  sortFieldSpecs
  filterFieldSpecs
  enumSpecs
```

### Stage 3. Implement the `--select` Projector

Needed:

- selector validator through `QuerySchemaIndex`;
- path canonicalization;
- merge of overlapping selectors;
- projector from full read-view to response item.

Normative rules:

- default projection: `type`, `id`, `slug`;
- unknown selector -> `INVALID_ARGS`;
- object-node selector returns the whole subgraph;
- missing `content.sections.<name>` -> `null`.

### Stage 4. Implement Parser and Binder for `--where-json`

Split the work into two steps:

1. parse raw JSON -> untyped AST;
2. bind AST against `QuerySchemaIndex` -> typed AST.

After binding it must be known that:

- all fields are allowed;
- the operator is allowed for the field type;
- `value` has the correct shape;
- enum values are valid.

Any error at this stage must fail with `INVALID_QUERY`.

### Stage 5. Implement the Filter Evaluator

Evaluator input:

- typed AST;
- the entity full read-view.

Required helper:

```text
resolve_read_value(entityView, path) -> { present, value }
```

It must work uniformly for:

- built-ins;
- `meta.<name>`;
- `refs.<field>.*`;
- `content.sections.<name>` for `where-json`;
- `content.raw` and `content.sections.<name>` for `--sort` / `--select`.

### Stage 6. Implement the Selection Pipeline

Recommended order:

1. load schema (`schema.entity`);
2. build `QuerySchemaIndex`;
3. validate `--type`;
4. validate `--select`;
5. validate and bind `--where-json`, if provided;
6. enumerate workspace entities;
7. early filter by `--type`;
8. build full read-view for remaining entities;
9. apply `where-json`;
10. sort the matched set;
11. apply `offset/limit`;
12. project `items` via `--select`;
13. build the final JSON response.

Reason for this order:

- the full read-view is needed for filtering and sorting;
- projection must happen at the end so `--select` does not affect filter and sort behavior.

### Stage 7. Implement Sorting and Pagination

Sorting requires:

- `QuerySortTerm { path, direction }`;
- default sort `type:asc`, `id:asc`;
- hidden suffix `type:asc`, `id:asc`;
- comparator with deterministic handling of missing values.

Pagination requires:

- `matched`: count after filtering and before paging;
- `returned`: count after paging;
- `has_more`: `matched > offset + returned`;
- `next_offset`: `offset + returned` if `has_more`, else `null`;
- `effective_sort`: final list of sort terms after hidden-tail injection.

### Stage 8. Build Response and Error Mapping

Success:

- `result_state = "valid"`
- `items`
- `matched`
- `page`

Errors:

- `INVALID_ARGS`
- `INVALID_QUERY`
- `ENTITY_TYPE_UNKNOWN`
- infrastructure errors with correct `exit_code`

### Stage 9. Update `query` Help

`query` help must explicitly describe:

- fixed sections in order: `Command`, `Syntax`, `Options`, `Rules`, `Examples`, `Schema`;
- canonical command syntax;
- all options;
- which arguments use the read namespace;
- the full `--where-json` language;
- allowed logical nodes;
- allowed leaf operators;
- allowed fields;
- typing rules;
- missing-value semantics;
- short copy-paste-ready examples;
- effective schema path and the full verbatim loaded schema text in the `Schema` section.

## 10. Test Matrix

Minimum required tests.

### 10.1. Basic Happy Path

- query without `--where-json`;
- default projection;
- default sort;
- `matched = 0`;
- multiple entity types without `--type`;
- early narrowing by one `--type`;
- early narrowing by multiple `--type`.

### 10.2. `--select`

- one leaf selector;
- multiple leaf selectors;
- object-node selector `meta`;
- object-node selector `refs.container`;
- merge of overlapping selectors;
- unknown selector -> `INVALID_ARGS`;
- missing `content.sections.<name>` -> `null`;
- `refs.<field>` returned as expanded object.

### 10.3. `--where-json`

- `eq`, `neq`;
- `in`, `not_in`;
- `exists`, `not_exists`;
- `gt`, `gte`, `lt`, `lte` on dates;
- `gt`, `gte`, `lt`, `lte` on numbers;
- `contains` on `content.sections.<name>`;
- `contains` on arrays if the schema contains arrays;
- `and`, `or`, `not`;
- nested logical expressions;
- missing value for leaf operators other than `exists/not_exists`;
- unknown operator -> `INVALID_QUERY`;
- unknown field -> `INVALID_QUERY`;
- `content.raw` in `where-json.field` -> `INVALID_QUERY`;
- `content.sections.<name>` with `eq/neq/in/not_in/gt/gte/lt/lte` -> `INVALID_QUERY`;
- object-path in `where-json.field` -> `INVALID_QUERY`;
- empty `and/or` -> `INVALID_QUERY`;
- `exists/not_exists` with `value` -> `INVALID_QUERY`;
- `in/not_in` without array -> `INVALID_QUERY`;
- type mismatch -> `INVALID_QUERY`;
- invalid enum value -> `INVALID_QUERY`.

### 10.4. Sorting

- sort by one field;
- sort by multiple fields;
- default sort;
- user sort + hidden tail;
- sort on missing values;
- object-path in `--sort` -> `INVALID_ARGS`;
- invalid sort direction -> `INVALID_ARGS`.

### 10.5. Pagination

- `limit = 0`;
- `limit > 0`;
- `offset = 0`;
- `offset` within range;
- `offset >= matched`;
- correct `returned`;
- correct `has_more`;
- correct `next_offset`;
- correct `effective_sort`.

### 10.6. Errors and Infrastructure

- unknown `entity_type` -> `ENTITY_TYPE_UNKNOWN`;
- invalid JSON in `--where-json` -> `INVALID_QUERY`;
- invalid `--limit` -> `INVALID_ARGS`;
- invalid `--offset` -> `INVALID_ARGS`;
- workspace file read error -> exit code `3`;
- schema error -> exit code `4`.

## 11. Recommended Delivery Order

To get a working vertical slice faster without rewrites:

1. `QueryRequest` and argument parsing.
2. `QuerySchemaIndex`.
3. Full read-view builder.
4. `--select` projector.
5. sort and pagination without `--where-json`.
6. `--where-json` parser, binder, and evaluator.
7. error mapping.
8. `help query`.
9. negative and boundary tests.

## 12. Definition of Done

`query` is considered implemented when all of the following are true:

- the command accepts all options from this document;
- all selectors, sort fields, and filter fields are validated through the standard schema (`schema.entity`, `meta.fields`, `content.sections`);
- `--where-json` supports the complete language described here;
- `content.raw` is forbidden in `where-json`, and only `contains|exists|not_exists` are allowed for `content.sections.<name>`;
- sorting and offset pagination are deterministic;
- `items`, `matched`, and `page.*` match the contract;
- errors map to the correct `code` and `exit_code`;
- `help query` explains the read namespace and filter language without external documentation;
- the minimal test matrix passes.
