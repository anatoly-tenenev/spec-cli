# Specification for `query`/`get` Rework: Move from `--where-json` to `--where` + JMESPath

Status: working specification based on design decisions from `2026-03-30` to `2026-03-31`.

Note: the original Russian document was kept as a standalone working artifact in the repository root. This English version is added to the `doc/README.md` documentation index.

## 1. Goal

The current JSON-DSL filtering contract `query --where-json` must be replaced with `query --where <expr>` using JMESPath expressions, while the `query` and `get` read contracts must be aligned around a consistent `refs` model.

This change has three parts:

1. The `query` CLI contract.
2. The `query`/`get` read contract.
3. Schema-aware integration with `go-jmespath`.

## 2. Scope

In scope:

- removing `--where-json` and introducing `--where`;
- moving `query` to schema-aware JMESPath;
- aligning the `query` and `get` read contract around `refs`;
- new constraints for path-based `--sort` and `--select` that depend on the active type set;
- extending `go-jmespath` to support the required schema-aware mode.

Out of scope:

- `--where-file`;
- `--validate-query`;
- `--count-only`;
- a full rewrite of documentation and examples under `doc/`;
- separate work on expanded help text beyond minimally non-contradictory CLI help.

## 3. Normative `query` CLI Contract

### 3.1. Options

New contract:

```bash
spec-cli query [options]
```

Options covered by this task:

- `--type <entity_type>`: narrows the active type set early, repeatable.
- `--where <expr>`: JMESPath expression.
- `--select <field>`: projection path, repeatable.
- `--sort <field[:asc|desc]>`: sort term, repeatable.
- `--limit <n>`.
- `--offset <n>`.

### 3.2. Removal of `--where-json`

- `--where-json` is removed completely.
- Any use of `--where-json` is handled as an unknown option by the current parser layer.
- No special compatibility alias is introduced.

### 3.3. Repeated `--where`

- No more than one `--where` is allowed per query.
- A repeated `--where` returns `INVALID_ARGS`.
- The user combines conditions manually with `&&` / `||` inside a single expression.

### 3.4. `--where=<expr>` Form

- The inline form `--where=<expr>` is not specified separately.
- Its behavior is inherited from the current parser layer without special exceptions.

## 4. `--where` Semantics

### 4.1. Language

- `--where` accepts a JMESPath expression.
- The full function set available in the integrated `go-jmespath` version is used.
- `spec-cli` does not add project-specific helper functions on top of JMESPath.

### 4.2. Expression Result

- Any JMESPath result is allowed.
- Filtering is based on JMESPath truthiness.
- `boolean` is not required as the result type.

Consequences:

- scalar results are allowed;
- array results are allowed;
- object results are allowed;
- object/array constructors are not forbidden by themselves.

### 4.3. What Is Forbidden in `--where`

Forbidden constructs:

- any syntactic mention of `content.raw`;
- any access to `content` that does not go through `content.sections...`;
- access to `meta.<field>` for schema fields of type `entityRef`.

Allowed examples:

- `meta.status == 'active'`
- `refs.owner.id == 'SVC-1'`
- `length(refs.watchers[?reason == 'missing']) > \`0\``
- `content.sections.summary`
- `keys(content.sections)`

Disallowed examples:

- `content`
- `keys(content)`
- `content.raw`
- `meta.owner`

### 4.4. Validation of Forbidden Constructs

- The bans on `content.raw` and root `content` outside `content.sections...` are validated against the expression AST.
- String heuristics are forbidden.
- `spec-cli` must not try to guess the meaning of an expression from raw text.

## 5. Schema-Aware Execution Model

### 5.1. Active Type Set

The active type set is determined as follows:

- if `--type` is not specified, the active type set consists of all entity types from `schema.entity`;
- if `--type` is specified, the active type set consists only of the specified types.

### 5.2. Schema for `--where`

- The schema is built not for "the command as a whole", but for a single query item.
- Without `--type`, the root query-item schema is built through `oneOf` over the effective item shapes of the active type set.
- With `--type`, the schema is built from the narrowed type set.
- The required schema union for this task is the root `oneOf` over active item shapes; nested nullish behavior inside one item should be modeled primarily through ordinary object optionality and JMESPath missing semantics, not by introducing extra `oneOf(..., null)` at every nested path.

### 5.3. Responsibility Split Between `spec-cli` and `go-jmespath`

`spec-cli` is responsible for:

