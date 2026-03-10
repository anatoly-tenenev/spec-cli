# Индекс кодовой базы (Agent Map)

Короткая карта проекта для быстрого входа в код.

## Правило актуальности

- Любые изменения структуры директорий, entrypoint-файлов, ролей слоёв или состава команд должны сопровождаться обновлением этого файла в том же изменении.

## Быстрый маршрут выполнения CLI

1. `internal/cli/app.go` — `NewApp`, `(*App).Run`.
2. `internal/application/commandbus/bus.go` — `(*Bus).Dispatch`.
3. `internal/application/commands/<command>/handler.go` — `(*Handler).Handle`.
4. Для `validate`: `options.Parse` -> `schema.Load` -> `workspace.BuildCandidateSet` -> `engine.RunValidation`.

## Состояние Binary Entrypoint

- Директория `cmd/spec-cli` существует, но в текущем состоянии не содержит Go-файлов и функции `main`.
- Команды `make build`/`make run` уже нацелены на `./cmd/spec-cli`; для рабочего бинарника нужен `cmd/spec-cli/main.go` с вызовом `cli.NewApp(...).Run(...)`.

## Карта слоёв

- `internal/cli`
  - Entry point: `internal/cli/app.go` — `NewApp`, `(*App).Run`.
  - Ответственность:
    - Сборка приложения и регистрация handlers (`validate`, `query`, `add`, `update`) через command bus.
    - Парсинг глобальных опций CLI (`--format`, `--workspace`, `--schema`, `--config`, `--require-absolute-paths`, `--verbose`).
    - Единый рендеринг успешных и ошибочных ответов в `json`.
  - Подпакеты: отсутствуют.

- `internal/application/commandbus`
  - Entry point: `internal/application/commandbus/bus.go` — `New`, `(*Bus).Register`, `(*Bus).Dispatch`.
  - Ответственность:
    - Реестр command handlers по имени команды.
    - Dispatch запроса в конкретный handler.
    - Возврат доменной ошибки `INVALID_ARGS` для неизвестной команды.
  - Подпакеты: отсутствуют.

- `internal/application/commands/validate`
  - Entry point: `internal/application/commands/validate/handler.go` — `NewHandler`, `(*Handler).Handle`.
  - Ответственность:
    - Оркестрация пайплайна `validate`: parse options -> normalize paths -> load schema -> scan workspace -> run engine.
    - Объединение schema issues и instance issues в единый результат.
    - Формирование контрактного `json`-ответа и `ExitCode` с учётом `--warnings-as-errors`.
  - Подпакеты:
    - `validate/internal/options` — parse/norm опций команды.
    - `validate/internal/schema` — загрузка и compile-time проверка схемы.
    - `validate/internal/workspace` — scan markdown workspace и parse frontmatter/content.
    - `validate/internal/expressions` — compiler/evaluator для выражений.
    - `validate/internal/engine` — runtime validation pipeline.
    - `validate/internal/model` — внутренние структуры состояния команды.
    - `validate/internal/support` — pure helpers для YAML/коллекций/типов.

- `internal/application/commands/validate/internal/options`
  - Entry point: `internal/application/commands/validate/internal/options/parse.go` — `Parse`; `internal/application/commands/validate/internal/options/paths.go` — `NormalizePaths`.
  - Ответственность:
    - Парсинг флагов команды `validate` (`--type`, `--fail-fast`, `--warnings-as-errors`).
    - Нормализация `workspace/schema` путей в абсолютные.
    - Валидация режима `--require-absolute-paths` и возврат `INVALID_ARGS` при нарушениях.
  - Подпакеты: отсутствуют.

- `internal/application/commands/validate/internal/schema`
  - Entry point: `internal/application/commands/validate/internal/schema/loader.go` — `Load`.
  - Ответственность:
    - Чтение schema-файла и parse YAML/JSON в AST.
    - Проверка root mapping, duplicate keys и closed-world верхнего уровня (`version/entity/description`).
    - Ранний fail-fast на `SchemaError` уровня `error` с возвратом `SCHEMA_INVALID` до runtime-прохода.
    - Делегирование разбора `entity.<type>` в подпакет entity.
  - Подпакеты:
    - `validate/internal/schema/internal/entity` — разбор правил на уровне типа.

