# Data-First Structure of `spec-cli` Integration Cases

This document defines the convention for integration tests in the format of **real data**,
so the primary readability lives in the case file structure rather than in Go code.

Related document: [SPEC_UTILITY_CLI_PROTOTYPE.md](../001-base/SPEC_UTILITY_CLI_PROTOTYPE.md).

## 1. Principles

- Integration tests are `data-first`: a case is described by files.
- Go tests and the runner are minimal: load the case, run the CLI, compare the expected result.
- Integration tests are **black-box contract tests**: only observable CLI behavior is checked.
- Cases are grouped **by command/contract suite**: `validate`, `query`, `add`, `update`, and `global_options`.
- `validate` uses an additional domain grouping `group -> case` (strictly 2 levels).
- Each case contains the input workspace, the schema, and the expected CLI response.
- For mutating commands (`add`, `update`), the case also contains the expected `workspace.out`.
- The case catalog is stored separately from ordinary `testdata`: `tests/integration/cases`.

### 1.1. Black-Box Boundaries (Mandatory)

- A case is defined through the public contract: `args` -> `stdout/stderr` -> `exit_code` (and `workspace.out` for mutating commands).
- Scenarios and expectations must not assume anything about the internal architecture/implementation (layers, engine reuse, order of internal steps).
- Every meaningful contract behavior must have a **direct** integration case.
- Indirect coverage via another scenario is not sufficient evidence of contract behavior.

## 2. Canonical Layout

```text
tests/
  integration/
    run_cases_test.go
    runner/
      load_case.go
      run_cli.go
      assert_response.go
      assert_workspace.go
      normalize.go

    cases/
      validate/
        <GG_group-name>/
          <NNNN_outcome_case-id>/
            case.json
            spec.schema.yaml
            workspace.in/
            response.json

      query/
        <XXXX_case-id>/
          case.json
          spec.schema.yaml
          workspace.in/
          response.json

      add/
        <XXXX_case-id>/
          case.json
          spec.schema.yaml
          workspace.in/
          workspace.out/
          response.json

      update/
        <XXXX_case-id>/
          case.json
          spec.schema.yaml
          workspace.in/
          workspace.out/
          response.json

      global_options/
        <GG_group-name>/
          <NNNN_outcome_case-id>/
            case.json
            spec.schema.yaml
            workspace.in/
            response.json
```

## 3. Case Directory Contract

The case directory must contain:

- for `validate`: `tests/integration/cases/validate/<GG_group-name>/<NNNN_outcome_case-id>/`;
- for `query|add|update`: `tests/integration/cases/<command>/<XXXX_case-id>/`.
- for `global_options`: `tests/integration/cases/global_options/<GG_group-name>/<NNNN_outcome_case-id>/`.

Case directory contents:

- `case.json` - execution and assertion metadata.
- `spec.schema.yaml` - schema used by the command.
- `workspace.in/` - input workspace state.
- `response.json` - expected command response (canonicalized JSON).

Additional content for mutating commands:

- `workspace.out/` - expected workspace state after execution.

Optional files (if separate control is needed):

- `stderr.txt` - expected stderr.
- `notes.md` - case notes.

## 4. `case.json` Format

`case.json` describes only the execution scenario; the actual data lives in neighboring files.

Example:

```json
{
  "id": "add_doc_minimal",
  "description": "add creates a new document in empty workspace",
  "command": "add",
  "args": [
    "add",
    "--workspace", "${WORKSPACE}",
    "--schema", "${SCHEMA}",
    "--format", "json",
    "--type", "doc",
    "--slug", "intro"
  ],
  "expect": {
    "exit_code": 0,
    "response_file": "response.json",
    "stderr_file": ""
  },
  "workspace": {
    "input_dir": "workspace.in",
    "output_dir": "workspace.out",
    "assert_output": true
  },
  "runtime": {
    "cwd": "${WORKSPACE}"
  }
}
```

Field rules:

- `command`: one of `validate|query|add|update` (for dedicated global-options suites the command in each case remains the real CLI command, e.g. `validate`).
- `args`: CLI arguments without the binary name.
- `${WORKSPACE}` and `${SCHEMA}` are runner placeholders.
- `expect.exit_code`: expected process exit code.
- `expect.response_file`: path to the expected response file (usually `response.json`).
- `runtime.cwd` (optional): process working directory for the case; supports `${WORKSPACE}` and `${SCHEMA}` placeholders; default is repository root.
- `workspace.assert_output`:
  - `false` for read-only commands (`validate`, `query`),
  - `true` for mutating commands (`add`, `update`).

