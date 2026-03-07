# Базовая реализация `spec-cli validate` (MVP без `expressions` и `entity_ref`)

## 1. Цель

Документ фиксирует минимальную, но рабочую реализацию команды `validate` по контракту `SPEC_UTILITY_CLI_API_RU.md`.

Цель MVP:

- стабильно проверять схему и документы;
- выдавать детерминированный `json/ndjson`-контракт;
- явно и безопасно деградировать там, где нужны `expressions` и `entity_ref`.

## 2. Границы MVP

### 2.1. Что реализуем

- разбор аргументов `validate` и глобальных флагов (`--workspace`, `--schema`, `--format`, `--require-absolute-paths`);
- загрузка и валидация структуры схемы;
- чтение Markdown-документов и строгий парсинг frontmatter;
- проверки:
  - встроенные поля (`type`, `id`, `slug`, `created_date`, `updated_date`);
  - `meta.fields` (по `schema.type/schema.enum/schema.const` и `required/required_when` без вычисления expression-объектов);
  - `content.sections` (по `required/required_when` без вычисления expression-объектов);
  - уникальность `id`, числового suffix `id` в рамках типа, `slug` в рамках типа;
- формирование `summary`, `issues[]`, `result_state`, exit code;
- вывод `--format json` и `--format ndjson`.

### 2.2. Что пока не реализуем

- вычисление/подстановку `expressions` (включая шаблоны `<...>` в значениях правил);
- валидацию и резолв ссылок `entity_ref`;
- инкрементальные режимы и расширенные области проверки.

### 2.3. Как деградируем по неохваченным зонам

Если во входной схеме/данных встречаются конструкции, требующие `expressions` или `entity_ref`, валидатор:

- добавляет диагностику `ProfileError` с `standard_ref`;
- продолжает остальные проверки, если это безопасно;
- не падает во `INTERNAL_ERROR` только из-за неподдержанной функциональности.

## 3. CLI-контракт `validate` в MVP

### 3.1. Поддерживаемый синтаксис

```bash
spec-cli validate [options]
```

Поддерживаемые опции:

- `--type <entity_type>` (повторяемая);
- `--fail-fast`;
- `--warnings-as-errors`.

### 3.2. Фиксированный режим выполнения в MVP

- Всегда выполняется полная валидация workspace (`full`).
- Проверка контента (`content.sections`) всегда включена.
- Частичная/инкрементальная валидация в этом MVP отсутствует.

### 3.3. Покрытие (`summary.coverage`)

- `coverage.mode="strict"`.
- `coverage.complete=true`.
- `result_state="partially_valid"` в этом MVP не используется.

## 4. Модель данных в коде

### 4.1. Диагностика

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

Обязательные поля: `class`, `message`, `standard_ref`.

`class`: `SchemaError | InstanceError | ProfileError`.

### 4.2. Агрегат результата

- `result_state`;
- `summary`:
  - `schema_valid`;
  - `entities_scanned`;
  - `entities_valid`;
  - `errors`;
  - `warnings`;
  - `coverage` (`mode`, `complete`, `checked_entities`, `candidate_entities`, `skipped_entities`);
- `issues[]`.

## 5. Пайплайн выполнения

### 5.1. Шаг 1: Parse/Normalize CLI

- разобрать флаги;
- проверить взаимоисключения/валидность значений;
- резолвить filesystem path от `cwd`;
- при `--require-absolute-paths` отклонять относительные явно переданные пути (`INVALID_ARGS`, exit `2`).

### 5.2. Шаг 2: Load schema

- загрузить YAML схемы;
- при отсутствии/нечитаемости: `SCHEMA_NOT_FOUND`, exit `4`;
- при ошибке парсинга: `SCHEMA_PARSE_ERROR`, exit `4`;
- при структурной невалидности: `SCHEMA_INVALID`, exit `4`.

### 5.3. Шаг 3: Build candidate set

- собрать все сущности из workspace;
- применить `--type` фильтр;
- вычислить `candidate_entities` и `checked_entities`.

### 5.4. Шаг 4: Parse documents

Для каждого кандидата:

- прочитать файл;
- проверить frontmatter-контракт:
  - начинается первой строкой `---`;
  - закрывается `---` или `...`;
  - верхний уровень — YAML mapping;
  - дубли ключей (включая вложенные) запрещены;
