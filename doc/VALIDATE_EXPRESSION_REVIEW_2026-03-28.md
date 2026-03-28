# Validate Expression Review 2026-03-28

This document records review findings for the current changes around `validate`, `pathTemplate`, and the new interpolation engine based on `go-jmespath`.

## Context

- The current staged diff was reviewed.
- `make vet` and `make test` were also executed.
- Specific scenarios were reproduced locally through temporary harness checks inside the module.

## Findings

### 1. High: `pathTemplate.use` accepts interpolations that are not statically safe

In [`internal/application/commands/validate/internal/expressions/compiler.go`](../internal/application/commands/validate/internal/expressions/compiler.go), `CompileModeTemplatePart` treats an expression as valid if its inferred type may be `string`, `number`, or `boolean`, even when it may also evaluate to `null`.

After that, [`internal/application/commands/validate/internal/schema/internal/entity/internal/pathpattern/parser.go`](../internal/application/commands/validate/internal/schema/internal/entity/internal/pathpattern/parser.go) checks only for guard roots, but does not guarantee that the expression in `use` cannot evaluate to `null` on an allowed execution branch.

Reproducible example:

```yaml
pathTemplate:
  cases:
    - use: "docs/${meta.foo || meta.bar}.md"
```

With two optional fields, the schema is accepted, but during rendering in [`internal/application/commands/validate/internal/expressions/interpolation.go`](../internal/application/commands/validate/internal/expressions/interpolation.go) the expression may return `null`, which then leads to `instance.interpolation.type_mismatch`.

This conflicts with the static-safety requirement for `pathTemplate.cases[].use` in [`doc/001-base/SPEC_UTILITY_CLI_VALIDATE_COMPLETION_PLAN_2026-03-09.md`](./001-base/SPEC_UTILITY_CLI_VALIDATE_COMPLETION_PLAN_2026-03-09.md):

- `pathTemplate.cases[].use` must allow only statically safe value usage;
- schema-time and runtime errors must remain clearly separated.

### 2. Medium: guard analysis rejects safe expressions in `pathTemplate.use`

In [`internal/application/commands/validate/internal/schema/internal/entity/internal/pathpattern/parser.go`](../internal/application/commands/validate/internal/schema/internal/entity/internal/pathpattern/parser.go), `GuardedPathsWhenTrue()` is used as if it indicates which paths "must be protected by `when`". In reality, that API means something different: which paths are guaranteed to be present when the expression itself has already evaluated to `true`.

Because of that, expressions that are safe by themselves and do not require an additional `when` are rejected.

Reproducible example:

```yaml
pathTemplate:
  cases:
    - use: "docs/${meta.foo == 'x'}.md"
```

This template is currently rejected with `schema.pathTemplate.use_missing_guard`, even though the expression compiles correctly and safely renders to `docs/false.md` when `foo` is missing.

This makes the static validation stricter than described in [`doc/001-base/SPEC_UTILITY_CLI_VALIDATE_COMPLETION_PLAN_2026-03-09.md`](./001-base/SPEC_UTILITY_CLI_VALIDATE_COMPLETION_PLAN_2026-03-09.md): the expected behavior is to validate static safety of placeholder/interpolation usage, not to require an external guard for every path that appears in guard analysis.

## Coverage Note

At the time of review, `make vet` and `make test` both pass, which means the two scenarios above are not currently captured by tests as regressions.
