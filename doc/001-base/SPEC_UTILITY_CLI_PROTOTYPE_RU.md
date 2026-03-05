# Прототип `spec-cli` (фокус: `validate`, `query`, `add`, `update`)

Документ описывает прототип CLI-утилиты на основе требований из [SPEC_UTILITY_CLI_API_RU.md](./SPEC_UTILITY_CLI_API_RU.md).
Цель: зафиксировать минимально полезную реализацию с машинно-стабильным контрактом и архитектурой, готовой к расширению.

## 1. Рамки прототипа

В прототип входят только команды:

- `validate`
- `query`
- `add`
- `update`

Ограничения прототипа:

- утилита machine-first (первичный клиент: AI-агент/CI);
- неинтерактивная работа по умолчанию;
- обязательная поддержка `--format json` и `--format ndjson` для всех 4 команд;
- единые `result_state`, `error.code`, `error.exit_code`;
- обязательная поддержка `revision` в ответах с `entity`.

## 2. Базовые принципы контракта

### 2.1 Формат запуска

```bash
spec-cli [global options] <command> [command options]
```

Глобальные опции прототипа:

- `--workspace <path>` (default: `.`)
- `--schema <path>` (default: `spec.schema.yaml`, резолв от `cwd`)
- `--format <json|ndjson>`
- `--config <path>`
- `--require-absolute-paths`
- `--verbose`

Правила `--config`:

- `--config` указывает на JSON/YAML-файл с дефолтами глобальных опций;
- приоритет значений: явный CLI-флаг > `--config` > встроенный default;
- неизвестные ключи в config-файле завершаются `INVALID_ARGS`;
- недоступный/непарсящийся config-файл завершается `INVALID_ARGS`.

Правила `--verbose`:

- `--verbose` не меняет обязательные поля и семантику контракта;
- в `json/ndjson` может добавляться опциональный объект `diagnostics`;
- `diagnostics` допускается в top-level `result`, в `summary`, и в `error.details.debug`.

### 2.2 Обязательные инварианты

- В `json/ndjson` данные печатаются в `stdout`, диагностика — в `stderr`.
- CLI не делает prompt при нехватке аргументов.
- В контрактах не раскрываются внутренние filesystem path сущностей.
- Для `query` по умолчанию детерминированная сортировка: `type asc`, затем `id asc`.
- В каждом `json`-ответе и в NDJSON-записях `result/summary/error` есть `result_state`.

Допустимые `result_state`:

- `valid`
- `invalid`
- `partially_valid`
- `not_found`
- `unsupported`
- `indeterminate`

### 2.3 NDJSON-паттерн

- Для single-response команд (`add`, `update`) поток содержит `record_type: "result"` и финальный `record_type: "summary"`.
- Для коллекций (`query`) данные идут как `record_type: "item"`, затем `summary`.
- Для валидации (`validate`) данные идут как `record_type: "issue"`, затем `summary`.
- При ошибке до начала stream: одна строка `record_type: "error"` без `summary`.
- При runtime-ошибке после частичного stream: финальная строка `record_type: "error"` с `result_state: "indeterminate"` без `summary`.

### 2.4 Конкурентность и `revision`

- `revision` — вычисляемый opaque token состояния документа (frontmatter + content).
- `revision` возвращается в ответах с `entity`.
- `revision` используется как машинный маркер версии сущности для последующего сравнения состояний.

## 3. Контракты команд прототипа

### 3.1 `validate`

Назначение:

- проверка схемы;
- проверка документов на соответствие схеме;
- проверка уникальности `slug`/`id`;
- проверка `meta` и `content.sections`.

Синтаксис:

```bash
spec-cli validate
```

Опции команды в прототипе:

- отсутствуют (валидация запускается в фиксированном full-режиме)

Ключевые правила:

- всегда валидируется весь workspace (`validation_scope=full`);
- покрытие всегда строгое (`coverage.mode=strict`);
- проверка контента включена всегда;
- режимов инкрементальной/частичной валидации в прототипе нет.