- выделить тело документа;
- подготовить контекст встроенных полей.

Ошибка чтения файла -> I/O код с `exit=3` (baseline для file I/O).

### 5.5. Шаг 5: Validate schema rules vs entity

Проверки сущности:

- `type`: существует в схеме;
- `id`: строка, совпадает с `id_prefix-<number>`;
- числовой suffix `id` парсится как целое >= 0;
- `slug`: строка, формат `^[a-z0-9]+(?:-[a-z0-9]+)*$`;
- `created_date`, `updated_date`: формат `YYYY-MM-DD`;
- `meta.fields`:
  - обязательность считается по модели `required/required_when` (11.5/11.6), где expression в `required_when` в MVP не вычисляется;
  - тип совпадает строго;
  - `enum` (если задан) содержит значение;
  - `schema.const` проверяется только если literal (без `<...>`);
- `content.sections`:
  - обязательность секций считается по модели `required/required_when` (11.5/11.6), где expression в `required_when` в MVP не вычисляется;
  - обязательные секции присутствуют;
  - дубли section-label внутри документа считаются ошибкой.

### 5.6. Шаг 6: Global checks

После прохода всех сущностей:

- глобальная уникальность полного `id`;
- уникальность `slug` в рамках типа;
- уникальность числового suffix `id` в рамках типа.

### 5.7. Шаг 7: Finalize summary

- `errors`/`warnings` считаются по `issues[]`;
- `entities_valid = entities_scanned - entities_with_error`;
- `result_state`:
  - `invalid`, если есть error-issues;
  - `valid`, если ошибок нет и покрытие strict;
- exit code:
  - `1`, если есть ошибки валидации;
  - `0`, если ошибок нет;
  - `1`, если `--warnings-as-errors` и есть warnings.

### 5.8. Шаг 8: Render output

`--format json`:

- единый объект результата `validate`.

`--format ndjson`:

- по строке на issue: `record_type="issue"`;
- финальная строка: `record_type="summary"` с `summary` и `result_state`;
- если issues нет, выводится только summary-строка.

## 6. Обработка неподдержанных конструкций

### 6.1. Expressions

Если правило требует вычисления выражения (`<...>` в `schema.const` или expression-объект в `required_when`), MVP:

- не вычисляет выражение;
- создает issue:
  - `class="ProfileError"`
  - `code="profile.expression_not_supported"`
  - `level="warning"`.

### 6.2. Entity references

Если поле схемы имеет тип `entity_ref` или проверка зависит от резолва ссылок, MVP:

- не делает резолв цели;
- создает issue:
  - `class="ProfileError"`
  - `code="profile.entity_ref_not_supported"`
  - `level="warning"`.

## 7. Псевдокод

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

## 8. Минимальный план реализации по модулям

- `cmd/validate_command.*`
  - парсинг аргументов;
  - вызов use-case;
  - рендер `json/ndjson/text`.
- `validation/engine.*`
  - orchestration пайплайна;
  - fail-fast;
  - summary/result_state.
- `validation/schema_checks.*`
  - базовая структурная валидация схемы.
- `validation/document_parser.*`
  - frontmatter parser + markdown sections extractor.
- `validation/rules_builtin.*`
  - `type/id/slug/date`, `meta.fields`, `content.sections`.
- `validation/rules_global.*`
  - уникальность `id`/suffix/slug.
- `validation/unsupported_rules.*`
  - обнаружение `expressions`/`entity_ref` и генерация `ProfileError`.
- `validation/output.*`
  - json/ndjson writer.

## 9. Критерии готовности MVP

- команда `validate` стабильно отрабатывает на полном workspace;
- `json` и `ndjson` соответствуют контракту (`record_type`, `summary`, `result_state`);
- корректные exit codes (`0/1/2/3/4/5`);
- при `expressions`/`entity_ref` нет silent-pass: есть явная `ProfileError`.

## 10. Что добавить следующим шагом

- полноценный движок `expressions`;
- резолв и валидацию `entity_ref` + граф зависимостей;
- инкрементальные режимы и расширенные области проверки;
- расширенный профиль строгой проверки без деградаций.
