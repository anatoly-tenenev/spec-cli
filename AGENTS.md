# AGENTS.md

## Constitution

`spec-cli` is a machine-first CLI utility written in Go for working with spec documents.

The statements below decide cases the rules further down do not cover. If a rule contradicts a statement here, the rule is wrong. Each item names what would overturn it; an item that cannot be overturned is a slogan, not a decision.

1. **The consumer is an agent, not a person at a terminal.**
   Machine format by default, no interactive prompts, responses that describe themselves. Contract stability outweighs readability by eye.
   *Overturned by:* a primary workflow with a human at the terminal.

2. **Markdown stays the source; there is no second data layer.**
   No index, cache, or derived state that can desynchronise. Documents stay readable and editable without the tool, and written output stays diff-friendly.
   *Overturned by:* accepting a store that the tool owns.

3. **The schema is the single source of truth about structure.**
   Entity types, paths, validation, help, and projections all derive from the schema. When the schema is unavailable, refuse rather than guess.
   *Overturned by:* supporting documents without a schema.

4. **Editing goes through an agent; hand edits stay repairable.**
   Direct edits are tolerated, so the workspace can be non-conforming at any moment. Bringing a document back into conformance is a first-class operation, and every repair path starts by reading a non-conforming document.
   *Overturned by:* hand editing becoming forbidden rather than tolerated.

5. **Storage layout is an implementation detail; the model is logical.**
   An entity is `meta`, `refs`, and `content.sections`, not a file. Filesystem paths do not appear in responses, and the layout can change without breaking contracts.
   *Overturned by:* promoting paths to part of the public model.

6. **Responses are machine-stable and deterministic.**
   The same input yields the same bytes. Traversal and sort order are fixed; map iteration order must never reach the output. The shape of a response is a contract.
   *Overturned by:* nothing currently foreseen.

7. **Strictness on write, usability on read.**
   `add`, `update`, and `delete` must not leave a non-conforming workspace. Read commands return what is on disk; `validate` judges conformance. Reads must keep working on a non-conforming workspace, because repair depends on them.
   *Overturned by:* hand editing becoming forbidden, which removes the need to repair.

8. **The tool never invents data.**
   When there is no unambiguous answer, say so and name the document and the field. Never pick one candidate silently, and never report silent absence in place of ambiguity. A loud refusal beats a quiet guess.
   *Overturned by:* nothing currently foreseen.

9. **Breaking changes are allowed; silent ones are not.**
   The project is early, so backward compatibility does not bind. Every behaviour change must surface as a failing test, an error, or an updated fixture, and never as the same exit code with different behaviour.
   *Overturned by:* the first release that promises stability.

## Communication Rules
- Communicate in the language the dialogue started in unless the user explicitly asks to switch languages.

## Technology and Baseline
- Language: Go (`go 1.24+`).
- Distribution format: a single `spec-cli` binary.
- Prefer the standard library.
- Add external dependencies only when they provide clear value and the reason is documented in the PR/commit.

## Architectural Rules
- Follow the `Hexagonal + Command Bus` style.
- The CLI layer (`internal/cli`) must only parse arguments and route commands.
- Use-case logic belongs in `internal/application/commands/<command>`.
- Domain types/errors belong in `internal/domain`.
- Keep `json/ndjson` output in `internal/output`.
- Do not mix response formatting and business logic in one place.

## Contract Invariants
- Supported formats: `--format json` and `--format ndjson`.
- Every response/record must contain `result_state`.
- Errors must include: `error.code`, `error.message`, `error.exit_code`.
- For `ndjson`, use `record_type` (`result|item|issue|summary|error`) according to the command scenario.
- For entities, return `revision` as an opaque token.

## Error Codes and Exit Codes
- Use the single shared set of error codes from the domain layer.
- Base exit code mapping:
  - `0` success
  - `1` domain error
  - `2` invalid args/query
  - `3` read/write error
  - `4` schema error
  - `5` internal error

## Repository Structure
- Entry point: `cmd/spec-cli/main.go`
- Main layers: `internal/cli`, `internal/application`, `internal/domain`, `internal/output`, `internal/contracts`
- Prototype specification: `doc/001-base/SPEC_UTILITY_CLI_PROTOTYPE.md`
- Local working specification: `spec/SPEC_STANDARD_RU_REVISED_V3.md` (the `spec/` directory is in `.gitignore`)
- Documentation index (entry point): `doc/README.md`
- Codebase map: the package comments themselves; `go doc <package>` prints one, `go list ./internal/... | xargs -n1 go doc` prints all of them.

