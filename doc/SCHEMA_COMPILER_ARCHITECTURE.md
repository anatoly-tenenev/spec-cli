# Unified Schema Compiler Architecture

## 1. Purpose

This document defines the target architecture for schema validation and schema-driven command execution in `spec-cli`.

The current implementation lets each schema-aware command load and validate the schema independently. That model duplicates YAML parsing, duplicate-key checks, top-level validation, and command-specific projections of the same schema semantics.

The target architecture replaces per-command schema loaders with one shared compile step:

1. Read and parse schema source once.
2. Compile it into one canonical semantic model.
3. Build command capabilities from that canonical model.
4. Execute commands only against those capabilities.

## 2. Goals

The architecture must provide:

- one canonical interpretation of schema semantics for all strict schema-aware commands;
- one global rule for schema validity;
- one place where schema diagnostics are produced;
- one mental model for command execution: `parse args -> compile schema -> build/use capability -> run command`;
- tolerant degraded behavior only where explicitly allowed (`help`);
- deterministic diagnostics and deterministic capability build order.

## 3. Non-Goals

This architecture does not attempt to:

- preserve partial-schema execution for strict commands;
- keep command-local schema loaders as first-class long-term abstractions;
- support `ndjson` for schema diagnostics in this iteration;
- introduce cross-process schema caching in the first implementation wave.

## 4. Global Validity Model

### 4.1. Definition

The schema is **globally valid** when the shared compile step completes and produces no diagnostics with level `error`.

Warnings do not invalidate the schema.

### 4.2. Operational Rule

Strict schema-aware commands must execute a shared `schema.Compile(...)` step before building any command capability.

If compile produces one or more schema errors:

- `query`, `get`, `add`, `update`, and `delete` must stop before command execution;
- `validate` must stop before workspace/entity validation;
- the command must return its normal error response plus a top-level `schema` block with compile diagnostics.

`help` remains explicitly tolerant and may continue to expose degraded schema-derived output as a separate behavior.

### 4.3. Important Consequence

Schema validity is not evaluated relative to:

- the command kind;
- the active type set;
- the subset of schema features used by the current invocation.

If any part of the schema is invalid, the full schema is invalid for every strict schema-aware command.

## 5. Top-Level Execution Model

The target execution model for strict schema-aware commands is:

1. Parse CLI arguments.
2. For mutating commands, acquire workspace lock.
3. Run `schema.Compile(...)`.
4. Build the command capability from the compiled schema.
5. Execute command-specific logic.
6. Return the command result or command error.
7. Always include the top-level `schema` block once compile has been attempted.

Notes:

- For `add`, `update`, and `delete`, compile runs after workspace lock acquisition by explicit decision.
- Compile results are cached only within the current process / command run.

## 6. Layering

The architecture is split into three layers.

### 6.1. Schema Source Layer

Responsibility:

- read schema file;
- parse YAML/JSON;
- preserve path-aware node structure;
- detect duplicate keys;
- expose raw document tree for the compiler.

This layer must not contain command concepts such as selectors, write paths, or validation coverage.

### 6.2. Canonical Compiler Layer

Responsibility:

- normalize the parsed document into one canonical semantic schema model;
- validate schema semantics;
- compile expressions and path-template fragments;
- emit deterministic diagnostics;
- return compiled schema plus diagnostics.

This is the single source of truth for schema meaning.

### 6.3. Capability Layer

Responsibility:

- derive command-specific capabilities from the compiled schema;
- expose read/write/validation/reference/help projections;
- keep command use cases free from raw schema parsing.

Capabilities are adapters over compiled semantics, not alternate schema parsers.

## 7. Canonical Schema Model

The shared compiler must return a canonical schema model that describes schema semantics without embedding command concerns.

Illustrative shape:

```go
type CompiledSchema struct {
    Version     string
    Description string
    Entities    map[string]EntityType
}

type EntityType struct {
    Name         string
    IDPrefix     string
    PathTemplate PathTemplate
    MetaFields   map[string]MetaField
    Sections     map[string]Section
}

type MetaField struct {
    Name        string
    Value       ValueSpec
    Required    Requirement
    Description string
}

type Section struct {
    Name        string
    Titles      []string
    Required    Requirement
    Description string
}

type Requirement struct {
    Always bool
    Expr   *CompiledExpression
}

type ValueSpec struct {
    Kind         ValueKind
    Format       string
    Enum         []Literal
    Const        *Literal
    Ref          *RefSpec
    Items        *ValueSpec
    UniqueItems  bool
    MinItems     *int
    MaxItems     *int
}

type RefSpec struct {
    Cardinality  RefCardinality
    AllowedTypes []string
}
```

