# Индекс кодовой базы (Agent Map)

Короткая карта проекта для быстрого входа в код.  
Детали и реализация читаются напрямую в указанных файлах.

## Правило актуальности

- Любые изменения структуры директорий, entrypoint-файлов, ролей слоёв или состава команд должны сопровождаться обновлением этого файла в том же изменении.

## Быстрый маршрут выполнения CLI

1. `cmd/spec-cli/main.go`
2. `internal/cli/app.go`
3. `internal/application/commandbus/bus.go`
4. `internal/application/commands/<command>/handler.go`

## Карта слоёв

- `cmd/spec-cli`
  - `main.go` — process entrypoint.
- `internal/cli`
  - `app.go` — сборка приложения, регистрация команд, единый output/error rendering.
  - `global_options.go` — парсинг глобальных флагов (`--format`, `--workspace`, `--schema`).
  - `router.go` — список поддерживаемых команд.
- `internal/application/commandbus`
  - `bus.go` — регистрация и dispatch command handlers.
- `internal/application/commands`
  - `validate/handler.go` — orchestration `validate`.
  - `validate/internal/options` — parse/norm опций команды.
  - `validate/internal/schema` — загрузка и базовая проверка schema.
  - `validate/internal/workspace` — scan workspace и parse frontmatter.
  - `validate/internal/engine` — pipeline проверок и агрегация issues.
  - `validate/internal/model` — внутренние типы команды.
  - `validate/internal/support` — pure helper-функции.
  - `query/handler.go` — use-case `query` (текущий прототип).
  - `add/handler.go` — scaffold use-case `add`.
  - `update/handler.go` — scaffold use-case `update`.
- `internal/contracts`
  - `requests` — входные контракты команд.
  - `responses` — выходные контракты команд.
  - `capabilities` — capability payload.
- `internal/domain`
  - `errors` — единый каталог error codes/exit codes.
  - `validation` — доменная модель validation issue.
- `internal/output`
  - `jsonwriter` — JSON вывод.
  - `ndjsonwriter` — NDJSON вывод.
  - `errormap` — map domain error -> result_state.
- `tests/integration`
  - `run_cases_test.go` — интеграционный раннер data-first кейсов.
  - `cases/validate/*` — кейсы для команды `validate`.

## Текущий статус команд

- `validate` — реализована в MVP.
- `query` — прототип ответа без полноценного движка запросов.
- `add` — scaffold, возвращает `NOT_IMPLEMENTED`.
- `update` — scaffold, возвращает `NOT_IMPLEMENTED`.
