# Индекс документации

Эта страница — единая точка входа в документацию проекта `spec-cli`.

## Как пользоваться

1. Начинать поиск документации с этого файла.
2. Переходить к нужному документу по разделам ниже.
3. При добавлении нового документа в `doc/` обязательно обновлять этот индекс в том же изменении.

## Документы

## Локальная спецификация (из `spec/`)

- `../spec/SPEC_STANDARD_RU_REVISED_V3.md` — рабочая спецификация из директории `spec/`.
- Примечание: директория `spec/` находится в `.gitignore`, это ожидаемое поведение для локального рабочего артефакта.

### Общие документы

- [CODEBASE_INDEX_RU.md](./CODEBASE_INDEX_RU.md) — краткая карта кодовой базы (agent map) в формате `entrypoint + ответственность + подпакеты` для каждого слоя/пакета, включая актуальное состояние `help`/`add`/`update`/`delete`/`version`, канонический path-контракт `help` (`ResolvedPath`), recovery-контракт degraded schema в `help`, pre-dispatch контракт global options CLI и интеграционного раннера (включая data-first группы `cases/help/10_general` без дублирующего default-кейса, `cases/help/15_schema_recovery`, `cases/help/20_errors`, опциональный `workspace.in` для help-кейсов и `workspace.out` для mutating-команд).

### 001 Base

- [001-base/SPEC_UTILITY_CLI_PROTOTYPE_RU.md](./001-base/SPEC_UTILITY_CLI_PROTOTYPE_RU.md) — спецификация прототипа CLI (`validate`, `query`, `add`, `update`), контрактные инварианты, архитектурные рамки, DoD.
- [001-base/SPEC_UTILITY_CLI_VERSION_IMPLEMENTATION_PLAN_RU.md](./001-base/SPEC_UTILITY_CLI_VERSION_IMPLEMENTATION_PLAN_RU.md) — план реализации команды `spec-cli version` (JSON-контракт, общий глобальный `--format`, без command-specific `--format`, коды ошибок и покрытие контрактными тестами).
- [001-base/PLAN_GET_IMPLEMENTATION_RU.md](./001-base/PLAN_GET_IMPLEMENTATION_RU.md) — подробный план реализации baseline-команды `spec-cli get` по API baseline и локальной рабочей спецификации.
- [001-base/SPEC_UTILITY_CLI_ADD_IMPLEMENTATION_PLAN_RU.md](./001-base/SPEC_UTILITY_CLI_ADD_IMPLEMENTATION_PLAN_RU.md) — подробный план реализации команды `spec-cli add` для baseline CLI API: write-контракт из raw schema, pre-write validation, атомарная запись и black-box интеграционные тесты.
- [001-base/QUERY_IMPLEMENTATION_PLAN_RU.md](./001-base/QUERY_IMPLEMENTATION_PLAN_RU.md) — детальный план реализации команды `spec-cli query` на стандартной схеме (`entity/meta.fields/content.sections`): CLI-контракт, read-namespace, фильтрация (включая ограничения `where-json` для `content.sections.*` и запрет `content.raw`), сортировка, пагинация, JSON-ответ и тест-план.
- [001-base/PLAN_UPDATE_IMPLEMENTATION_RU.md](./001-base/PLAN_UPDATE_IMPLEMENTATION_RU.md) — подробный план реализации baseline-команды `spec-cli update`: patch-операции (`--set/--set-file/--unset`), whole-body-режим, pre/post-валидация, optimistic concurrency (`--expect-revision`), атомарная запись, контрактные black-box тесты и зафиксированные решения реализации (payload/errors/hash/path/output scope).
- [001-base/PLAN_DELETE_IMPLEMENTATION_RU.md](./001-base/PLAN_DELETE_IMPLEMENTATION_RU.md) — подробный план реализации baseline-команды `spec-cli delete`: удаление по `id`, optimistic concurrency (`--expect-revision`), reverse-ref защита ссылочной целостности, `dry-run`, JSON/error-контракт и black-box интеграционные сценарии.
- [001-base/SPEC_UTILITY_CLI_VALIDATE_BASE_IMPLEMENTATION_RU.md](./001-base/SPEC_UTILITY_CLI_VALIDATE_BASE_IMPLEMENTATION_RU.md) — базовая реализация команды `spec-cli validate` для MVP (границы, пайплайн, деградация по `expressions`/`entity_ref`, формат `json`).
- [001-base/SPEC_UTILITY_CLI_VALIDATE_EXPRESSIONS_IMPLEMENTATION_PLAN_RU.md](./001-base/SPEC_UTILITY_CLI_VALIDATE_EXPRESSIONS_IMPLEMENTATION_PLAN_RU.md) — план поэтапной реализации полной поддержки `expressions` в `spec-cli validate` (compiler/evaluator/context, диагностики, тест-план, критерии готовности).
- [001-base/SPEC_UTILITY_CLI_VALIDATE_COMPLETION_PLAN_RU_2026-03-09.md](./001-base/SPEC_UTILITY_CLI_VALIDATE_COMPLETION_PLAN_RU_2026-03-09.md) — финальный план завершения реализации `spec-cli validate` (закрытие `expressions` и `entity_ref`, статические проверки схемы, конформантность `validator_conformant`, этапы hardening и тестирования).

### 002 Integration

- [002-integration/INTEGRATION_CASES_LAYOUT_RU.md](./002-integration/INTEGRATION_CASES_LAYOUT_RU.md) — data-first и black-box контрактная структура интеграционных кейсов, layout `tests/integration/cases`, контракт содержимого кейса (`case.json`, `spec.schema.yaml`, `workspace.in/out`, `response.json`), включая двухуровневую группировку `validate/<group>/<case>`, соглашение для `validate`-кейсов `NNNN_ok_*` / `NNNN_err_*` и правило суффикса `_json` для кейсов с `--format json`.