- building the query-item schema;
- materializing the runtime context;
- AST policy checks;
- mapping errors into the CLI contract.

`go-jmespath` is responsible for:

- branch/union analysis;
- nullable/optional analysis;
- compile-time schema-aware validation of the expression;
- runtime evaluation.

`spec-cli` must not:

- rewrite expressions manually into branch-aware form;
- add `type == '...'` guards automatically;
- emulate type narrowing with its own DSL pass on top of JMESPath.

Target requirement:

- expressions like `feature_field == '123' || service_field == 'test'` must be analyzed by the library based on schema, not by manual rewriting in the CLI layer.

## 6. Requirements for `go-jmespath`

The integrated version of `github.com/anatoly-tenenev/go-jmespath` already supports schema-aware compile, `format: date`, and `oneOf` in JSON Schema.

For this task, the library must provide:

- support for the root `oneOf` used by the query-item schema compiler;
- schema-aware analysis of alternative branches for query-item schemas;
- correct handling of nullable leaves and branch-specific properties;
- compatibility with the date semantics already in use (`format: date`).

`anyOf` and `allOf` are not required for this task:

- the `query item` is modeled through root `oneOf` over the active type set;
- nested nullable/missing behavior is not required to be expressed through additional `oneOf(..., null)` unions;
- no contract requirement is defined for schema composition through `allOf`;
- no contract requirement is defined for weaker union semantics through `anyOf`.

Library readiness for `oneOf` is part of the solution, not an optional optimization.

## 7. Normative Query-Item Schema Model

### 7.1. Built-ins

The schema-aware context uses the built-in fields:

- `type`
- `id`
- `slug`
- `revision`
- `createdDate`
- `updatedDate`

Typing:

- `type`: enum from the active type set.
- `id`: `string`.
- `slug`: `string`.
- `revision`: `string`.
- `createdDate`: `string` + `format: date`.
- `updatedDate`: `string` + `format: date`.

### 7.2. `meta`

- `meta` contains only schema-known non-ref fields.
- `meta.<entityRef>` is forbidden in the filter namespace.
- `enum` / `const` from the schema are carried into the schema-aware context.
- `integer` maps to `number`.
- `array` is supported as an array type with item typing as precisely as the library allows.
- `type: object` in `meta.fields.*.schema.type` is not supported within this change.
- If `type: object` appears in the schema, `query` and `get` must fail with a schema-load error under the same blocking contract as the local standard; the `--where` rework does not expand the read-side type profile on its own.
- `type: null` is modeled as a real `null`.
- `required` for `meta.fields` and `content.sections` is parsed under the same rules as in the local standard: `boolean`, or a string consisting entirely of a single interpolation `${expr}`.
- A syntactically invalid string `required` must break schema load, not silently degrade into optional.
- For the schema-aware type model, dynamic `required: ${expr}` may be treated conservatively as potential optionality/nullability if mandatory-ness cannot be determined at compile time, but the string itself still must pass valid interpolation parsing.

### 7.3. `content.sections`

- `content.sections` is allowed in `--where` as an object root.
- Root `content` is still forbidden, except for the path `content.sections...`.
- `content.sections` contains only schema-known sections.
- A section leaf is modeled as `string`; optionality is represented by whether the property is listed in `required`, not by wrapping the leaf itself into `string|null`.
- The full set of operations on section leaves is defined by `go-jmespath`; the old `where-json` special restrictions on sections are not preserved.

### 7.4. `refs`

The scalar ref object in the schema-aware context contains:

- `resolved`
- `id`
- `type`
- `slug`
- `reason`

Leaf typing:

- `resolved`: `boolean`
- `id`: `string`
- `type`: enum over the allowed target types
- `slug`: `string`
- `reason`: enum over unresolved reasons

Contract values for `reason`:

- `missing`
- `ambiguous`
- `type_mismatch`

`type` for a ref leaf:

- if the ref has `refTypes`, an enum built from them is used;
- if no narrowing is specified, an enum over all schema entity types is used.

Schema-model clarification:

- the schema-aware model does not need a separate `oneOf(ref-object, null)` branch for a scalar ref field;
- a scalar ref field is modeled as an optional object property under `refs`;
- nested ref leaves are also modeled as ordinary object properties rather than `oneOf(..., null)` unions;
- nullish behavior for explicit-null / absent refs is provided by ordinary JMESPath missing propagation at evaluation time.

Array ref field:

- modeled as an array;
- optionality of the array field is represented by property presence/absence, not by a field-level `array | null` union in the schema;
- ordinary JMESPath null propagation remains sufficient for runtime nullish array cases.

## 8. Runtime Materialization for `--where`

### 8.1. `meta`

- `meta` is allowed as an object root.
- In the runtime context, `meta` contains only schema-known non-ref fields that are actually present in the document.
- An explicit `null` in the document remains `field: null`.
- A missing field is removed from the `meta` object.

### 8.2. `refs`

- `refs` is allowed as an object root.
- In the runtime context, `refs` reflects actual data, not a full schema-shaped container.
- An explicit-null scalar ref is not included as a key in the root `refs` object.
- Resolved and unresolved scalar refs are included as objects.
- If an array ref field exists in frontmatter and was normalized successfully, it is always included in `refs`, including `[]` and `[null, null]`.

Consequence:

- `keys(refs)` reflects actual ref fields;
- access to a missing `refs.owner` in JMESPath returns `null` under ordinary missing semantics.
- the schema-aware compiler does not need to model this scalar-ref case as `refs.owner: ref-object | null`; ordinary missing-property semantics already provide the required `null` behavior for `refs.owner` and its descendants.

### 8.3. `content.sections`

- `content.sections` is materialized as an object root with pruning to schema-known sections that actually exist.
- If the document has no schema-known sections, `content.sections` still exists as an empty object `{}`.

## 9. Change to the `query` and `get` Read Contract

`query` and `get` must use an aligned read contract.

### 9.1. Unknown `type`

- If a document in the workspace has a `type` that is not present in `schema.entity`, the command fails with `READ_FAILED`.
- Unknown types are not silently skipped and are not included in a partial built-ins-only mode.

### 9.2. Syntactically Invalid Ref Values

Examples:

- `owner: {}`
- `owner: ""`
- `watchers: [SVC-1, {}]`

Rule:

- such values are treated as a read-model error;
- the command fails with `READ_FAILED`.

### 9.3. Scalar Ref: Full Object

#### a. Successfully resolved ref

The full public object contains:

- `resolved: true`
- `id`
- `type`
- `slug`

The `reason` field is absent in the full public object.

#### b. Explicit `null` in frontmatter

- Full `refs.<field>` is returned as `null`.

#### c. Unresolved ref

The full public object contains:

- `resolved: false`
- `id`
- `type`
- `slug: null`
- `reason`

For an unresolved ref:

- `type` is populated if it can be determined;
- if schema narrowing specifies exactly one type, that value may be used as a deterministic type hint.

### 9.4. `reason` Classification

`missing`:

- no targets were found for the raw id.

`type_mismatch`:

- something was found for the raw id, but after narrowing by hint/`refTypes`, no compatible targets remained.

`ambiguous`:

- after narrowing by hint/`refTypes`, more than one compatible target remained.

Important:

- if narrowing leaves exactly one compatible target, the ref is considered resolved;
- raw global ambiguity does not block resolution if narrowing makes the result deterministic.

### 9.5. Array Refs

For array refs:

- length and order are preserved;
- each element may be a resolved object, an unresolved object, or `null`;
- an unresolved element uses the same `reason` contract as a scalar unresolved ref.

## 10. Path-Based `--select`

### 10.1. General Rules

- `--select` remains a path-based projection.
- If the same path has different JSON types across active types, `--select` is still allowed.
- Different items may return different JSON types for the same selected path.

### 10.2. `refs.<field>.reason`

- `refs.<field>.reason` becomes an official leaf path.
- For a resolved ref, leaf projection by `reason` returns `null`.
- For an explicit-null ref, leaf projection by `reason` is also logically `null`, but if only that leaf is selected, the full projection does not synthesize a ref object on top of a full `null`.

### 10.3. Array-Ref Leaf Paths

Path-based leaf paths of the form:

- `refs.<field>.id`
- `refs.<field>.resolved`
- `refs.<field>.type`
- `refs.<field>.slug`
- `refs.<field>.reason`

are allowed only if `refs.<field>` is a scalar ref across the entire active type set.

If this field is an array ref in at least one type within the active type set:

- the path-based ref leaf is forbidden;
- the command fails with `INVALID_ARGS` / `INVALID_QUERY` at plan-validation time.

## 11. Path-Based `--sort`

### 11.1. Sort-Kind Compatibility

`--sort` is validated not globally against the old merged index, but relative to the active type set.