The exact type names may differ, but the architectural rule is fixed:

- the canonical model contains schema semantics only;
- it does not contain read selectors, write path allow-lists, delete reference slots, or command response formatting data.

## 8. Compile Pipeline

The shared compile step must run in deterministic phases.

### Phase 1. Load Source

Inputs:

- schema path;
- display/source path used in diagnostics.

Outputs:

- raw bytes;
- YAML/JSON root node;
- initial file/parse diagnostics.

Validation examples:

- file not found;
- empty file;
- YAML/JSON parse error;
- duplicate keys.

### Phase 2. Normalize Document

Convert source nodes into one canonical schema representation.

Examples:

- `pathTemplate: string | []string | object{cases}` -> one canonical `PathTemplate`;
- `required: bool | "${expr}"` -> one canonical `Requirement`;
- scalar `entityRef` and `array.items.type=entityRef` -> one canonical ref model;
- normalize and sort deterministic collections such as `refTypes`.

### Phase 3. Semantic Validation

Validate the normalized schema as one language.

Examples:

- unsupported keys in closed-world objects;
- duplicate `idPrefix` values across entity types;
- invalid `idPrefix` format;
- invalid `pathTemplate` structure or interpolation;
- expression compile/static errors;
- incompatible `type` + `const` / `enum`;
- invalid `items`, `minItems`, `maxItems`, `uniqueItems`;
- unknown entity types in `refTypes`;
- invalid built-in/reserved names.

The compiler must gather as many diagnostics as possible with local recovery.

### Phase 4. Freeze Result

Return:

- canonical compiled schema;
- full deterministic diagnostics list;
- summary flags such as `valid`, `error_count`, `warning_count`.

Capability builders may run only when the caller chooses to continue and, for strict commands, only when there are no schema errors.

## 9. Diagnostics Model

### 9.1. Producer

Schema diagnostics are produced only by the shared compiler.

Command layers may wrap compile failure into a command error response, but they must not reinterpret schema meaning locally.

### 9.2. Severity

- `error` blocks strict schema-aware commands.
- `warning` is returned to the caller but does not block command execution.

### 9.3. Recovery Policy

The compiler should continue after local errors whenever the remaining subtree can still be interpreted safely enough to produce more diagnostics.

The goal is maximum useful diagnostics, not fail-fast behavior.

## 10. Capability Builders

Capabilities are built from the compiled schema, not from source YAML.

### 10.1. Read Capability

Consumers:

- `query`
- `get`
- tolerant `help` schema projection if desired

Responsibilities:

- selector namespace;
- filter/sort namespace;
- scalar/array ref exposure rules;
- read-side field kind metadata.

### 10.2. Write Capability

Consumers:

- `add`
- `update`

Responsibilities:

- allowed write paths;
- allowed unset paths;
- allowed set-file paths;
- type-aware metadata/section/ref constraints;
- canonical path calculation through compiled path-template semantics.

### 10.3. Validation Capability

Consumers:

- `validate`
- potentially post-write entity validation in mutating commands

Responsibilities:

- required-field rules;
- required-section rules;
- enum/const checks;
- scalar/array ref validation rules;
- path correctness rules;
- deterministic instance-validation plan.

### 10.4. Reference Capability

Consumers:

- `delete`

Responsibilities:

- inbound reference slots by entity type;
- scalar vs array cardinality;
- reverse-reference integrity checks.

### 10.5. Help Projection

Consumers:

- `help`

Responsibilities:

- schema-derived option catalogs and projection output;
- degraded mode remains explicitly separate from strict command behavior.

## 11. Command Integration

### 11.1. `schema check`

`schema check` becomes the explicit public command for compile diagnostics.

Behavior:

- run `schema.Compile(...)`;
- return the same contract shape as `validate`, but only for schema diagnostics;
- do not inspect workspace entities.