Минимальный JSON-контракт:

```json
{
  "result_state": "invalid",
  "validation_scope": "full",
  "summary": {
    "schema_valid": true,
    "validator_conformant": true,
    "entities_scanned": 42,
    "entities_valid": 39,
    "errors": 3,
    "warnings": 1,
    "coverage": {
      "mode": "strict",
      "complete": true
    }
  },
  "issues": []
}
```

### 3.2 `query`

Назначение:

- структурный поиск сущностей по built-in полям, `meta.*` и ссылочным полям.

Синтаксис:

```bash
spec-cli query [options] --where-json <json>
```

Опции прототипа:

- `--type <entity_type>` (повторяемая)
- `--where-json <json>` или `--where-file <path>` (ровно один источник)
- `--fields <preset>`
- `--select <field>` (повторяемая)
- `--limit`, `--offset` (offset pagination)
- `--sort <field[:asc|desc]>` (повторяемая)
- `--count-only`
- `--validate-query`

Поддерживаемые операторы:

- `eq`, `neq`, `in`, `not_in`, `exists`, `not_exists`, `gt`, `gte`, `lt`, `lte`, `contains`
- логические узлы: `and`, `or`, `not`

Ключевые правила:

- неизвестный оператор/поле -> `INVALID_QUERY`;
- несовместимые типы в сравнениях -> `INVALID_QUERY`;
- `enum` проверяется с учетом регистра;
- при пользовательском `--sort` добавляется стабильный хвост `type:asc,id:asc` (если отсутствует).
- при `--count-only` поле `items` возвращается пустым массивом;
- при `--count-only` поле `matched` обязательно и отражает число совпадений до пагинации;
- при `--count-only` в `page.returned` возвращается `0`.

Минимальный JSON-контракт:

```json
{
  "result_state": "valid",
  "items": [],
  "matched": 0,
  "page": {
    "mode": "offset",
    "limit": 100,
    "offset": 0,
    "returned": 0,
    "has_more": false,
    "next_offset": null,
    "effective_sort": ["type:asc", "id:asc"]
  }
}
```

### 3.3 `add`

Назначение:

- создание новой сущности/документа.

Синтаксис:

```bash
spec-cli add [options]
```

Обязательные опции:

- `--type <entity_type>`
- `--slug <slug>`

Опции прототипа (ядро):

- ссылочный контекст: `--ref-field`, `--ref-id`
- метаданные: `--meta`, `--meta-json`, `--meta-file`
- контент: `--content-file`, `--content-stdin`
- режим: `--dry-run`

Ключевые правила:

- `--content-file` и `--content-stdin` взаимоисключающие;
- `--meta-json` и `--meta-file` взаимоисключающие, `--meta` их дополняет/переопределяет;
- `id`, `created_date`, `updated_date` всегда вычисляются автоматически;
- если тип сущности требует иерархическую ссылку (`entity_ref`) для размещения/валидации, ссылочный контекст обязателен;
- если задается ссылка (передан `--ref-id`), `--ref-field` обязателен, иначе `INVALID_ARGS`;
- `--ref-field` без `--ref-id` недопустим (`INVALID_ARGS`);
- если тип требует ссылочный контекст, `--ref-field` обязателен всегда, даже если подходящее поле в схеме одно;
- если тип не требует ссылочного контекста и `--ref-id` не передан, создание допускается без ссылки;
- `--ref-id` резолвится глобально (предполагается глобальная уникальность `id` между типами);
- если целевой путь уже существует, команда завершается `PATH_CONFLICT`;
- pre-validation и post-validation всегда включены;
- запись атомарна: при провале post-validation состояние диска откатывается.

Минимальный JSON-контракт:

