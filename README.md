# spec-cli

CLI for reading, validating, and modifying structured Markdown specifications.

`spec-cli` works with Markdown documents described by a schema based on [Spec Schema](https://spec-schema.org/).

It does not define an SDD workflow. An SDD framework or AI agent decides what to do. `spec-cli` provides commands to work with the specification.

> Status: early / experimental.

## Why

Markdown works well for specifications. It is readable, works with Git, and does not require a separate storage format.

Editing becomes more difficult as the specification becomes more structured.

A structured specification has rules:

* which entity types exist;
* which fields they have;
* how entities reference each other;
* where documents are stored;
* which sections documents can contain.

Reading a large specification also has its own problem. An agent usually does not need all documents and all fields for every task. It needs a specific part of the specification.

`spec-cli` provides commands to:

* inspect the available model and commands;
* query only the required entities and fields;
* validate the specification;
* add, update, and delete entities.

The source stays as normal Markdown.

There is no separate database, no second JSON representation, and no synchronization between documentation and machine-readable data.

## Reading the specification

An agent does not have to read the whole workspace.

It can first inspect the available structure:

```bash
spec-cli help
```

`help` describes the CLI, the specification model, and schema-derived values available for the current workspace.

For more details about a command:

```bash
spec-cli help query
```

Then `query` can return only the required part of the specification:

```bash
spec-cli query \
  --type feature \
  --where "meta.status == 'active'" \
  --select id \
  --select slug \
  --select meta.status
```

`get` reads a specific entity when its ID is already known.

For GraphQL-based reads, `spec-cli` can expose the generated GraphQL schema:

```bash
spec-cli graphql-help --schema-only
```

and execute read-only GraphQL queries:

```bash
spec-cli graphql-query --file query.graphql
```

This allows an agent to request the entities, fields, and relationships needed for the current task instead of loading the entire specification.

See [GraphQL as an API for a Specification](https://spec-schema.org/blog/graphql-as-api-for-spec/) for more details about this approach.

## Modifying the specification

`spec-cli` also provides commands for structured changes:

```text
add
update
delete
```

These commands use the schema when modifying Markdown documents.

A tool does not need to implement its own logic for document paths, fields, references, and document structure.

## Validation

Use `validate` to check the workspace against the schema:

```bash
spec-cli validate
```

Use `schema check` to validate the schema itself:

```bash
spec-cli schema check
```

## SDD frameworks

`spec-cli` is not an SDD framework.

An SDD framework defines its own workflow. For example:

```text
requirements
→ design
→ tasks
→ implementation
```

Another framework can use a different workflow.

When the framework needs to read or change the specification, it can use `spec-cli`.

```text
SDD framework / AI agent
          |
          v
       spec-cli
          |
          v
   Markdown specification
```

The framework decides what to do.

`spec-cli` reads, validates, and modifies the specification.

It can also be used directly by AI agents, scripts, CI, or humans without an SDD framework.

## A useful analogy

`spec-cli` is roughly like SQL for a specification.

```text
help                    → inspect the available model
query / get             → read data
graphql-help/query      → structured GraphQL reads
add                     → create
update                  → modify
delete                  → remove
validate                → check
```

The analogy is not exact. The source of truth remains Markdown files.

## What it works with

A workspace contains:

* Markdown documents with YAML frontmatter;
* a schema that defines entity types, fields, references, paths, and allowed sections.

For example:

```yaml
version: "0.0.7"

entity:
  service:
    idPrefix: SVC
    pathTemplate: "services/${slug}/index.md"
    content:
      sections:
        description: { title: Description }

  feature:
    idPrefix: FEAT
    pathTemplate: "${refs.service.dirPath}/features/${meta.status}/${slug}.md"

    meta:
      fields:
        status:
          schema:
            type: string
            enum: [draft, active, deprecated]

        service:
          schema:
            type: entityRef
            refType: service

    content:
      sections:
        description: { title: Description }
```

A workspace using this schema can look like this:

```text
specs/
└── services/
    └── ledger-api/
        ├── index.md
        └── features/
            ├── active/
            │   └── retry-window.md
            └── deprecated/
                └── legacy-retry.md
```

A document is still normal Markdown:

```markdown
---
type: feature
id: FEAT-7
slug: retry-window
createdDate: 2026-03-10
updatedDate: 2026-03-12
status: active
service: SVC-1
---

## Description {#description}

Retry failed requests with capped exponential backoff and idempotency keys.
```

## Commands

### Read and inspect

* `help` — inspect the CLI and specification model
* `query` — read and filter multiple entities
* `get` — read a specific entity
* `graphql-help` — inspect the GraphQL model
* `graphql-query` — execute read-only GraphQL queries

### Validate

* `schema check` — check the schema
* `validate` — validate the workspace

### Modify

* `add` — add an entity
* `update` — update an entity
* `delete` — delete an entity

### Other

* `version` — print the version

Run:

```bash
spec-cli help <command>
```

for detailed command usage.

## Installation

See [INSTALL.md](INSTALL.md).

## Related

* [Spec Schema](https://spec-schema.org/) — schema format used by `spec-cli`
* [GraphQL as an API for a Specification](https://spec-schema.org/blog/graphql-as-api-for-spec/) — why structured queries are useful for agents
* [`rust-cc-spec`](https://github.com/anatoly-tenenev/rust-cc-spec) — example schema and workspace