## 5. `response.json` Representation

`response.json` is required in all cases.

- For `--format json`, it stores a regular top-level JSON response object.
- For `--format ndjson`, it stores a **canonical JSON representation of NDJSON**:
  - an array of records in the expected order;
  - each record corresponds to one NDJSON line.

NDJSON example:

```json
[
  { "record_type": "item", "result_state": "valid", "item": { "id": "d1" } },
  { "record_type": "summary", "result_state": "valid", "summary": { "returned": 1 } }
]
```

This way all cases are read the same way: a single expected file type (`response.json`) regardless of output format.

## 6. Comparison Rules

### 6.1 Command Response

The runner must compare:

- the structure and values of `response.json`;
- required contract invariants (`result_state`, `error.*`, `record_type`, `revision` depending on the scenario);
- `exit_code`.

The runner must not use internal implementation diagnostics that are unavailable to an external CLI user.

Normalization of unstable values (for example, time fields) is allowed, but only through an explicit rule in `runner/normalize.go`.

### 6.2 Workspace

If `workspace.assert_output=true`, the runner compares `workspace.out` with the actual workspace after the command:

- directory and file structure;
- byte-for-byte content of text files;
- no unexpected extra files.

## 7. Case Naming

Base case directory format:

`<XXXX>_<case-id>`

where:

- `XXXX` is a required 4-digit numeric prefix (`0001`, `0002`, ...);
- `<case-id>` is a semantic scenario identifier.

`validate` uses an additional group level:

- group directory: `<GG>_<group-name>`;
- case directory inside the group: `<NNNN>_<outcome>_<case-id>`;
- `outcome` is a required marker of the expected result (`ok` or `err`);
- `GG` is a two-digit group code (`10`, `20`, ...), `NNNN` is a 4-digit sequence number within the group.

`validate` group codes:

- `10_contract`
- `20_schema`
- `30_instance_builtin`
- `40_instance_meta_content`
- `50_pathTemplate_expr`
- `60_entityRef_context`
- `70_global_uniqueness`
- `80_profile_conformance`

Recommended `case-id` format:

`<intent>_<scope>_<expected>`

Examples:

- `10_contract/0001_ok_validate_full_ok_json`
- `40_instance_meta_content/0001_err_validate_required_when_meta_and_sections_json`
- `0002_query_by_tag_valid`
- `0101_add_doc_valid_minimal`
- `0203_update_title_conflict_invalid`

Requirements:

- mandatory prefix `XXXX_` before `case-id`;
- only `lower_snake_case`;
- the name must reflect the expected behavior;
- the name must remain stable over time (used in logs and CI reports);
- for `validate`, the outcome prefix is mandatory: `<NNNN>_ok_...` for `expect.exit_code=0`, `<NNNN>_err_...` for `expect.exit_code!=0`;
- for cases with `--format json`, the directory name must end with `_json`;
- recommended `case.json.id` format for `validate`: `validate_<GG>_<NNNN>_<outcome>_<case-id>`.

## 8. Minimal Case Set (First Pass)

For each command:

- baseline `success`;
- `invalid_args`;
- `domain_error`;
- `schema_error`;
- `json` format and `ndjson` format.

Additionally:

- for `add/update`: at least 1 case with `workspace.out`;
- for `query`: at least 1 case with `items` and 1 case with an empty result;
- for `validate`: at least 1 case with issues.

## 9. Why Not `testdata`

`testdata` works well for ordinary unit tests, but the goal here is different:

- make integration scenarios visually self-documenting;
- separate large data-driven cases from package-level unit tests;
- simplify review through file diffs over inputs/outputs.

That is why cases are stored as first-class artifacts in `tests/integration/cases`.

## 10. Checklist for a New Case

1. Create the case directory:
   - `validate`: `tests/integration/cases/validate/<GG_group-name>/<NNNN_outcome_case-id>/`;
   - `query|add|update`: `tests/integration/cases/<command>/<XXXX_case-id>/`.
2. Add `case.json`.
3. Add `spec.schema.yaml`.
4. Prepare `workspace.in/`.
5. Add `response.json`.
6. For `add/update`, add `workspace.out/` and set `assert_output=true`.
7. Run the integration runner and verify that the case passes without manual assumptions.
