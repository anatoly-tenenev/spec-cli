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
- Codebase index (agent map): `doc/CODEBASE_INDEX.md`

## Development Commands
- Formatting: `make fmt`
- Static checks: `make vet`
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
- When changing codebase structure/roles, keep `doc/CODEBASE_INDEX.md` up to date in the same change.
- Do not add interactive prompts by default.
- Do not expose internal entity filesystem paths in API responses.

## Code Structure Rules (entrypoint-first, strict)
- For any application-logic directory, the invariant is: one directory = one abstraction level.
- Only entrypoint files for that directory level are allowed in the directory root.
- An entry point contains orchestration and the package's public API; implementation details must not live there.
- Any detailed implementation must be moved into subpackages with domain-specific names.
- Detail files next to the entrypoint in the directory root are forbidden.
- If a directory root has multiple `.go` files, each of them must be an entrypoint for a separate top-level role; otherwise move the file into a subpackage.
- For command use cases, the entrypoint is fixed: `internal/application/commands/<command>/handler.go`.
- For complex commands, place details under `internal/application/commands/<command>/internal/...` using domain-specific package names (`options`, `schema`, `workspace`, `engine`, etc.).
- Do not use `common`, `support`, `util`, or `helpers` as universal packages; name a package after its subject, not after its role in the dependency graph.
- Create a new directory with non-trivial logic only together with a clear entrypoint file and an explicit directory role.
- `.go` files are limited to `600` lines; when the limit is exceeded, split the file while preserving the entrypoint-first rules.
- If a `.go` file exceeds `600` lines, move detailed logic into subpackages under `internal/<domain_role>/...`; splitting into several detail files in the same root directory is forbidden.
- The intermediate step "first split the file in the root, then move it into subpackages" is not allowed: move directly to the target structure with the entrypoint in the root and details in subpackages.
- Review criterion: within 5 seconds it must be clear which file is the directory entrypoint and where the details live.
- Finishing a task without an entrypoint-first self-check is forbidden: before the final response, check every modified directory and ensure there are no detail files next to an entrypoint in its root.
- If a modified directory still contains more than one `.go` file, the final message must explicitly list their roles (the entrypoint of each top-level role) and confirm that details were moved to subpackages.
- Any structural refactoring must be done directly in the target structure; temporary violations of entrypoint-first within a commit/change are not allowed.

## Command Implementation Standard (default)
- `handler.go` must remain a thin orchestration layer: parse options -> load inputs/schema -> run use case -> build the `json/ndjson` response.
- For `validate`, use the current structure as the baseline:
  - `internal/model` - internal command types.
  - `internal/options` - command option parsing and path normalization.
  - `internal/workspace` - candidate scan and frontmatter/content parsing.
  - `internal/engine` - main validation pipeline and issue aggregation.
  - `internal/ruletypes` - matching a value against a declared rule type or enum.
  - `internal/duplicates` - finding duplicated keys in an index.
- Before creating a subpackage, check whether a package with that name already exists under another command. Identical names across commands are a signal to look for shared logic, not a template to copy.
- Logic shared by two or more commands belongs in `internal/application/commands/internal/<role>`. Logic shared with `readmodel` or `schema` belongs in a package directly under `internal/application/` (see `entitydoc`, `values`). Copying is allowed only with a written reason why the copies must differ.
- Do not change business logic, error codes, or issue codes without an explicit task to change behavior.
- After any command changes, run at least `make vet` and `make test`.

## Documentation Rules
- Before searching for project details, check `doc/README.md` first.
- If a new document is added, the agent must add it to the index with a short description of its purpose.
- Use numbering like `NNN-*` only for stage/milestone documentation directories.
- Put generally applicable documentation (indexes, maps, shared conventions) in the root of `doc/` without a numbered prefix.
- When updating `doc/CODEBASE_INDEX.md`, every entry must be self-sufficient and include: entrypoint (file path + public function/method), responsibilities of the current level (2-5 concrete tasks), and subpackages with their roles (if any).
- `doc/CODEBASE_INDEX.md` must not contain vague wording without explanation: `details moved out`, `etc.`, `internal logic`.
- Acceptance criterion for an entry in `doc/CODEBASE_INDEX.md`: it must be clear where to go next in the code without extra clarification.
- `doc/CODEBASE_INDEX.md` must be updated in the same change if at least one of these events happened:
  - a file was added/removed/renamed under `cmd/**`, `internal/**`, or `tests/integration/**`;
  - a directory entrypoint file or a public entrypoint-level function/method was changed;
  - subpackage roles, CLI routing, or the set of supported commands changed.
- It is forbidden to finish the task and write the final response without the doc-index self-check:
  - check `git diff --name-status --cached` (or `git diff --name-status` if nothing is staged);
  - if the changes trigger the conditions above, `doc/CODEBASE_INDEX.md` must be present in the diff;
  - if there are documentation changes, also check whether `doc/README.md` remains up to date.
- If the self-check shows that `doc/CODEBASE_INDEX.md` does not need an update, explicitly state in the final response why the triggers did not fire.
