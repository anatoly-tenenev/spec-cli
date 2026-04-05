# Schema Compiler Raw vs Semantic Model Split

## 1. Purpose

This document defines the cleaner target architecture for the shared schema compiler beyond the current migration step.

The immediate bug around duplicated schema diagnostics (`schema.value.const_type_mismatch` plus derived `schema.expression.context_invalid`) is only one symptom of a broader modeling problem:

- one compiler model currently carries both
  - normalized source semantics, including invalid constraints that were present in the source schema;
  - downstream execution semantics used to build derived schemas and capabilities.

The target architecture should separate these concerns explicitly.

## 2. Problem Statement

Today the compiler can:

1. correctly diagnose a primary schema error;
2. keep the invalid facet inside the compiled model;
3. feed that invalid facet into a downstream derived schema builder;
4. receive a second, derivative failure from that downstream builder.

Example:

- source schema says `type: string` and `const: 1`;
- semantic validation correctly emits `schema.value.const_type_mismatch`;
- expression-context builder serializes the same invalid `const` back into a JMESPath schema;
- JMESPath rejects that derived schema and the compiler emits `schema.expression.context_invalid`.

This is undesirable because:

- the root cause is already known;
- the second error is derivative noise, not an independent schema defect;
- downstream builders become sensitive to invalid source facets that should already be quarantined by the compiler.

## 3. Design Goal

The compiler should enforce this invariant:

> Invalid source facets may produce diagnostics, but they must not remain active constraints in downstream semantic projections.

That means:

- source truth is preserved for diagnostics;
- semantic truth for execution is preserved for downstream builders;
- derived schemas and capabilities are built only from semantically usable constraints.

## 4. Target Split

### 4.1. Raw / Normalized Layer

The first model keeps the normalized structure of the source schema.

Responsibilities:

- preserve what the user wrote, after shape normalization;
- keep enough information for deterministic diagnostics and paths;
- represent invalid and valid facets alike;
- serve as the input to semantic validation.

This layer is diagnostic-oriented, not execution-oriented.

Illustrative shape:

```go
type NormalizedValueSpec struct {
    Kind        ValueKind
    Format      string
    Enum        []Literal
    Const       *Literal
    Ref         *RefSpec
    Items       *NormalizedValueSpec
    UniqueItems bool
    MinItems    *int
    MaxItems    *int
}
```

The exact type names are not important. The important property is that this layer may still contain invalid combinations because it mirrors the normalized source.

### 4.2. Semantic / Usable Layer

The second model contains only constraints that are safe for downstream use.

Responsibilities:

- expose the schema semantics that remain meaningful after validation;
- exclude constraints that were already proven invalid;
- act as the only input for capability builders and derived-schema builders.

Illustrative shape:

```go
type SemanticValueSpec struct {
    Kind        ValueKind
    Format      string
    Enum        []Literal
    Const       *Literal
    Ref         *RefSpec
    Items       *SemanticValueSpec
    UniqueItems bool
    MinItems    *int
    MaxItems    *int
}
```

The field set may look similar, but the invariant is different:

- `SemanticValueSpec.Const` is present only if it is semantically valid for `Kind`;
- `SemanticValueSpec.Enum` contains only semantically valid members;
- nested specs follow the same rule recursively.

## 5. Operational Rule

The compile pipeline should become:

1. load source;
2. normalize into raw/normalized schema;
3. validate raw schema and emit diagnostics;
4. project raw schema into semantic/usable schema;
5. build derived schemas and command capabilities only from semantic schema.

Important consequence:

- downstream builders may still see an overall invalid schema run;
- but they must not be asked to interpret already-invalid constraints as executable semantics.

## 6. Recovery Policy

This architecture does not "fix" an invalid schema.

It applies local recovery after diagnostics:

- incompatible `const` remains a reported schema error;
- incompatible `const` does not remain an active semantic constraint;
- incompatible `enum` members remain reported schema errors;
- incompatible `enum` members do not remain active semantic constraints.

This is standard compiler recovery, not hidden mutation.

The compiler still returns:

- `Valid = false`;
- full primary diagnostics;
- no derivative failures caused only by replaying already-invalid constraints into downstream schema engines.

## 7. Why This Is Cleaner Than Ad-Hoc Sanitizing

An ad-hoc local fix inside one downstream builder would couple the recovery logic to that builder.

Examples of what is not desirable:

- expression-context builder silently dropping invalid `const`;
- another future builder repeating the same rule differently;
- compatibility checks being duplicated outside the compiler core.

The cleaner rule is:

- the compiler owns recovery from invalid source facets;
- downstream builders consume only the semantic projection;
- no builder re-decides whether a source constraint is usable.

## 8. Derived Schema Rule

Any builder that serializes compiler data into a secondary schema language must consume the semantic model only.

Examples:

- JMESPath expression context schema;
- future query/get derived read schemas;
- any validation/reference helper that emits JSON-schema-like constraints.

This keeps the contract stable:

- primary schema diagnostics come from the shared compiler;
- downstream schema engines do not invent duplicate diagnostics for the same root defect.

## 9. Migration Path

### 9.1. Immediate Practical Step

Without redesigning the entire compiler model in one change, the current code can introduce an explicit compiler-owned projection for derived schemas.

That projection should:

- reuse the compiler's type-compatibility rules;
- drop invalid `const`;
- drop invalid `enum` members;
- recurse into nested `items`;
- be used by expression-context building.

This is an intermediate step toward the full raw/semantic split.

### 9.2. Ideal End State

The final architecture should stop treating this as a special projection for one builder.

Instead:

- raw/normalized schema becomes a distinct internal representation;
- semantic/usable schema becomes the canonical compiled output for capabilities and derived builders;
- any diagnostic-oriented source detail that is not semantically usable stays in the raw layer only.

## 10. Non-Goals

This document does not require:

- preserving invalid constraints for runtime execution;
- exposing raw invalid constraints in public command capabilities;
- changing the public schema diagnostics contract;
- redesigning tolerant `help` behavior in the same step.

## 11. Recommendation

For the long-term compiler architecture:

- adopt the explicit raw/semantic split;
- treat derivative-schema safety as a compiler invariant, not a builder-local workaround;
- keep all downstream builders on the semantic side only.

For the current migration wave:

- implement the compiler-owned projection for derived schemas now;
- use it as a stepping stone toward the full split, not as the final model.
