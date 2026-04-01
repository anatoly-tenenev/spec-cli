# Query Schema `const`/`enum` Review 2026-04-01

This document records a reproduced schema-validation bug in the current `query`/`get` read-side implementation.

## Context

- The current implementation accepts malformed `meta.fields.*.schema` combinations where `type` is incompatible with `const` or `enum`.
- The failure may remain hidden during schema load and surface only later when `query --where` tries to build the schema-aware JMESPath context.
- This behavior conflicts with the project-wide contract that structurally invalid schema must fail early with `SCHEMA_INVALID`.

## Finding

### High: malformed `const`/`enum` is not rejected at schema-load time

In [`internal/application/commands/query/internal/schema/loader.go`](../internal/application/commands/query/internal/schema/loader.go), `enum` is checked only for being a non-empty array, while `const` is accepted without validating compatibility with the declared `type`.

The same class of validation is also absent from the current `get` read-model loader in [`internal/application/commands/get/internal/schema/loader.go`](../internal/application/commands/get/internal/schema/loader.go).

Later, when `query --where` builds the schema-aware JMESPath context in [`internal/application/commands/query/internal/engine/where.go`](../internal/application/commands/query/internal/engine/where.go), the library rejects the malformed schema and the CLI maps it to `INVALID_QUERY` instead of a schema error.

Reproducible example:

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
            const: 1
```

Observed behavior:

1. `spec-cli query` succeeds.
2. `spec-cli get --id FEAT-1` succeeds.
3. `spec-cli query --where "id == 'FEAT-1'"` fails with:

```json
{
  "error": {
    "code": "INVALID_QUERY",
    "message": "failed to compile where schema context: unsupported_schema: $.properties.meta.properties.status: schema type does not match const/enum values",
    "exit_code": 2
  },
  "result_state": "invalid"
}
```

## Why This Is a Bug

- The root problem is in the schema, not in the user-provided `--where` expression.
- The same malformed schema should not behave differently depending on whether `--where` is present.
- The current behavior leaks an internal downstream validation failure from the JMESPath integration layer and misclassifies it as a query error.

## Expected Contract

The schema above must be rejected during schema load with:

- `error.code = SCHEMA_INVALID`
- `error.exit_code = 4`

That classification should be consistent for:

- `query`
- `query --where ...`
- `get`

## Recommended Fix Direction

- Validate `type` vs `const` compatibility during schema load.
- Validate every `enum` member against the declared `type` during schema load.
- Apply the same validation rule set to both `query` and `get` schema loaders so the read-side contract stays aligned.
- Keep downstream JMESPath schema-compile failures reserved for real expression/schema-context integration problems, not for malformed source schema that can be rejected earlier.