If the same path has incompatible sort kinds in the active type set:

- `--sort` on that path is forbidden;
- the command fails with `INVALID_ARGS` / `INVALID_QUERY`.

Example:

- `feature.meta.status` = `string`
- `service.meta.status` = `boolean`

Then without narrowing, sorting by `meta.status` is forbidden.

### 11.2. Ban on `meta.<entityRef>`

- `meta.<field>` is forbidden in `--sort` if that field is an `entityRef` in at least one type in the active type set.
- This rule must match `--select` and `--where`: ref data is read through `refs.<field>`, not through `meta.<field>`.
- If `meta.owner` is an `entityRef` in one type and a regular scalar meta field in another, `--sort meta.owner` is still forbidden without narrowing; the sorter must not "skip" the ref branch and leave the path valid.

### 11.3. Path Missing for Some Types

If a path exists for some active types and is absent for others:

- `--sort` is allowed;
- the current sorter missing semantics are used for the missing path.

The current rule remains:

- missing sorts before present in `asc`;
- missing sorts after present in `desc`.

### 11.4. `refs.<field>.reason`

- `refs.<field>.reason` is allowed in `--sort`.
- For resolved refs, this behaves as a missing/null value under current sorter semantics.

### 11.5. Array Refs

- Path-based leaf sort on an array ref is forbidden.
- If a ref field is scalar in some types and an array ref in others, path-based ref-leaf sort is also forbidden until the active type set is narrowed.

## 12. `--where` Semantics over `refs`

### 12.1. Scalar Ref with Explicit `null`

For an explicit-null scalar ref:

- full public `refs.<field>` remains `null`;
- in `--where`, access to `refs.<field>` and its descendant paths must naturally resolve to `null` under JMESPath missing semantics;
- `resolved` in this case is treated as `null`, not `false`.

Clarification:

- this behavior does not require a dedicated schema branch `oneOf(ref-object, null)` for the scalar ref field;
- an absent property under `refs` is sufficient for both `refs.<field>` and descendant paths to evaluate to `null`.

### 12.2. Resolved Ref

For `--where`, a resolved ref may contain synthetic `reason: null`, even though the full public output omits the `reason` field.

Goal:

- expressions like `refs.owner.reason == 'missing'` must be valid and simply evaluate to `false` on resolved refs.

Schema-model clarification:

- the compiler does not need a dedicated `reason: null` union branch for this case;
- ordinary missing/null propagation is sufficient as long as the path remains valid in the schema-aware object model.

### 12.3. Array Ref Navigation

Full JMESPath navigation over array refs is allowed in `--where`, for example:

- `length(refs.watchers[?reason == 'missing']) > \`0\``
- `refs.watchers[?resolved == \`false\`]`

This is allowed only in `--where`, not in path-based `--select` / `--sort`.

## 13. Errors and Exit Codes

### 13.1. Compile-Time `--where` Errors

Schema-aware compile / AST policy / invalid expression errors:

- code: `INVALID_QUERY`
- `error.message`: library-adjacent text is allowed
- internal library codes are not exposed externally as separate public error codes

### 13.2. Runtime `--where` Errors

If the expression passes compile time, but `go-jmespath` returns a runtime error on a specific record:

- the command fails with `READ_FAILED`.

`spec-cli` does not introduce special per-record fallback semantics such as "treat as false".

### 13.3. Path-Based Errors

- unknown or forbidden `--select` / `--sort` paths continue to be reported as CLI/query validation errors;
- removed `--where-json` is handled as an unknown CLI option by the parser layer.

## 14. Architectural Requirements for the Implementation

### 14.1. Reuse of the JMESPath Adapter

The current JMESPath wrapper lives under `validate/internal/...`.

For `query`, it must be moved into a shared package available to at least:

- `query`
- `validate`

The new shared wrapper must cover:

- compile schema;
- compile expression;
- search/evaluate;
- unified mapping of compile/runtime errors into `spec-cli`.

### 14.2. Restructuring the `query` Schema/Index Layer

The current `query/internal/schema/index.go` builds one merged index and forbids conflicting field kinds across `schema.entity`.

This must change:

- the global ban on conflicting field kinds across all entity types no longer fits;
- validation of `--sort` and `--select` must work relative to the active type set;
- the schema layer must know the distinction between scalar ref and array ref;
- the build-plan layer must be able to validate path-based operations relative to the active type set, not only against one global map.