## Development Commands
- Formatting: `make fmt`
- Static checks: `make vet`
- Package comments: `make lint`
- Tests: `make test`
- Build: `make build`

## Git Command Restrictions
- It is forbidden to run any `git` command that changes repository state (for example: `git add`, `git commit`, `git reset`, `git restore`, `git checkout` with write effects, `git clean`, `git rebase`, `git merge`, `git cherry-pick`).
- Only non-mutating `git` commands are allowed (for example: `git status`, `git diff`, `git log`, `git show`, `git branch --show-current`).

## Expectations for Changes
- Preserve the machine-stable response contract.
- When adding/changing a command, update contract tests and snapshot/golden files.
- Integration tests must remain black-box contract tests: verify only public CLI behavior (`args` -> `stdout/stderr` -> `exit_code`, and for mutating commands also `workspace.out`).
- Integration tests must not assume anything about the internal implementation of the utility (layers, engine reuse, order of internal calls).
- Every meaningful contract behavior must have a direct integration case; indirect coverage via another scenario is not sufficient.
- When adding/changing documentation in `doc/`, update `doc/README.md` in the same change.
- When a package's role changes, update its package comment in the same change.
- Do not add interactive prompts by default.
- Do not expose internal entity filesystem paths in API responses.

## Code Structure and Shared Code

### Directory Structure
- At most one `internal/` in a package path, not counting the module's top-level `internal/`. Every `internal/` is a wall behind which code cannot be reused; a second wall guarantees a copy instead of reuse.
- Name a package after its subject, not after its role in the dependency graph: `common`, `support`, `util` and `helpers` are forbidden.
- A `.go` file is limited to 600 lines. When it exceeds the limit, split it into another file in the same package, named after what it holds. Reach for a subpackage only when the split part has more than one importer: in Go a file split costs nothing, a package split adds a visibility wall.

### Shared Code
- A copy is a deferred divergence. When copying code, immediately decide one of two things: extract it into a shared place, or record in a comment why the copies must differ. There is no third state - "identical for now, we will sort it out later" always resolves into divergence.
- One question, one place that answers it. Structural sameness is not grounds for extraction: before extracting, write down which question the code answers and confirm it is the same question. If the copies have a reason to change apart, keep them apart and record the reason.
- Resolve divergences between copies before extracting, not after, and confirm the decision by running the code, not by reasoning about it.
- Extract at the layer of the question, not at the layer of the consumer. If a shared place serves two consumers out of six, the extraction was made too high.
- When decomposition and deduplication conflict, deduplication wins: one shared package beats two private ones answering the same question.
- Where shared code goes: needed by two commands - `internal/application/commands/internal/<role>`; needed by commands together with `readmodel` or `schema` - a package directly under `internal/application/`; needed by `internal/cli` as well - one level above. A shared place must already exist and be the obvious destination, rather than being invented each time. The package comment answers why this is one thing and not two.
- Measure duplication mechanically (`make dupl`), not by eye: copies live in different subtrees and each looks reasonable on its own.

## Command Implementation Standard (default)
- The entrypoint is fixed: `internal/application/commands/<command>/handler.go`. It must remain a thin orchestration layer: parse options -> load inputs/schema -> run use case -> build the `json/ndjson` response.
- A command keeps its details in subpackages under its own `internal/`, created as the command needs them: `options` - argument parsing, `model` - command types, `workspace` - reading the workspace, `engine` - the use case. Commands use the same names for the same roles, so the same thing is found in the same place.
- The same subpackage name under two commands means the same role, not shared code. Before writing logic into one, check whether its namesake under another command already has it: if so, it belongs in a shared package, not in a second copy.
- Logic shared by two or more commands belongs in `internal/application/commands/internal/<role>`. Logic shared with `readmodel` or `schema` belongs in a package directly under `internal/application/` (see `entitydoc`, `values`). Copying is allowed only with a written reason why the copies must differ.
- Do not change business logic, error codes, or issue codes without an explicit task to change behavior.
- After any command changes, run at least `make vet` and `make test`.

## Documentation Rules
- Before searching for project details, check `doc/README.md` first.
- If a new document is added, the agent must add it to the index with a short description of its purpose.
- Use numbering like `NNN-*` only for stage/milestone documentation directories.
- Put generally applicable documentation (indexes, maps, shared conventions) in the root of `doc/` without a numbered prefix.
- Every package under `internal/` must carry a package comment stating why the package exists and which boundary it holds, not restating its name.
- The package comment moves with the package: it is written in the same change that creates the package and disappears with it.
- The map of the codebase is the package comments read through `go doc`; there is no separate index file to keep in sync. `make lint` fails on a package without a comment.