- `internal/application/commands/validate/internal/schema/internal/entity`
  - Entry point: `internal/application/commands/validate/internal/schema/internal/entity/parser.go` — `ParseType`.
  - Ответственность:
    - Парсинг `id_prefix` и контроль уникальности префиксов между типами.
    - Проверка closed-world ключей для `entity.<type>`, `meta`, `meta.fields[]`, `meta.fields[].schema`, `content`, `content.sections[]`.
    - Парсинг `meta.fields` (`type`, `enum`, `const`, `refTypes`, `required_when`).
    - Парсинг `content.sections` и условий обязательности.
    - Статическая проверка strict-операторов `eq/in` в `required_when` на potentially-missing операндах.
    - Сборка expression context и агрегация schema issues.
  - Подпакеты:
    - `validate/internal/schema/internal/entity/internal/pathpattern` — разбор и проверки `path_pattern`.
    - `validate/internal/schema/internal/entity/internal/schemachecks` — переиспользуемые schema-check helpers (closed-world keysets, strict required_when missing analysis).

- `internal/application/commands/validate/internal/schema/internal/entity/internal/pathpattern`
  - Entry point: `internal/application/commands/validate/internal/schema/internal/entity/internal/pathpattern/parser.go` — `Parse`.
  - Ответственность:
    - Нормализация `path_pattern` из форм `string | list | object`.
    - Проверка closed-world ключей `path_pattern`-объекта и `cases[]`.
    - Compile-time разбор `cases[].when` выражений.
    - Проверка placeholders (`meta:*`, `ref:*`), strict-операторов в `when`, статических `exists` guard и правила единственного unconditional-case в конце.
  - Подпакеты: отсутствуют.

- `internal/application/commands/validate/internal/workspace`
  - Entry point: `internal/application/commands/validate/internal/workspace/candidates.go` — `BuildCandidateSet`; `internal/application/commands/validate/internal/workspace/frontmatter.go` — `ParseFrontmatter`.
  - Ответственность:
    - Детерминированный scan `.md` файлов workspace и фильтрация по `--type`.
    - Разбор frontmatter с контролем duplicate keys.
    - Извлечение labels секций и определение дубликатов заголовков.
  - Подпакеты: отсутствуют.

- `internal/application/commands/validate/internal/expressions`
  - Entry point: `internal/application/commands/validate/internal/expressions/compiler.go` — `Compile`; `internal/application/commands/validate/internal/expressions/evaluator.go` — `Evaluate`.
  - Ответственность:
    - Компиляция expression AST (`eq`, `eq?`, `in`, `in?`, `all`, `any`, `not`, `exists`) с проверкой arity/type/reference.
    - Валидация ссылок на `meta/ref` в compile context.
    - Runtime evaluation strict/safe операторов и reporting evaluation errors.
  - Подпакеты: отсутствуют.

- `internal/application/commands/validate/internal/engine`
  - Entry point: `internal/application/commands/validate/internal/engine/runner.go` — `RunValidation`; `internal/application/commands/validate/internal/engine/issues.go` — `CountIssuesByLevel`.
  - Ответственность:
    - Полный runtime pipeline: parse кандидатов, built-in проверки, schema-driven проверки.
    - Resolve `entity_ref`, оценка `required_when`, валидация `path_pattern`.
    - Агрегация entity/global issues, подсчёт метрик coverage/validity/conformance.
  - Подпакеты: отсутствуют.

- `internal/application/commands/query`
  - Entry point: `internal/application/commands/query/handler.go` — `NewHandler`, `(*Handler).Handle`.
  - Ответственность:
    - Обработка прототипа команды `query`.
    - Возврат стабильного пустого результата (`items`, `page`) в `json`.
    - Валидация неподдержанных аргументов (`--help`).
  - Подпакеты: отсутствуют.

- `internal/application/commands/add`
  - Entry point: `internal/application/commands/add/handler.go` — `NewHandler`, `(*Handler).Handle`.
  - Ответственность:
    - Scaffold команды `add`.
    - Возврат доменной ошибки `NOT_IMPLEMENTED`.
  - Подпакеты: отсутствуют.

- `internal/application/commands/update`
  - Entry point: `internal/application/commands/update/handler.go` — `NewHandler`, `(*Handler).Handle`.
  - Ответственность:
    - Scaffold команды `update`.
    - Возврат доменной ошибки `NOT_IMPLEMENTED`.
  - Подпакеты: отсутствуют.

