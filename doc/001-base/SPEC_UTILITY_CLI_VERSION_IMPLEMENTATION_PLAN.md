# `version` Command Implementation Plan

## 1. Goal

Implement the baseline command:

```bash
spec-cli version
```

Expected normative JSON response:

```json
{
  "result_state": "valid",
  "version": "0.1.0"
}
```

The command must follow the general CLI output rules: the format is selected by the global `--format` option, not by command-specific `version` options.

## 2. Baseline Requirements That Affect the Implementation

### 2.1. Command Behavior

- `version` returns the utility version.
- The normative contract of the command is a JSON response.
- `result_state` is required in every top-level JSON response.
- Successful responses use `result_state: "valid"`.
- `version` has no command-specific options; any options after `version` are treated as `INVALID_ARGS`.

### 2.2. Global Rules

- The CLI is invoked as `spec-cli [global options] <command> [command options]`.
- The output format is selected globally through `--format`.
- The supported global format value for the baseline is `json`.
- In JSON mode, data is written to `stdout`, diagnostics to `stderr`.
- The CLI is non-interactive by default.

### 2.3. Errors and Exit Codes

- If the command cannot return its primary result in JSON, it must use a top-level `error`.
- Minimal error fields: `code`, `message`, `exit_code`.
- Exit code `0` is used for success.
- Exit code `2` is used for invalid CLI arguments or an invalid query.
- Exit code `5` is used for internal utility errors.

## 3. Implementation Decisions

### 3.1. Version Source

Choose a single source for the version string that:

- is available without reading the workspace or the schema;
- does not depend on domain data;
- can be used by code, build scripts, and tests.

Practical baseline solution:

- define one canonical source of the version string inside the application;
- the `version` command only reads this value and serializes it into the response;
- do not compute the version dynamically from the workspace, schema, or filesystem state.

Hard policy for the baseline:

- there is only one runtime source of the version: the build-time variable/constant `Version`;
- the release value of `Version` must come from CI/CD or the release pipeline;
- the recommended upstream source for the release value is the Git tag of the release build;
- a local or non-release build must use an explicit fallback such as `dev` or `0.1.0-dev`;
- the runtime code of the `version` command must not call `git`, read tags, commit hashes, workspace files, or schema files to compute the version;
- if the build pipeline did not inject a release version, the command returns the fallback value as a regular build version string.

### 3.2. Command Contract

Minimal command contract:

- input: the `version` command and global CLI options;
- successful JSON output:
  - `result_state = "valid"`
  - `version = <string>`
- the format is controlled only by the global `--format` option under the shared CLI rules.

### 3.3. Responsibility Boundaries

The `version` command must not:

- read the workspace;
- load the schema;
- validate documents;
- depend on entity paths or the read/write namespace;
- return additional fields unless the baseline explicitly requires them.

## 4. Implementation Steps

### Step 1. Introduce the Version Model

- define where the canonical version string lives;
- implement build-time injection for `Version` from the CI/release pipeline;
- define and document the fallback value for local builds;
- expose this value to the command handler;
- avoid duplicating the version value across multiple runtime locations.

Step result:

- the codebase contains a single version source that returns either a release string such as `0.1.0` or a fallback such as `dev`.

### Step 2. Add `version` to CLI Routing

- register the `version` subcommand in the main CLI parser/router;
- connect the command description to the help/discovery layer if help is generated from the command registry;
- ensure that the command does not require command-specific options.

Step result:

- `spec-cli version` reaches a dedicated handler.

### Step 3. Implement the Command Handler

- obtain the canonical version string in the handler;
- build the success payload:
  - `result_state: "valid"`
  - `version: "<semantic version>"`
- do not add command-specific format handling for `version`.

Step result:

- the command consistently produces a JSON response without touching the workspace or the schema.

### Step 4. Integrate Shared Error Handling

- if the command parser receives unsupported arguments, return a CLI error with exit code `2`;
- if the global parser receives an invalid global format, return a CLI error with exit code `2`;
- if response serialization or access to the internal version provider unexpectedly fails, return an internal error with exit code `5`;
- in JSON mode, use a top-level `error` only when the primary payload cannot be returned.

Step result:

- the behavior matches the shared baseline error contract.

### Step 5. Add Tests

- cover the normative JSON scenario;
- cover the global `--format json` usage;
- cover invalid command argument errors;
- cover the invariant that the command does not read the schema/workspace, if that can be verified architecturally or through a unit/integration seam.

Step result:

- the command is protected from contract regressions.

## 5. Test Plan

### 5.1. Positive Scenarios

1. `spec-cli version`
   Expected:
   - exit code `0`
   - `stdout` contains valid JSON
   - the JSON contains only the fields required by the baseline:
     - `result_state = "valid"`
     - `version` as a non-empty string

2. `spec-cli --format json version`
   Expected:
   - exit code `0`
   - the result is equivalent to `spec-cli version`

### 5.2. Negative Scenarios

1. `spec-cli version --unknown-flag`
   Expected:
   - exit code `2`
   - a correct diagnostic message
   - a top-level `error` in JSON

2. `spec-cli version --format json`
   Expected:
   - exit code `2`
   - `INVALID_ARGS`, because `--format` is not a command-specific option of `version`

3. `spec-cli --format yaml version`
   Expected:
   - exit code `2`
   - an error for an invalid value of the global `--format`

4. Internal version provider failure
   Expected:
   - exit code `5`
   - a top-level `error` with the required fields

5. Local build without CI injection
   Expected:
   - exit code `0`
   - the command returns a predefined fallback such as `dev` or `0.1.0-dev`
   - the behavior is deterministic and does not depend on Git metadata at runtime

### 5.3. Contract Checks

- `result_state` is always present in a successful JSON response;
- `version` is present only as a string field;
- the command does not add domain fields unrelated to the baseline `version`;
- `stderr` does not pollute `stdout` in JSON mode;
- the runtime of the command does not read Git metadata to compute the version.

## 6. Completion Criteria

The command is considered implemented if:

- `spec-cli version` returns the normative baseline JSON;
- the command exits successfully with code `0`;
- invalid arguments exit with code `2`;
- internal failures exit with code `5`;
- the implementation does not depend on the workspace or schema;
- automated tests were added for JSON and error-path scenarios.

## 7. Rollout Order

Recommended sequence:

1. Extract the canonical version source.
2. Configure build-time version injection from CI/tag and the fallback for local builds.
3. Register the `version` command in the CLI parser/router.
4. Implement the JSON payload through the shared output pipeline.
5. Connect shared error handling.
6. Add unit/integration tests.
7. Verify the actual CLI output manually or with a process-level test.

## 8. Out of Scope

- extending the `version` contract with additional build metadata;
- reading the version from the schema or the workspace;
- supporting additional machine-first modes beyond baseline JSON;
- adding command-specific format behavior for `version`;
- any changes to `help`, `validate`, `query`, `get`, `add`, `update`, or `delete`.
