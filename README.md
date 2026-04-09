# spec-cli

**CLI for validating, querying, and updating Markdown specs.**

`spec-cli` works with a spec workspace: Markdown documents with structured metadata described by a schema based on the [Spec Schema](https://spec-schema.org/).

> **Status:** early / experimental.

## Why

`spec-cli` is for working with Markdown like code.
The right way to use it is through an AI agent working with the documentation.

No separate database, no second JSON layer, and no manual sync between "docs for people" and "data for machines".

## What it does

Input:

- Markdown documents with YAML frontmatter
- a schema that defines entity types, fields, links, and allowed sections

Output:

- `validate` for workspace checks
- `query` for bulk reads
- `get` for targeted reads
- `add`, `update`, `delete` for controlled writes
- `help` for command discovery

The source stays as normal Markdown. This is not a separate database and not a machine-only format.

## Installation

See [INSTALL.md](./INSTALL.md).

## Example schema

```yaml
version: "0.0.4"
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
            refTypes: [service]
    content:
      sections:
        description: { title: Description }
```

## Example workspace

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

## Example document

```md
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

## Example query

```bash
spec-cli query \
  --type feature \
  --where "meta.status == 'active'" \
  --select meta \
  --select refs \
  --select content.sections
```

## Related example

See [`rust-cc-spec`](https://github.com/anatoly-tenenev/rust-cc-spec) for an example schema and workspace.

## Baseline

The current list of commands is:

* `version`
* `help`
* `validate`
* `query`
* `get`
* `add`
* `update`
* `delete`

It will increase in future releases. 
