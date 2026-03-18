# Baseline `spec-cli validate` Implementation (MVP without `expressions` and `entity_ref`)

## 1. Goal

This document records a minimal but working implementation of the `validate` command under the `SPEC_UTILITY_CLI_API_RU.md` contract.

MVP goal:

- validate the schema and documents in a stable way;
- produce a deterministic `json/ndjson` contract;
- degrade explicitly and safely where `expressions` and `entity_ref` are required.

## 2. MVP Scope

### 2.1. What Is Implemented

- parsing `validate` arguments and global flags (`--workspace`, `--schema`, `--format`, `--require-absolute-paths`);
- loading the schema and validating its structure;
- reading Markdown documents and strictly parsing frontmatter;
- checks for:
  - built-in fields (`type`, `id`, `slug`, `created_date`, `updated_date`);
  - `meta.fields` (`schema.type/schema.enum/schema.const` and `required/required_when` without evaluating expression objects);
  - `content.sections` (`required/required_when` without evaluating expression objects);
  - uniqueness of `id`, numeric `id` suffix inside a type, and `slug` inside a type;
- building `summary`, `issues[]`, `result_state`, and the exit code;
- output for `--format json` and `--format ndjson`.

### 2.2. What Is Not Implemented Yet

- evaluation/substitution of `expressions` (including `<...>` templates inside rule values);
- validation and resolution of `entity_ref` links;
- incremental modes and extended validation scopes.

### 2.3. How Unsupported Areas Degrade

If the input schema/data contains constructs that require `expressions` or `entity_ref`, the validator:

- adds a `ProfileError` diagnostic with `standard_ref`;
- continues the remaining checks where that is safe;
- does not fail with `INTERNAL_ERROR` only because of unsupported functionality.

## 3. MVP `validate` CLI Contract

### 3.1. Supported Syntax

```bash
spec-cli validate [options]
```

Supported options:

- `--type <entity_type>` (repeatable);
- `--fail-fast`;
- `--warnings-as-errors`.

### 3.2. Fixed Execution Mode in the MVP

- The full workspace is always validated (`full`).
- Content validation (`content.sections`) is always enabled.
- Partial/incremental validation is not part of this MVP.

### 3.3. Coverage (`summary.coverage`)

- `coverage.mode="strict"`.
- `coverage.complete=true`.
- `result_state="partially_valid"` is not used in this MVP.

## 4. Data Model in Code

### 4.1. Diagnostic

```json
{
  "code": "meta.required_missing",
  "level": "error",
  "class": "InstanceError",
  "message": "Required metadata field 'owner' is missing",
  "standard_ref": "11.5",
  "entity": {
    "type": "feature",
    "id": "FEAT-1",
    "slug": "core-auth"
  },
  "field": "frontmatter.owner"
}
```

Required fields: `class`, `message`, `standard_ref`.

`class`: `SchemaError | InstanceError | ProfileError`.

### 4.2. Result Aggregate

- `result_state`;
- `summary`:
  - `schema_valid`;
  - `entities_scanned`;
  - `entities_valid`;
  - `errors`;
  - `warnings`;
  - `coverage` (`mode`, `complete`, `checked_entities`, `candidate_entities`, `skipped_entities`);
- `issues[]`.

## 5. Execution Pipeline

### 5.1. Step 1: Parse/Normalize CLI

- parse flags;
- validate mutual exclusions/value correctness;
- resolve filesystem paths from `cwd`;
- when `--require-absolute-paths` is enabled, reject explicitly provided relative paths (`INVALID_ARGS`, exit `2`).

### 5.2. Step 2: Load Schema

- load schema YAML;
- if missing/unreadable: `SCHEMA_NOT_FOUND`, exit `4`;
- if parsing fails: `SCHEMA_PARSE_ERROR`, exit `4`;
- if structurally invalid: `SCHEMA_INVALID`, exit `4`.

### 5.3. Step 3: Build Candidate Set

- collect all entities from the workspace;
- apply the `--type` filter;
- compute `candidate_entities` and `checked_entities`.

### 5.4. Step 4: Parse Documents

For each candidate:

- read the file;
- validate the frontmatter contract:
  - it starts with `---` on the first line;
  - it closes with `---` or `...`;
  - the top level is a YAML mapping;
  - duplicate keys, including nested ones, are forbidden;
- extract the document body;
- build the built-in field context.

File read errors map to an I/O code with `exit=3` (baseline rule for file I/O).

### 5.5. Step 5: Validate Schema Rules Against the Entity

Entity checks:

