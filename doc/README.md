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

### 001 Base

- [001-base/SPEC_UTILITY_CLI_PROTOTYPE_RU.md](./001-base/SPEC_UTILITY_CLI_PROTOTYPE_RU.md) — спецификация прототипа CLI (`validate`, `query`, `add`, `update`), контрактные инварианты, архитектурные рамки, DoD.
- [001-base/SPEC_UTILITY_CLI_VALIDATE_BASE_IMPLEMENTATION_RU.md](./001-base/SPEC_UTILITY_CLI_VALIDATE_BASE_IMPLEMENTATION_RU.md) — базовая реализация команды `spec-cli validate` для MVP (границы, пайплайн, деградация по `expressions`/`entity_ref`, формат `json/ndjson`).

### 002 Integration

- [002-integration/INTEGRATION_CASES_LAYOUT_RU.md](./002-integration/INTEGRATION_CASES_LAYOUT_RU.md) — data-first структура интеграционных кейсов, layout `tests/integration/cases`, контракт содержимого кейса (`case.json`, `spec.schema.yaml`, `workspace.in/out`, `response.json`), правило именования директорий кейсов с числовым префиксом `XXXX_`.
