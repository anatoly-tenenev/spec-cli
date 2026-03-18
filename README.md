# spec / spec-cli

**A CLI contract over Markdown specs for humans, CI, and AI agents.**

`spec-cli` is an early, actively developed baseline for working with a spec workspace:
Markdown documents with structured metadata described by a schema based on the [Spec Schema](https://spec-schema.org/) standard.

The idea behind the project is simple:
engineering knowledge should remain normal documents for humans,
while still being available to automation as an explicit and reliable contract.

`spec-cli` does exactly that:
it projects a spec workspace into a CLI interface for reading, validation, and safe changes.

> **Status:** early / experimental.  
> The project is under active development: the baseline is already useful, but the contract, UX, and capability boundaries are still taking shape.

## Why

Regular engineering documentation works well for humans,
but poorly as an interface for automation.

When an agent, CI, or an internal tool needs to rely on such documents, one of two things usually happens:

- too much Markdown has to be read in full;
- a separate JSON / index / DB layer appears next to the documents and starts drifting away from them.

`spec-cli` exists to prevent that.

It allows the same document corpus to be used in several modes at once:

- as documentation for humans;
- as a validatable contract layer for CI;
- as an addressable data layer for tooling and AI agents.

## What It Is

Input:

- Markdown documents with `YAML frontmatter`;
- a schema based on the [Spec Schema](https://spec-schema.org/) standard that defines entity types, fields, links, and allowed sections.

Output:

- `query` for bulk reads;
- `get` for targeted reads;
- `validate` for workspace checks;
- `add`, `update`, `delete` for safe writes;
- `help` as a discovery interface for tooling and agents.

In other words, the project does not replace documents with a machine-only format.
It makes Markdown specs operationally accessible to automation.

## Why This Relates to Spec-Driven Development

`spec-cli` fits naturally into workflows where the specification stops being a "document on a shelf" and becomes a working development artifact.

In short:

- the specification remains the source of engineering intent;
- automation gets an explicit interface to it;
- the team does not maintain narrative documents separately from a machine-only layer.

In that sense, `spec-cli` can be seen as an infrastructure layer for spec-driven development:
not as a code generator from specs, but as a way to make the spec corpus a stable interface for humans, CI, and agents.

## Core Model

1. Each spec document remains a normal Markdown file.
2. The schema defines the structural rules.
3. `spec-cli` projects the schema and the documents into a command-facing interface.
4. Automation reads and changes only the data layer it actually needs.

The point is not to have "documents for humans" and separate "data for machines."
The point is for the same document to work in both directions.

## What It Looks Like on Disk

A small workspace example:

```text
specs/
  domains/
    payments/
      domain.md
      services/
        ledger-api/
          service.md
          features/
            retry-window.md
          adr/
            2026-03-10-idempotency-key.md
```

The meaning is straightforward:

- `domain.md` describes the `payments` domain;
- `service.md` describes the `ledger-api` service;
- `features/` contains individual service features;
- `adr/` contains architectural decisions for that service.

The structure is already readable from the path itself:
for a human, it helps navigate the corpus;
for a tool, it helps understand an entity's context and its place in the workspace.

## What a Single Document Looks Like

For example, an ADR might look like this:

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

So this is not a special binary format and not a separate database.
It is a normal Markdown document with:

- structured fields at the top;
- regular text below;
- sections and metadata that can be read by both humans and tools.

## Example

Suppose an agent needs to quickly understand which ADRs have already been accepted, without pulling the full text of every document into context.

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

That is the point of the project:

- a human keeps reading the full document;
- an agent gets only the data layer it needs;
- CI and tooling work with the same corpus;
- there is no need for a second source of truth next to Markdown.

## Baseline

The baseline is intentionally narrow for now:

- `version`
- `help`
- `validate`
- `query`
- `get`
- `add`
- `update`
- `delete`

This is the minimum useful contract for real workflows:

- read many;
- read one;
- validate;
- write safely.

## Why It Matters

AI agents did not create this problem, but they made it much more visible.

They do not just need text,
but a narrow, explicit, and predictable interface to engineering knowledge.

If such an interface does not exist, a shadow layer almost inevitably appears:
parsers, indexes, ad hoc JSON projections, and local rules that begin living separately from the documents.

`spec-cli` is an attempt to stop before that fork
and give a spec workspace a minimal shared contract.

## In Short

`spec-cli` is an attempt to make Markdown specs not only readable, but operationally usable.

- documents remain Markdown;
- the schema defines the rules;
- the CLI provides the interface;
- humans, CI, and AI agents work with the same source of truth.