```json
{
  "result_state": "valid",
  "dry_run": false,
  "created": true,
  "file": {
    "existed_before": false
  },
  "entity": {
    "type": "feature",
    "id": "FEAT-8",
    "slug": "retry-window",
    "created_date": "2026-02-25",
    "updated_date": "2026-02-25",
    "revision": "sha256:abc123",
    "metadata": {}
  },
  "validation": {
    "before_write_ok": true,
    "after_write_ok": true,
    "issues": []
  }
}
```

### 3.4 `update`

Назначение:

- patch-обновление существующей сущности без неявной полной перезаписи документа.

Синтаксис:

```bash
spec-cli update [options]
```

Обязательная идентификация:

- `--id <id>`

Опции прототипа (ядро):

- patch metadata: `--set-meta`, `--set-meta-json`, `--set-meta-file`, `--unset-meta`
- patch content: `--content-file`, `--content-stdin`, `--clear-content`
- patch sections: `--set-section`, `--set-section-file`, `--clear-section`
- режим: `--dry-run`

Ключевые правила:

- изменяются только явно указанные поля;
- конфликтующие patch-источники -> `INVALID_ARGS`;
- если patch-операций не передано -> `INVALID_ARGS`;
- pre-validation и post-validation всегда включены;
- идемпотентность обязательна: повтор того же patch может вернуть успешный `noop`;
- запись атомарна: post-validation fail не оставляет частично записанный файл.

Минимальный JSON-контракт:

```json
{
  "result_state": "valid",
  "dry_run": false,
  "updated": true,
  "noop": false,
  "target": {
    "match_by": "id",
    "id": "FEAT-8"
  },
  "file": {
    "written": true
  },
  "changes": [
    {
      "field": "metadata.status",
      "op": "set",
      "before": "draft",
      "after": "active"
    }
  ],
  "entity": {
    "type": "feature",
    "id": "FEAT-8",
    "slug": "retry-window",
    "created_date": "2026-02-25",
    "updated_date": "2026-02-26",
    "revision": "sha256:def456",
    "metadata": {}
  },
  "validation": {
    "before_write_ok": true,
    "after_write_ok": true,
    "issues": []
  }
}
```

## 4. Ошибки и exit codes (для прототипа)

Базовые exit codes:

- `0` success
- `1` доменная ошибка команды
- `2` ошибка аргументов/запроса
- `3` ошибка чтения/записи
- `4` ошибка схемы
- `5` внутренняя ошибка

Минимальный JSON-формат ошибки:

```json
{
  "result_state": "invalid",
  "error": {
    "code": "INVALID_QUERY",
    "message": "Invalid structured filter node",
    "exit_code": 2,
    "details": {
      "arg": "--where-json"
    }
  }
}
```

Коды, которые обязательно реализовать уже в прототипе:

- `INVALID_ARGS`
- `SCHEMA_NOT_FOUND`
- `SCHEMA_PARSE_ERROR`
- `SCHEMA_INVALID`
- `ENTITY_TYPE_UNKNOWN`
- `ENTITY_NOT_FOUND`
- `TARGET_AMBIGUOUS`
- `PATH_CONFLICT`
- `ID_CONFLICT`
- `SLUG_CONFLICT`
- `INVALID_QUERY`
- `WRITE_FAILED`
- `INTERNAL_ERROR`

## 5. Расширяемая архитектура прототипа

### 5.0 Технологический выбор

- Язык реализации прототипа: `Go` (целевой baseline: `Go 1.24+`).
- Формат поставки: один CLI-бинарник `spec-cli`.
- Предпочтение стандартной библиотеке; внешние зависимости только при явной пользе (например, CLI-роутинг).

### 5.1 Архитектурный стиль

`Hexagonal + Command Bus`:

- CLI-слой только парсит аргументы и строит `CommandRequest`.
- Каждый use-case (`validate/query/add/update`) реализован отдельным command-handler.
- Доступ к ФС, схеме, сериализации, индексу и времени — через порты (интерфейсы).
- Вывод (`json/ndjson`) отделен от бизнес-логики.

Преимущества:

- легко добавлять новые команды (`delete/move/rename/...`) без рефакторинга ядра;
- можно переиспользовать use-case в API/daemon режиме;
- упрощается контрактное тестирование.

### 5.2 Слои и модули

Рекомендуемая структура:

```text
cmd/
  spec-cli/
    main.go
internal/
  cli/
    app.go
    global_options.go
    router.go
  application/
    commandbus/
      bus.go
    commands/
      validate/
        handler.go
      query/
        handler.go
      add/
        handler.go
      update/
        handler.go
  domain/
    entity/
      model.go
    query/
      ast.go
      validator.go
      evaluator.go
    validation/
      issue.go
    errors/
      codes.go
  infrastructure/
    fs/
    schema/
    markdown/
    index/
    clock/
  output/
    jsonwriter/
    ndjsonwriter/
    errormap/
  contracts/
    responses/
    capabilities/
```

### 5.3 Ключевые доменные контракты

- `Entity`: `type`, `id`, `slug`, `created_date`, `updated_date`, `revision`, `metadata`, `content`.
- `RevisionService`: вычисление opaque `revision`.
- `QueryAst` + `QueryValidator` + `QueryEvaluator`.
- `ValidationIssue`: `class`, `message`, `standard_ref`, `severity`.
- `MutationPlan` для `add/update`: описывает вычисленные изменения до записи.

### 5.4 Единый pipeline write-команд

Для `add` и `update` используется общий сценарий:

1. parse args -> `CommandRequest`;
2. preflight (schema/load/index/target resolve);
3. построение `MutationPlan`;
4. pre-validation;
5. `dry-run` ответ или атомарная запись;
6. post-validation;
7. rollback при ошибке;
8. формирование стабильного JSON/NDJSON-ответа.

Такой pipeline облегчает добавление будущих write-команд (`move/rename/retarget/delete/apply`) через переиспользование шагов 2-8.

### 5.5 Точки расширения

- Реестр команд (`CommandRegistry`) с декларативной регистрацией.
- Реестр capability-флагов (`CapabilitiesProvider`) с авто-генерацией `capabilities`.
- Расширяемый каталог query-операторов (новые `op` без поломки текущих).
- Плагины валидатора (дополнительные правила/профили).
- Абстракция storage-adapter (локальная ФС сейчас, удаленный backend позже).
- Новые output-writer для дополнительных машинных форматов без изменения command-handlers.

## 6. План реализации прототипа

1. Каркас CLI, global options, error mapping, `json/ndjson` writers.
2. Базовые infra-модули: schema loader, markdown parser/serializer, entity index.
3. `validate` с `summary/issues` и NDJSON `issue + summary`.
4. `query` с `where-json`, AST/validator/evaluator, paging/sort.
5. `add` с автоматическим вычислением встроенных полей, dry-run, atomic write, pre/post validation.
6. `update` с patch semantics, `changes[]`, no-op.
7. Контрактные тесты и golden snapshots для JSON/NDJSON ответов.

## 7. Definition of Done для прототипа

- Все 4 команды реализованы и стабильно работают в `--format json` и `--format ndjson`.
- Все ответы содержат корректный `result_state`.
- `query` детерминирован при отсутствии пользовательской сортировки.
- `add/update` поддерживают `dry-run`, pre/post validation, rollback, атомарность.
- `update` возвращает `changes[]`.
- Ошибки соответствуют единому JSON/NDJSON-контракту и маппингу `error.code -> exit_code`.
- Есть тесты на позитивные и негативные сценарии по каждой команде.

## 8. Data-first интеграционные кейсы

Детальное соглашение по структуре интеграционных кейсов (каталог `tests/integration/cases`,
структура по командам, контракт директории кейса, формат `case.json`/`response.json`,
правила `workspace.in/workspace.out`) описано в документе:

- [INTEGRATION_CASES_LAYOUT_RU.md](../002-integration/INTEGRATION_CASES_LAYOUT_RU.md)
