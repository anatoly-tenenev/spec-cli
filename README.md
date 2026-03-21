# spec-cli

**CLI for validating, querying, and updating Markdown specs.**

`spec-cli` works with a spec workspace: Markdown documents with structured metadata described by a schema based on the [Spec Schema](https://spec-schema.org/).

> **Status:** early / experimental. Not for production now.

## Why

Engineering documentation is easy for people to read, but hard to use from automation.

Usually one of two things happens:

- tools end up reading too much raw Markdown;
- a separate technology (JSON/index/DB) appears next to the documents and starts drifting away from them.

`spec-cli` is a tool to keep the documents as the source of truth while still making them usable from tooling.

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

## Example workspace

```text
specs/
  domains/
    payments/domain.md
  services/
    ledger-api/service.md
    features/retry-window.md
    adr/2026-03-10-idempotency-key.md
````

## Example document

```md
---
type: adr
id: ADR-12
slug: retry-policy
created_date: 2026-03-10
updated_date: 2026-03-10
status: accepted
decision_makers:
  - payments-arch
  - sre-lead
---

## Context {#context}
During partial degradations, clients were generating too many repeated requests.

## Decision {#decision}
Use capped exponential backoff, jitter, and mandatory idempotency.

## Consequences {#consequences}
Load during incidents goes down, but some clients get slightly slower recovery.
```

## Example query

```bash
spec-cli query \
  --type adr \
  --where-json '{"field":"meta.status","op":"eq","value":"accepted"}' \
  --select id \
  --select slug \
  --select created_date \
  --select content.sections.decision \
  --sort created_date:desc \
  --format json
```

The point is simple: people keep reading the full documents, while tooling reads only the fields and sections it needs.

## Where this fits

This fits workflows where specs are real working artifacts, not just documents that sit on the side.

Instead of maintaining separate human-readable and agent-readable layers, the goal is to keep a single set of Markdown documents and still supports validation, queries, and controlled writes.

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