- `internal/contracts/requests`
  - Entry point: `internal/contracts/requests/options.go` — публичный API типов `OutputFormat`, `GlobalOptions`, `Command` (функции отсутствуют).
  - Ответственность:
    - Контракт глобальных опций и командного запроса.
    - Единый enum форматов вывода (`json`).
    - Передача запроса между CLI, bus и handlers.
  - Подпакеты: отсутствуют.

- `internal/contracts/responses`
  - Entry point: `internal/contracts/responses/types.go` — публичный API типов `ResultState`, `CommandOutput` (функции отсутствуют).
  - Ответственность:
    - Контракт ответа команды для `json`.
    - Единый enum `result_state`.
    - Транспорт `ExitCode` из use-case в CLI.
  - Подпакеты: отсутствуют.

- `internal/contracts/capabilities`
  - Entry point: `internal/contracts/capabilities/default.go` — публичная переменная `Default`.
  - Ответственность:
    - Декларация поддерживаемых команд.
    - Декларация поддерживаемых форматов вывода.
  - Подпакеты: отсутствуют.

- `internal/domain/errors`
  - Entry point: `internal/domain/errors/codes.go` — `New`, `ExitCodeFor`.
  - Ответственность:
    - Единый каталог доменных кодов ошибок.
    - Формирование `AppError` с code/message/details/exit_code.
    - Маппинг доменных кодов в процессные exit codes.
  - Подпакеты: отсутствуют.

- `internal/domain/validation`
  - Entry point: `internal/domain/validation/issue.go` — публичный API типов `IssueLevel`, `Entity`, `Issue` (функции отсутствуют).
  - Ответственность:
    - Доменная модель validation issue.
    - Единый shape для ошибок/предупреждений в контракте ответа.
  - Подпакеты: отсутствуют.

- `internal/output/jsonwriter`
  - Entry point: `internal/output/jsonwriter/writer.go` — `New`, `(*Writer).Write`.
  - Ответственность:
    - Запись одного JSON payload в `io.Writer`.
    - Отключение HTML-escaping для machine-stable вывода.
  - Подпакеты: отсутствуют.

- `internal/output/errormap`
  - Entry point: `internal/output/errormap/mapper.go` — `ResultStateForCode`.
  - Ответственность:
    - Маппинг domain error code в `result_state` контракта.
    - Централизация политики отображения ошибок для output слоя.
  - Подпакеты: отсутствуют.

- `tests/integration`
  - Entry point: `tests/integration/run_cases_test.go` — `TestValidateCases`.
  - Ответственность:
    - Data-first запуск интеграционных кейсов `validate` из двухуровневой структуры `tests/integration/cases/validate/<group>/<NNNN_outcome_case-id>`.
    - Детерминированный обход групп и кейсов (лексикографическая сортировка на каждом уровне).
    - Валидация соглашения нейминга `NNNN_ok_*` / `NNNN_err_*` с проверкой соответствия `expect.exit_code` и `case.json.id`.
    - Подготовка временного workspace/schema и запуск приложения через `cli.NewApp(...).Run(...)`.
    - Проверка `exit_code`, `stderr` и `json`-ответа против golden-ожиданий.
  - Подпакеты:
    - `tests/integration/cases/validate/10_contract/*` — контрактные сценарии (`json`, `warnings-as-errors`, exit code).
    - `tests/integration/cases/validate/20_schema/*` — schema-level сценарии и ошибки загрузки/валидации схемы.
    - `tests/integration/cases/validate/30_instance_builtin/*` — built-in проверки entity и fail-fast/фильтрация по type.
    - `tests/integration/cases/validate/40_instance_meta_content/*` — проверки `meta.fields` и `content.sections` с `required_when`.
    - `tests/integration/cases/validate/50_path_pattern_expr/*` — сценарии `path_pattern.cases[].when` и strict/safe семантики.
    - `tests/integration/cases/validate/60_entity_ref_context/*` — сценарии `entity_ref`, `ref.*`, `ref.dir_path`.
    - `tests/integration/cases/validate/70_global_uniqueness/*` — глобальные проверки уникальности (`slug`/`id`) и смешанные invalid-сценарии.

## Текущий статус команд

- `validate` — расширена поддержка `expressions`, `entity_ref` и `path_pattern.cases[].when` (json-контракт).
- `query` — прототип ответа без полноценного движка запросов.
- `add` — scaffold, возвращает `NOT_IMPLEMENTED`.
- `update` — scaffold, возвращает `NOT_IMPLEMENTED`.