### 14.3. Alignment of `query` and `get`

Ref-resolution logic must be aligned between `query` and `get`:

- identical handling of syntactically invalid ref values;
- identical unresolved contract;
- identical distinction between explicit `null` and unresolved;
- identical `reason` contract;
- identical schema-load contract for `required`, including rejection of syntactically invalid `${expr}`;
- identical list of supported read-side schema types; `type: object` remains out of contract until standardized separately.

## 15. Minimum Test Plan

The minimum mandatory integration cases are listed below.

When migrating existing cases from `--where-json` to `--where`, preserve the meaning of the scenarios as much as possible:

- preserving the verified contract behavior has priority over preserving the literal shape of the input filter;
- if an old case verified a specific observable semantic, the new case should verify that same semantic through a JMESPath expression whenever possible;
- replacing old cases with fundamentally different scenarios is allowed only where the old behavior is explicitly removed by the new contract.

### 15.1. Migration Rules and Case Hygiene

- Positive black-box cases that lock down the normative read/query contract must use schema-valid fixtures for all entities that carry the scenario's meaning; missing mandatory fields/sections must not be hidden inside an "ok" case as an implicit prerequisite.
- Nullable guards and fallback expressions such as `content.sections.summary || ''`, `refs.owner.id != null`, or `content.sections.implementation != null` are allowed in positive cases only when nullable semantics actually follow from the active type set or from a genuinely optional path in the schema, not from a broken fixture.
- If behavior on an invalid fixture must be locked down separately, it must be a dedicated explicitly named case with an explicit degradation rationale, not a hidden baseline scenario for schema-first behavior.
- Technical prefilters whose only purpose is to exclude records that violate mandatory `owner` / `summary` / `implementation` are not a good substitute for a valid dataset and must be removed during case migration.
- Black-box duplicates are forbidden: if `args`, `spec.schema.yaml`, `workspace.in`, and `response.json` are identical, only one canonical case must remain in the tree.

1. `query --where` with truthiness on scalar/object/array result.
2. Removed `--where-json`.
3. Ban on `content.raw`.
4. Ban on root `content` outside `content.sections...`.
5. Ban on `meta.<entityRef>`.
6. Ban on `--sort meta.<entityRef>` in a mixed active type set, including the case where one type has a ref and another has a scalar meta field.
7. `createdDate` / `updatedDate` comparisons via date semantics.
8. Multi-type `query --where` without `--type` through schema alternatives.
9. `feature_field == '123' || service_field == 'test'` without manual type guards, if the upgraded library supports it.
10. Allowed `--select` when one path has different JSON types in different types.
11. Forbidden `--sort` when one path has different sort kinds.
12. Allowed `--sort` when a path is missing for part of the types.
13. Unknown `type` in workspace -> `READ_FAILED`.
14. Syntactically invalid scalar ref -> `READ_FAILED`.
15. Syntactically invalid array-ref item -> `READ_FAILED`.
16. Syntactically invalid string `required` -> schema-load error in `query`.
17. `type: object` in read-side schema -> schema-load error equally for `query` and `get`.
18. Unresolved ref with `missing`.
19. Unresolved ref with `type_mismatch`.
20. Unresolved ref with `ambiguous`.
21. Deterministic resolution after narrowing by `refTypes`.
22. `refs.<field>.reason` in `--select`.
23. `refs.<field>.reason` in `--sort`.
24. Ban on path-based ref leaf for array ref.
25. Ban on path-based ref leaf under scalar/array conflict across the active type set.
26. Array-ref navigation in `--where`.
27. Matching `query` and `get` read contract for ref outputs.

## 16. Completion Criteria

The change is complete when all of the following are true at the same time:

1. `query --where-json` is removed and replaced with `--where`.
2. `query --where` uses schema-aware JMESPath.
3. `go-jmespath` supports the `oneOf`-based schema-aware mode and the related branch-aware analysis of alternatives required by `query`.
4. `query` and `get` return the same updated ref contract.
5. The active type set is taken into account in validation of `--where`, `--select`, and `--sort`.
6. `query` and `get` do not diverge in schema-load rules for `required` and in supported read-side types, including rejection of `type: object`.
7. All new contract cases are covered by black-box integration tests, and migrated positive cases do not depend on broken fixtures or explicit duplicates.
8. Minimal `query` help no longer lies about `--where-json`, even if expanded documentation is updated as a separate task.