- `type`: exists in the schema;
- `id`: string, matches `id_prefix-<number>`;
- numeric `id` suffix parses as integer `>= 0`;
- `slug`: string, format `^[a-z0-9]+(?:-[a-z0-9]+)*$`;
- `created_date`, `updated_date`: `YYYY-MM-DD` format;
- `meta.fields`:
  - requiredness follows the `required/required_when` model (11.5/11.6), where expression-form `required_when` is not evaluated in the MVP;
  - type must match exactly;
  - `enum` (if present) must contain the value;
  - `schema.const` is checked only when it is literal (without `<...>`);
- `content.sections`:
  - section requiredness follows the `required/required_when` model (11.5/11.6), where expression-form `required_when` is not evaluated in the MVP;
  - required sections must exist;
  - duplicate section labels inside one document are errors.

### 5.6. Step 6: Global Checks

After iterating through all entities:

- global uniqueness of full `id`;
- uniqueness of `slug` within a type;
- uniqueness of numeric `id` suffix within a type.

### 5.7. Step 7: Finalize Summary

- `errors`/`warnings` are counted from `issues[]`;
- `entities_valid = entities_scanned - entities_with_error`;
- `result_state`:
  - `invalid` if there are error issues;
  - `valid` if there are no errors and coverage is strict;
- exit code:
  - `1` if validation errors exist;
  - `0` if there are no errors;
  - `1` if `--warnings-as-errors` is set and there are warnings.

### 5.8. Step 8: Render Output

`--format json`:

- one unified `validate` result object.

`--format ndjson`:

- one line per issue: `record_type="issue"`;
- final line: `record_type="summary"` with `summary` and `result_state`;
- if there are no issues, only the summary line is emitted.

## 6. Handling Unsupported Constructs

### 6.1. Expressions

If a rule requires expression evaluation (`<...>` in `schema.const` or an expression object in `required_when`), the MVP:

- does not evaluate the expression;
- creates an issue:
  - `class="ProfileError"`
  - `code="profile.expression_not_supported"`
  - `level="warning"`.

### 6.2. Entity References

If a schema field has type `entity_ref` or validation depends on resolving links, the MVP:

- does not resolve the target;
- creates an issue:
  - `class="ProfileError"`
  - `code="profile.entity_ref_not_supported"`
  - `level="warning"`.

## 7. Pseudocode

```text
runValidate(args):
  cfg = parseArgs(args)
  schema = loadAndValidateSchema(cfg.schemaPath)

  candidates = buildCandidates(cfg, schema)
  issues = []
  stats = initStats(candidates)

  for entity in candidates:
    doc = parseDocument(entity.path)
    issues += validateBuiltinFields(doc, schema)
    issues += validateMetaRequiredFieldsLiteralOnly(doc, schema)
    issues += validateContentSections(doc, schema)

    issues += detectUnsupportedExpressionUsage(doc, schema)
    issues += detectUnsupportedEntityRefUsage(doc, schema)

    if cfg.failFast and hasError(issues):
      break

  issues += runGlobalUniquenessChecks(candidates)

  summary = buildSummary(stats, issues)
  result = buildResult(summary, issues)

  render(cfg.format, result)
  return mapToExitCode(result, cfg.warningsAsErrors)
```

## 8. Minimal Implementation Plan by Module

- `cmd/validate_command.*`
  - argument parsing;
  - use-case invocation;
  - `json/ndjson/text` rendering.
- `validation/engine.*`
  - pipeline orchestration;
  - fail-fast;
  - summary/result_state.
- `validation/schema_checks.*`
  - baseline structural schema validation.
- `validation/document_parser.*`
  - frontmatter parser + Markdown sections extractor.
- `validation/rules_builtin.*`
  - `type/id/slug/date`, `meta.fields`, `content.sections`.
- `validation/rules_global.*`
  - uniqueness of `id`/suffix/slug.
- `validation/unsupported_rules.*`
  - detection of `expressions`/`entity_ref` and `ProfileError` generation.
- `validation/output.*`
  - JSON/NDJSON writer.

## 9. MVP Completion Criteria

- the `validate` command runs reliably over the full workspace;
- `json` and `ndjson` match the contract (`record_type`, `summary`, `result_state`);
- exit codes are correct (`0/1/2/3/4/5`);
- when `expressions`/`entity_ref` are present, there is no silent pass: an explicit `ProfileError` exists.

## 10. What to Add Next

- a full `expressions` engine;
- `entity_ref` resolution and validation plus the dependency graph;
- incremental modes and extended validation scopes;
- an extended strict profile without degradations.