### 11.2. `validate`

Flow:

1. run `schema.Compile(...)`;
2. return top-level `schema` block;
3. if schema has errors, stop;
4. otherwise build validation capability;
5. run workspace/entity validation;
6. return instance/workspace issues in `issues`.

Important rule:

- schema diagnostics live in top-level `schema`;
- `issues` contain only workspace/entity validation diagnostics.

### 11.3. `query` / `get`

Flow:

1. run `schema.Compile(...)`;
2. return top-level `schema` block;
3. if schema has errors, stop;
4. build read capability;
5. validate selectors / where / sort against read capability;
6. execute command.

### 11.4. `add` / `update`

Flow:

1. parse args;
2. acquire workspace lock;
3. run `schema.Compile(...)`;
4. return top-level `schema` block;
5. if schema has errors, stop;
6. build write capability;
7. validate write operations;
8. execute write pipeline;
9. run post-write validation capability if required.

### 11.5. `delete`

Flow:

1. parse args;
2. acquire workspace lock;
3. run `schema.Compile(...)`;
4. return top-level `schema` block;
5. if schema has errors, stop;
6. build reference capability;
7. execute reverse-reference checks and deletion pipeline.

### 11.6. `help`

`help` stays tolerant by explicit design choice.

It may continue to:

- project schema-derived help output when the schema compiles;
- expose degraded schema-unavailable output when the schema is not usable.

`help` must not become the architectural exception that leaks back into strict commands.

## 12. Response Contract Impact

All strict schema-aware commands gain one common top-level block:

```json
{
  "result_state": "ok",
  "schema": {
    "valid": true,
    "summary": {
      "errors": 0,
      "warnings": 1
    },
    "issues": [
      {
        "level": "warning",
        "class": "SchemaWarning",
        "code": "schema.example",
        "message": "..."
      }
    ]
  }
}
```

Contract rules:

- `schema` is included in success responses for strict commands after compile runs;
- `schema` is included in error responses if compile ran, even when the final error is not schema-related;
- on compile failure, commands return their normal error response plus the top-level `schema` block;
- `schema check` and `validate` use the same schema block model;
- `ndjson` support is explicitly out of scope for this architecture iteration.

## 13. Caching

The compiler result may be cached only within the current process / command run.

This cache exists to avoid recompiling the same schema repeatedly while one command builds multiple capabilities.

Out of scope for the first implementation:

- cross-process cache;
- on-disk cache invalidation.

## 14. Package Direction

The target package structure should reflect the architecture directly.

Illustrative layout:

```text
internal/application/schema/source
internal/application/schema/compile
internal/application/schema/model
internal/application/schema/diagnostics
internal/application/schema/capabilities/read
internal/application/schema/capabilities/write
internal/application/schema/capabilities/validate
internal/application/schema/capabilities/references
```

Design rules:

- source parsing stays below the compiler;
- command handlers depend on capabilities, not on YAML parsing;
- capability packages must not read files or parse YAML themselves;
- the compiler must not return command-specific payload structs.

## 15. Testing Strategy

The architecture should be tested at three layers.

### 15.1. Compiler Tests

Verify:

- duplicate keys;
- normalization behavior;
- semantic validation;
- deterministic issue ordering;
- warning vs error behavior;
- compiled canonical model shape.

### 15.2. Capability Builder Tests

Verify:

- read selector projection;
- write path projection;
- reference slot projection;
- validation rules projection.

### 15.3. Integration Tests

Verify public CLI behavior only:

- `schema check`;
- strict commands blocked by invalid schema;
- strict commands returning schema warnings on success;
- `validate` separating schema diagnostics from workspace issues;
- `help` degraded behavior unchanged where intended.

## 16. Why This Model Fits the Mental Model Better

The architecture intentionally reduces the system to three questions:

1. Can the schema source be read and compiled?
2. If yes, what is the canonical meaning of the schema?
3. Given that meaning, what capability does this command need?

That is simpler than the current mental model where each command:

- parses overlapping parts of schema independently;
- chooses its own local validity rules;
- maintains its own schema projection and diagnostics logic.

## 17. Final Architectural Rule

The shared compiler owns schema truth.

Capability builders adapt that truth for command use cases.

Commands do not parse schema semantics on their own.
