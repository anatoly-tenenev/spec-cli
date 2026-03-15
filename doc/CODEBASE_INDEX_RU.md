# Индекс кодовой базы (Agent Map)

Короткая карта проекта для быстрого входа в код.

## Правило актуальности

- Любые изменения структуры директорий, entrypoint-файлов, ролей слоёв или состава команд должны сопровождаться обновлением этого файла в том же изменении.

## Быстрый маршрут выполнения CLI

1. `internal/cli/app.go` — `NewApp`, `(*App).Run`.
2. `internal/application/commandbus/bus.go` — `(*Bus).Dispatch`.
3. `internal/application/commands/<command>/handler.go` — `(*Handler).Handle`.
4. Для `validate`: `options.Parse` -> `schema.Load` -> `workspace.BuildCandidateSet` -> `engine.RunValidation`.
5. Для `query`: `options.Parse` -> `schema.Load` -> `schema.BuildIndex` -> `engine.BuildPlan` -> `workspace.LoadEntities` -> `engine.Execute`.
6. Для `get`: `options.Parse` -> `schema.LoadReadModel` -> `engine.BuildSelectorPlan` -> `workspace.LocateByID` -> `workspace.ReadTarget` -> `engine.BuildEntityView` -> `engine.ProjectEntity`.
7. Для `add`: `options.Parse` -> `options.NormalizePaths` -> `schema.Load` -> `workspace.BuildSnapshot` -> `engine.Execute`.
8. Для `update`: `options.Parse` -> `options.NormalizePaths` -> `schema.Load` -> `workspace.BuildSnapshot` -> `engine.Execute`.
9. Для `delete`: `options.Parse` -> `options.NormalizePaths` -> `schema.Load` -> `workspace.BuildSnapshot` -> `engine.Execute`.

## Состояние Binary Entrypoint

- Entry point бинарника: `cmd/spec-cli/main.go` — `main`.
- `main` инициализирует `cli.NewApp(os.Stdout, os.Stderr, time.Now)` и завершает процесс через `os.Exit(app.Run(context.Background(), os.Args[1:]))`.
- Команды `make build`/`make run` используют пакет `./cmd/spec-cli` и собирают единый бинарник `spec-cli`.

## Карта слоёв

- `internal/cli`
  - Entry point: `internal/cli/app.go` — `NewApp`, `(*App).Run`.
  - Ответственность:
    - Сборка приложения и регистрация handlers (`validate`, `query`, `get`, `add`, `update`, `delete`) через command bus.
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
    - Проверка closed-world ключей на уровне `entity.<type>` (`id_prefix`, `path_pattern`, `meta`, `content`, `description`).
    - Парсинг `id_prefix` и контроль уникальности префиксов между типами.
    - Оркестрация разбора `meta.fields`, `content.sections`, `path_pattern` через специализированные подпакеты.
    - Сборка map полей и compile context для выражений и агрегация schema issues из подпакетов.
  - Подпакеты:
    - `validate/internal/schema/internal/entity/internal/metafields` — parse и compile-time проверки `meta.fields` и `meta.fields[].schema`.
    - `validate/internal/schema/internal/entity/internal/sections` — parse и compile-time проверки `content.sections`.
    - `validate/internal/schema/internal/entity/internal/requiredconstraint` — единый разбор `required`/`required_when` (bool или expression).
    - `validate/internal/schema/internal/entity/internal/expressioncontext` — сборка compile context для выражений и маппинг compile issues в schema issues.
    - `validate/internal/schema/internal/entity/internal/names` — валидация имен schema-ключей для `meta.fields` и `content.sections`.
    - `validate/internal/schema/internal/entity/internal/pathpattern` — разбор и проверки `path_pattern`.
    - `validate/internal/schema/internal/entity/internal/schemachecks` — переиспользуемые schema-check helpers (closed-world keysets, strict required_when missing analysis).

- `internal/application/commands/validate/internal/schema/internal/entity/internal/metafields`
  - Entry point: `internal/application/commands/validate/internal/schema/internal/entity/internal/metafields/parser.go` — `Parse`.
  - Ответственность:
    - Проверка структуры `meta` и closed-world ключей (`meta`, `meta.fields[]`, `meta.fields[].schema`).
    - Парсинг field schema (`type`, `enum`, `const`, `refTypes`, `items`, `uniqueItems`, `minItems`, `maxItems`) с type-check и валидацией ссылок на entity types.
    - Разбор `required`/`required_when` для каждого поля через `requiredconstraint.Parse`.
    - Статическая проверка strict-операторов `eq/in` в `required_when` на potentially-missing операндах.
  - Подпакеты: отсутствуют.

- `internal/application/commands/validate/internal/schema/internal/entity/internal/sections`
  - Entry point: `internal/application/commands/validate/internal/schema/internal/entity/internal/sections/parser.go` — `Parse`.
  - Ответственность:
    - Проверка структуры `content.sections` и closed-world ключей section-правил.
    - Парсинг section-правил (`title`, `required`, `required_when`) с валидацией форматов.
    - Разбор `required`/`required_when` для секций через `requiredconstraint.Parse`.
    - Статическая проверка strict-операторов `eq/in` в `required_when` на potentially-missing операндах.
  - Подпакеты: отсутствуют.

- `internal/application/commands/validate/internal/schema/internal/entity/internal/requiredconstraint`
  - Entry point: `internal/application/commands/validate/internal/schema/internal/entity/internal/requiredconstraint/parser.go` — `Parse`.
  - Ответственность:
    - Единый parse правил обязательности `required` и `required_when` для полей/секций.
    - Компиляция expression-формы `required_when` и возврат compile issues в schema issue-формате.
    - Применение инвариантов `required=true` + `required_when` (ошибка) и implicit `required=false` при `required_when` без явного `required` (warning).
  - Подпакеты: отсутствуют.

- `internal/application/commands/validate/internal/schema/internal/entity/internal/expressioncontext`
  - Entry point: `internal/application/commands/validate/internal/schema/internal/entity/internal/expressioncontext/context.go` — `Build`, `FromCompileIssue`, `IsBuiltinMetaField`.
  - Ответственность:
    - Описание builtin meta-полей для compile context (`type`, `id`, `slug`, `created_date`, `updated_date`).
    - Сборка `expressions.CompileContext` из builtin meta и schema-defined `meta.fields`.
    - Маппинг compile issues expressions-компилятора в доменный формат schema issues.
  - Подпакеты: отсутствуют.

- `internal/application/commands/validate/internal/schema/internal/entity/internal/names`
  - Entry point: `internal/application/commands/validate/internal/schema/internal/entity/internal/names/validate.go` — `ValidateMetaFieldName`, `ValidateSectionName`.
  - Ответственность:
    - Валидация регулярного формата имен `meta.fields` и `content.sections`.
    - Запрет переопределения builtin meta-полей в `meta.fields`.
  - Подпакеты: отсутствуют.

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
    - Оркестрация пайплайна `query`: parse options -> normalize paths -> load schema -> build schema index -> build query plan -> load workspace views -> execute query.
    - Формирование контрактного `json`-ответа (`items`, `matched`, `page`) и `help`-ответа для `query --help`.
    - Единый вход для маппинга доменных ошибок (`INVALID_ARGS`, `INVALID_QUERY`, `ENTITY_TYPE_UNKNOWN`, schema/read errors).
  - Подпакеты:
    - `query/internal/options` — parse/norm опций команды (`--type`, `--where-json`, `--select`, `--sort`, `--limit`, `--offset`).
    - `query/internal/schema` — загрузка стандартной схемы (`schema.entity`) и построение `QuerySchemaIndex` (selectors/sort/filter/type metadata).
    - `query/internal/workspace` — scan `.md`, parse frontmatter/content, build full read-view и resolve `refs.<field>`.
    - `query/internal/engine` — planner (select/sort/filter bind), evaluator `where-json`, сортировка, пагинация, projection, help text.
    - `query/internal/model` — внутренние структуры запроса/плана/индекса/AST/ответа.
    - `query/internal/support` — pure helper-функции для YAML, коллекций, literal/value операций.

- `internal/application/commands/query/internal/options`
  - Entry point: `internal/application/commands/query/internal/options/parse.go` — `Parse`; `internal/application/commands/query/internal/options/paths.go` — `NormalizePaths`.
  - Ответственность:
    - Парсинг query-опций, defaults (`limit=100`, `offset=0`) и базовая валидация синтаксиса `--sort`.
    - Централизованный возврат `INVALID_ARGS` для неизвестных/неполных аргументов.
    - Нормализация `workspace/schema` путей в абсолютные с учётом `--require-absolute-paths`.
  - Подпакеты: отсутствуют.

- `internal/application/commands/query/internal/schema`
  - Entry point: `internal/application/commands/query/internal/schema/loader.go` — `Load`; `internal/application/commands/query/internal/schema/index.go` — `BuildIndex`.
  - Ответственность:
    - Чтение schema-файла, parse YAML/JSON, duplicate-key checks и минимальная валидация shape `entity`.
    - Парсинг metadata/read/content для каждого entity type.
    - Построение `QuerySchemaIndex`: допустимые `entity_type`, selectors, sort/filter leaf-поля, типы полей и enum-ограничения.
    - Валидация read-selectors (`meta.*`, `refs.<field>.*`, `content.sections.*`) и конфликтов типов между entity types.
  - Подпакеты: отсутствуют.

- `internal/application/commands/query/internal/workspace`
  - Entry point: `internal/application/commands/query/internal/workspace/loader.go` — `LoadEntities`.
  - Ответственность:
    - Детерминированный scan `.md` файлов workspace и parse entity frontmatter/body.
    - Формирование full read-view (`type/id/slug/revision/created_date/updated_date/meta/refs/content.raw/content.sections`).
    - Построение глобального `id`-индекса и resolve `refs.<field>` в expanded object (`type/id/slug`).
    - Нормализация YAML значений (`time.Time` -> `YYYY-MM-DD`) и вычисление opaque `revision` (`sha256:<hex>`).
  - Подпакеты: отсутствуют.

- `internal/application/commands/query/internal/engine`
  - Entry point: `internal/application/commands/query/internal/engine/planner.go` — `BuildPlan`.
  - Ответственность:
    - Валидация type filters (`ENTITY_TYPE_UNKNOWN`) и build select-tree projector.
    - Parse -> bind typed AST для `--where-json` с проверкой `field/op/value`, type compatibility и enum.
    - Build effective sort (default + hidden tail), deterministic sorting с обработкой отсутствующих значений.
    - Выполнение pipeline filter/sort/paginate/project и сборка page-метаданных (`matched`, `returned`, `has_more`, `next_offset`, `effective_sort`).
    - Генерация подробного help-текста `query` в фиксированных секциях `Command/Syntax/Options/Rules/Examples/Schema`.
  - Подпакеты: отсутствуют.

- `internal/application/commands/get`
  - Entry point: `internal/application/commands/get/handler.go` — `NewHandler`, `(*Handler).Handle`.
  - Ответственность:
    - Оркестрация пайплайна `get`: parse options -> normalize paths -> load schema read-model -> validate selectors -> locate target by `id` -> read target -> build read-view -> project JSON.
    - Формирование контрактного `json`-ответа (`result_state`, `target`, `entity`) для single-entity read.
    - Единая обработка доменных ошибок (`INVALID_ARGS`, `SCHEMA_*`, `ENTITY_NOT_FOUND`, `TARGET_AMBIGUOUS`, `READ_FAILED`).
  - Подпакеты:
    - `get/internal/options` — parse/norm опций команды (`--id`, повторяемый `--select`) и нормализация путей.
    - `get/internal/schema` — загрузка схемы и построение read-model (`entity types`, `meta/ref/sections`, `allowed selectors`) с schema-ошибками.
    - `get/internal/workspace` — deterministic scan `.md`, locator target по быстрому извлечению `id`, strict parse target frontmatter/body, extraction `content.sections`, `revision`.
    - `get/internal/engine` — selector plan/tree, projection с absent-policy и special-case `content.sections.<name> = null`, build read-view (`meta`, `refs`, `content`), blocking-policy для запрошенных `refs/sections`.
    - `get/internal/model` — внутренние структуры опций, read-model, selector plan и parsed target.
    - `get/internal/support` — pure helper-функции для YAML/maps/deep-copy/validation-issues.

- `internal/application/commands/get/internal/options`
  - Entry point: `internal/application/commands/get/internal/options/parse.go` — `Parse`; `internal/application/commands/get/internal/options/paths.go` — `NormalizePaths`.
  - Ответственность:
    - Парсинг get-опций (`--id`, `--select`) и обязательность `--id`.
    - Валидация неизвестных/неполных аргументов с `INVALID_ARGS`.
    - Нормализация `workspace/schema` путей в абсолютные с учётом `--require-absolute-paths`.
  - Подпакеты: отсутствуют.

- `internal/application/commands/get/internal/schema`
  - Entry point: `internal/application/commands/get/internal/schema/loader.go` — `LoadReadModel`.
  - Ответственность:
    - Чтение schema-файла, parse YAML/JSON, duplicate-key checks и базовая валидация shape `entity`.
    - Построение read-model по `schema.entity`: `meta fields`, `entity_ref fields`, `content.sections` для каждого типа.
    - Формирование канонического whitelist selector-ов (`built-in`, `meta.*`, `refs.*`, `content.raw`, `content.sections.*`).
    - Возврат `SCHEMA_NOT_FOUND|SCHEMA_PARSE_ERROR|SCHEMA_INVALID` с `validation.issues`.
  - Подпакеты: отсутствуют.

- `internal/application/commands/get/internal/workspace`
  - Entry point: `internal/application/commands/get/internal/workspace/reader.go` — `LocateByID`, `ReadTarget`.
  - Ответственность:
    - Детерминированный scan `.md` файлов workspace и локатор target по точному `id` без полной валидации workspace.
    - Разделение статусов locator-а: not found (`ENTITY_NOT_FOUND`), ambiguous (`TARGET_AMBIGUOUS`), single target.
    - Strict parse target frontmatter/body, проверка `type/id`, вычисление `revision` и extraction sections.
    - Построение `id`-индекса parseable сущностей для расширения `refs`.
  - Подпакеты: отсутствуют.

- `internal/application/commands/get/internal/engine`
  - Entry point: `internal/application/commands/get/internal/engine/plan.go` — `BuildSelectorPlan`; `internal/application/commands/get/internal/engine/entity.go` — `BuildEntityView`.
  - Ответственность:
    - Валидация selector-ов по schema read-model и построение терминального select-tree.
    - Классификация требований запроса (`refs`, `content.raw`, `content.sections`, required leafs) и null-policy для `content.sections.<name>`.
    - Сбор read-view target-сущности (`meta`, expanded `refs`, `content`) с blocking-policy для невычислимых запрошенных `refs/sections`.
    - Projection поддерева ответа без включения невыбранных веток.
  - Подпакеты: отсутствуют.

- `internal/application/commands/get/internal/model`
  - Entry point: `internal/application/commands/get/internal/model/types.go` — публичные для пакета типы модели (функции отсутствуют).
  - Ответственность:
    - Типы опций команды (`Options`).
    - Типы selector-плана (`SelectNode`, `SelectorPlan`).
    - Типы schema read-model (`ReadModel`, `EntityTypeSpec`) и workspace read (`LocateResult`, `ParsedTarget`, `EntityIdentity`).
  - Подпакеты: отсутствуют.

- `internal/application/commands/get/internal/support`
  - Entry point: `internal/application/commands/get/internal/support/yaml.go` — `FirstContentNode`, `FindDuplicateMappingKey`, `ToStringMap`; `internal/application/commands/get/internal/support/collections.go` — `SortedMapKeys`; `internal/application/commands/get/internal/support/values.go` — `DeepCopy`, `ValidationIssue`, `WithValidationIssues`.
  - Ответственность:
    - Вспомогательные функции для YAML AST и duplicate-key checks.
    - Стабильная сортировка map-ключей.
    - Deep-copy и сборка `validation.issues` payload для доменных ошибок.
  - Подпакеты: отсутствуют.

- `internal/application/commands/add`
  - Entry point: `internal/application/commands/add/handler.go` — `NewHandler`, `(*Handler).Handle`.
  - Ответственность:
    - Оркестрация пайплайна `add`: parse options -> normalize paths -> load raw schema -> build workspace snapshot -> execute candidate build/validation/write.
    - Преобразование доменных ошибок `INVALID_ARGS`, `WRITE_CONTRACT_VIOLATION`, `VALIDATION_FAILED`, `PATH_CONFLICT`, schema/read-write ошибок в единый JSON envelope.
    - Передача внедряемого `Clock` для детерминированной генерации `created_date`/`updated_date`.
  - Подпакеты:
    - `add/internal/options` — parse/norm опций `add` (`--type`, `--slug`, `--set`, `--set-file`, `--content-file`, `--content-stdin`, `--dry-run`) с конфликтами, дубликатами path и absolute-path policy.
    - `add/internal/schema` — загрузка raw schema и построение локального write-контракта для `add` (`meta.*`, `refs.*`, `content.sections.*`, `set-file allowlist`, `path_pattern`, `meta/content rules`).
    - `add/internal/workspace` — deterministic snapshot workspace (`id`/`slug`/suffix индексы, existing paths, parseable entity identities, `dir_path` контекст для `entity_ref`).
    - `add/internal/engine` — применение write-операций, typed YAML parsing для `--set`, canonical body/frontmatter serialization, `path_pattern`/expression evaluation, full candidate validation, `revision`, dry-run и атомарная запись.
    - `add/internal/model` — внутренние типы use-case (`Options`, `WriteOperation`, schema/snapshot/candidate модели).
    - `add/internal/support` — helpers для YAML AST, deep copy/literal compare, stable collections и error-details `validation.issues`.

- `internal/application/commands/add/internal/options`
  - Entry point: `internal/application/commands/add/internal/options/parse.go` — `Parse`; `internal/application/commands/add/internal/options/paths.go` — `NormalizePaths`.
  - Ответственность:
    - Парсинг и базовая валидация аргументов `add` (`--type`, `--slug`, `--set`, `--set-file`, `--content-file`, `--content-stdin`, `--dry-run`).
    - Выявление request-level конфликтов (`whole-body` vs `content.sections.*`, дубли path, взаимоисключающие флаги).
    - Нормализация `workspace/schema` и файловых path аргументов с учетом `--require-absolute-paths`.
  - Подпакеты: отсутствуют.

- `internal/application/commands/add/internal/schema`
  - Entry point: `internal/application/commands/add/internal/schema/loader.go` — `Load`.
  - Ответственность:
    - Чтение schema-файла, parse YAML/JSON, duplicate-key checks и top-level validation.
    - Оркестрация разбора `entity.<type>` и сборки `AddSchema`.
    - Нормативная классификация schema-ошибок в `SCHEMA_NOT_FOUND|SCHEMA_PARSE_ERROR|SCHEMA_INVALID`.
  - Подпакеты:
    - `add/internal/schema/internal/entity` — parse `id_prefix`, `path_pattern`, `meta.fields`, `content.sections` и построение локального write-контракта (`allowWritePaths`, `allowSetFilePaths`) для `add`.

- `internal/application/commands/add/internal/workspace`
  - Entry point: `internal/application/commands/add/internal/workspace/snapshot.go` — `BuildSnapshot`; `internal/application/commands/add/internal/workspace/frontmatter.go` — `ParseFrontmatter`; `internal/application/commands/add/internal/workspace/sections.go` — `ExtractSections`.
  - Ответственность:
    - Детерминированный scan `.md` файлов и построение snapshot-индексов (`entitiesByID`, `slugsByType`, `maxSuffixByType`, `existingPaths`).
    - Tolerant parse frontmatter для формирования identity/meta контекста существующих сущностей.
    - Выделение labeled sections и детекция duplicate labels для full validation кандидата.
  - Подпакеты: отсутствуют.

- `internal/application/commands/add/internal/engine`
  - Entry point: `internal/application/commands/add/internal/engine/execute.go` — `Execute`.
  - Ответственность:
    - Оркестрация create-pipeline: apply writes -> build candidate -> resolve refs/path -> validate -> serialize -> write/dry-run response.
    - Единая маршрутизация доменных ошибок `WRITE_CONTRACT_VIOLATION`, `VALIDATION_FAILED`, `PATH_CONFLICT`, `WRITE_FAILED`, `INTERNAL_ERROR`.
    - Сборка контрактного payload `entity` и итогового `result_state=valid`.
  - Подпакеты:
    - `add/internal/engine/internal/writes` — применение write-операций (`--set`, `--set-file`, whole-body), typed YAML parsing, guard rails write-контракта и build markdown body.
    - `add/internal/engine/internal/refresolve` — резолв `entity_ref` по workspace snapshot и диагностика `missing|ambiguous|type_mismatch`.
    - `add/internal/engine/internal/pathcalc` — evaluation `path_pattern` (`when`-выражения, placeholder interpolation, normalization и guard от выхода за workspace).
    - `add/internal/engine/internal/validation` — full instance-validation кандидата (builtin/meta/content/global rules), сортировка issues и сборка `VALIDATION_FAILED`.
    - `add/internal/engine/internal/markdown` — canonical frontmatter/body serialization и вычисление `revision` (`sha256:*`).
    - `add/internal/engine/internal/storage` — filesystem checks (`path conflict`) и атомарная запись файла.
    - `add/internal/engine/internal/payload` — формирование публичного `entity` payload (`meta`, `refs`, `content.sections`).
    - `add/internal/engine/internal/expr` — интерпретатор schema-выражений (`exists|eq|eq?|in|in?|all|any|not`).
    - `add/internal/engine/internal/lookup` — lookup-адаптер значений кандидата для expression/path evaluation (`type`, `meta.*`, `refs.*`).
    - `add/internal/engine/internal/issues` — фабрика `domainvalidation.Issue` с entity-контекстом.

- `internal/application/commands/add/internal/model`
  - Entry point: `internal/application/commands/add/internal/model/types.go` — публичные для пакета структуры состояния команды (функции отсутствуют).
  - Ответственность:
    - Типы опций и write-операций.
    - Типы schema/snapshot/candidate моделей.
    - Общие структуры для передачи данных между `options -> schema -> workspace -> engine`.
  - Подпакеты: отсутствуют.

- `internal/application/commands/add/internal/support`
  - Entry point: `internal/application/commands/add/internal/support/yaml.go` — `FirstContentNode`, `FindDuplicateMappingKey`, `ParseYAMLValue`, `EncodeYAMLNode`; `collections.go` — `SortedMapKeys`, `SortedUniqueStrings`; `values.go` — `NormalizeValue`, `LiteralEqual`, `WithValidationIssues`.
  - Ответственность:
    - YAML AST helpers и value-level YAML parsing для typed scalar входа.
    - Нормализация/сравнение значений для validation и expression evaluation.
    - Унифицированная сборка деталей `validation.issues` для error-envelope.
  - Подпакеты: отсутствуют.

- `internal/application/commands/delete`
  - Entry point: `internal/application/commands/delete/handler.go` — `NewHandler`, `(*Handler).Handle`.
  - Ответственность:
    - Оркестрация пайплайна `delete`: parse options -> normalize paths -> load schema -> build workspace snapshot -> execute delete/checks.
    - Обработка `--help` через `engine.HelpPayload` и возврат контрактного `json`-help ответа без побочных эффектов.
    - Единая маршрутизация доменных ошибок (`INVALID_ARGS`, `SCHEMA_*`, `ENTITY_NOT_FOUND`, `AMBIGUOUS_ENTITY_ID`, `REVISION_UNAVAILABLE`, `DELETE_BLOCKED_BY_REFERENCES`, `WRITE_FAILED`).
  - Подпакеты:
    - `delete/internal/options` — parse/norm опций `delete` (`--id`, `--expect-revision`, `--dry-run`, `--help`) и absolute-path policy.
    - `delete/internal/schema` — загрузка raw schema и извлечение reference slots (`entity_ref`, `array.items.entity_ref`) для reverse-ref проверок.
    - `delete/internal/workspace` — deterministic scan `.md`, locator target по `id`, tolerant parse frontmatter для snapshot документов, вычисление `revision`.
    - `delete/internal/engine` — target lookup, optimistic concurrency (`--expect-revision`), reverse-ref blocking checks, dry-run/commit payload, help text.
    - `delete/internal/storage` — удаление target-файла и маппинг filesystem ошибок (`WRITE_FAILED`), включая test injection hook.
    - `delete/internal/model` — внутренние типы use-case (`Options`, `Schema`, `Snapshot`, `ParsedDocument`, `BlockingReference`).
    - `delete/internal/support` — pure helper-функции YAML/map для schema/workspace parsing.

- `internal/application/commands/delete/internal/options`
  - Entry point: `internal/application/commands/delete/internal/options/parse.go` — `Parse`; `internal/application/commands/delete/internal/options/paths.go` — `NormalizePaths`.
  - Ответственность:
    - Парсинг и базовая валидация аргументов `delete` (`--id`, `--expect-revision`, `--dry-run`, `--help`) с поддержкой `--flag=value`.
    - Проверка обязательности `--id` (кроме help) и нормализация boolean-значения `--dry-run`.
    - Нормализация `workspace/schema` путей с учётом `--require-absolute-paths`.
  - Подпакеты: отсутствуют.

- `internal/application/commands/delete/internal/schema`
  - Entry point: `internal/application/commands/delete/internal/schema/loader.go` — `Load`.
  - Ответственность:
    - Чтение schema-файла, parse YAML/JSON в AST и duplicate-key checks.
    - Валидация top-level shape (`version/entity/description`) и структуры `schema.entity`.
    - Извлечение reference slots по типам сущностей только для полей `entity_ref` и массивов `items.type=entity_ref`.
    - Классификация ошибок загрузки/парсинга/валидации в `SCHEMA_NOT_FOUND|SCHEMA_PARSE_ERROR|SCHEMA_INVALID`.
  - Подпакеты: отсутствуют.

- `internal/application/commands/delete/internal/workspace`
  - Entry point: `internal/application/commands/delete/internal/workspace/index.go` — `BuildSnapshot`, `FindTargetDocument`.
  - Ответственность:
    - Детерминированный scan `.md` файлов workspace и сбор baseline snapshot parseable сущностей (`type/id/revision/frontmatter`).
    - Быстрый locate target-кандидатов по exact `id` с tolerant fallback (`yaml parse` -> line-based extraction).
    - Нормализация YAML значений (`time.Time` -> `YYYY-MM-DD`) и вычисление opaque `revision` (`sha256:<hex>`).
    - Маппинг ошибок scan/read в `READ_FAILED` с нормализованными `reason`.
  - Подпакеты: отсутствуют.

- `internal/application/commands/delete/internal/engine`
  - Entry point: `internal/application/commands/delete/internal/engine/execute.go` — `Execute`; `internal/application/commands/delete/internal/engine/help.go` — `HelpPayload`.
  - Ответственность:
    - Разрешение target по `snapshot.TargetMatches` с диагностикой `ENTITY_NOT_FOUND`, `AMBIGUOUS_ENTITY_ID`, `REVISION_UNAVAILABLE`.
    - Проверка optimistic concurrency через `--expect-revision` и возврат `CONCURRENCY_CONFLICT`.
    - Поиск блокирующих incoming ссылок (`scalar/array entity_ref`) по schema slots и build стабильного списка `blocking_refs`.
    - Выполнение `dry-run` или реального удаления через storage и формирование контрактного payload (`result_state`, `dry_run`, `deleted`, `target.id/revision`).
    - Генерация help payload/text для `delete` в секциях `Command/Syntax/Options/Rules/Examples`.
  - Подпакеты: отсутствуют.

- `internal/application/commands/delete/internal/storage`
  - Entry point: `internal/application/commands/delete/internal/storage/delete.go` — `Delete`.
  - Ответственность:
    - Удаление целевого markdown-файла через `os.Remove`.
    - Маппинг причин write-ошибок в контрактный `reason` (`permission denied`, `target file is missing`, `filesystem delete failed`).
    - Поддержка test-only инъекции fs-сбоя через env `SPEC_CLI_TEST_INJECT_DELETE_FAILURE`.
  - Подпакеты: отсутствуют.

- `internal/application/commands/delete/internal/model`
  - Entry point: `internal/application/commands/delete/internal/model/types.go` — публичные для пакета структуры состояния команды (функции отсутствуют).
  - Ответственность:
    - Типы опций команды (`Options`) и schema-модели reference slots (`Schema`, `ReferenceSlot`).
    - Типы workspace snapshot (`Snapshot`, `ParsedDocument`, `TargetMatch`).
    - Типы диагностик блокирующих ссылок (`BlockingReference`) и связанные enum-константы.
  - Подпакеты: отсутствуют.

- `internal/application/commands/delete/internal/support`
  - Entry point: `internal/application/commands/delete/internal/support/yaml.go` — `FirstContentNode`, `FindDuplicateMappingKey`, `ToStringMap`; `collections.go` — `SortedMapKeys`.
  - Ответственность:
    - Вспомогательные функции для YAML AST и duplicate-key checks.
    - Безопасное приведение raw YAML decode-значений к `map[string]any`.
    - Стабильная сортировка map-ключей для детерминированного обхода schema полей.
  - Подпакеты: отсутствуют.

- `internal/application/commands/update`
  - Entry point: `internal/application/commands/update/handler.go` — `NewHandler`, `(*Handler).Handle`.
  - Ответственность:
    - Оркестрация пайплайна `update`: parse options -> normalize paths -> load schema -> build workspace snapshot -> execute update/checks.
    - Поддержка mutating-патча (`--set`, `--set-file`, `--unset`, whole-body операции) и optimistic concurrency (`--expect-revision`).
    - Возврат контрактного `json`-ответа (`updated/noop/changes/entity/validation`) с единым маппингом доменных ошибок.
  - Подпакеты:
    - `update/internal/options` — parse/norm опций `update` и конфликтов аргументов.
    - `update/internal/schema` — загрузка raw schema и построение write/read-контракта по типам.
    - `update/internal/workspace` — snapshot workspace, parse frontmatter и layout content sections.
    - `update/internal/engine` — apply writes, resolve refs/path, full validation, serialize/revision, dry-run/commit.
    - `update/internal/model` — внутренние типы use-case (`Options`, schema/snapshot/candidate модели).
    - `update/internal/support` — pure helper-функции YAML/values/collections для parse/validation.

- `internal/application/commands/update/internal/options`
  - Entry point: `internal/application/commands/update/internal/options/parse.go` — `Parse`; `internal/application/commands/update/internal/options/paths.go` — `NormalizePaths`.
  - Ответственность:
    - Парсинг `update`-флагов (`--id`, `--set`, `--set-file`, `--unset`, `--content-file`, `--content-stdin`, `--clear-content`, `--expect-revision`, `--dry-run`).
    - Валидация конфликтов аргументов (mutual exclusion whole-body флагов, дубли/конфликты write-path, обязательность `--id`, запрет пустого patch).
    - Нормализация `workspace/schema`, `--content-file` и `--set-file` путей с учетом `--require-absolute-paths`.
  - Подпакеты: отсутствуют.

- `internal/application/commands/update/internal/schema`
  - Entry point: `internal/application/commands/update/internal/schema/loader.go` — `Load`.
  - Ответственность:
    - Чтение schema-файла, parse YAML/JSON, deep duplicate-key checks и top-level валидация shape `version/entity/description`.
    - Построение `Schema.EntityTypes` для `update` (meta/content/path_pattern/write-contract).
    - Классификация ошибок загрузки/парсинга/валидации в `SCHEMA_NOT_FOUND|SCHEMA_PARSE_ERROR|SCHEMA_INVALID` с `validation.issues`.
  - Подпакеты:
    - `update/internal/schema/internal/entity` — parse `entity.<type>` (id_prefix/path_pattern/meta.fields/content.sections, allowSet/Unset/SetFile paths, required/required_when, enum/const/refTypes/items-constraints).

- `internal/application/commands/update/internal/schema/internal/entity`
  - Entry point: `internal/application/commands/update/internal/schema/internal/entity/parser.go` — `ParseType`.
  - Ответственность:
    - Разбор и валидация `id_prefix` с контролем уникальности между типами.
    - Разбор `path_pattern` (string/list/object+cases/when) с требованием fallback-case.
    - Разбор `meta.fields` и `content.sections` (order-aware parsing по YAML AST, required/required_when, title aliases, enum/const/array/items/refTypes).
    - Построение write-контракта типа (`AllowSetPaths`, `AllowUnsetPaths`, `AllowSetFilePaths`) для downstream `writes.Apply`.
  - Подпакеты: отсутствуют.

- `internal/application/commands/update/internal/workspace`
  - Entry point: `internal/application/commands/update/internal/workspace/snapshot.go` — `BuildSnapshot`; `frontmatter.go` — `ParseFrontmatter`, `ReadStringField`, `BuildMeta`; `sections.go` — `BuildSectionLayout`, `ExtractSections`.
  - Ответственность:
    - Детерминированный scan `.md` workspace и сбор snapshot-индексов (`EntitiesByID`, `SlugsByType`, `ExistingPaths`, `TargetMatches`).
    - Быстрый locate target-кандидатов по exact `id` с tolerant fallback (`yaml parse` -> line-based extraction).
    - Parse frontmatter/body и нормализация meta-полей для reference/validation контекста.
    - Разметка и extraction labeled sections (`[Title](#label)` и `Title {#label}`), включая детекцию duplicate labels.
  - Подпакеты: отсутствуют.

- `internal/application/commands/update/internal/engine`
  - Entry point: `internal/application/commands/update/internal/engine/execute.go` — `Execute`.
  - Ответственность:
    - Target lookup + optimistic concurrency guard по persisted `revision` (`ENTITY_NOT_FOUND`, `TARGET_AMBIGUOUS`, `CONCURRENCY_CONFLICT`).
    - Применение patch-операций/whole-body, `updated_date` bump, резолв `entity_ref`, evaluation `path_pattern`.
    - Full post-update validation (builtin/meta/content/global uniqueness/path/ref), deterministic issue sorting и маппинг в `VALIDATION_FAILED`.
    - Сериализация markdown/frontmatter, пересчёт `revision`, dry-run или commit (atomic write / move) и сборка `changes`/`entity` payload.
  - Подпакеты:
    - `update/internal/engine/internal/writes` — preflight write-контракта, typed YAML parsing (`--set`/`--set-file`), section/body patching и deterministic `changes` diff.
    - `update/internal/engine/internal/refresolve` — резолв scalar `entity_ref` c проверками `missing|ambiguous|type_mismatch`.
    - `update/internal/engine/internal/pathcalc` — выбор `path_pattern` case по `when`, placeholder interpolation и guard от выхода за workspace.
    - `update/internal/engine/internal/validation` — built-in/meta/content/global rules, required_when evaluation, section-title checks, `AsAppError`.
    - `update/internal/engine/internal/storage` — atomic write/move, rollback при rename-fail, path-conflict check, test injection hook `SPEC_CLI_TEST_INJECT_WRITE_FAILURE`.
    - `update/internal/engine/internal/markdown` — canonical serialization frontmatter/body и вычисление `revision` (`sha256:*`, persisted/new).
    - `update/internal/engine/internal/payload` — сбор публичного `entity` payload (`type/id/slug/revision/date/meta/refs`).
    - `update/internal/engine/internal/expr` — evaluator expression-операторов (`exists`, `eq/eq?`, `in/in?`, `all`, `any`, `not`).
    - `update/internal/engine/internal/lookup` — lookup-адаптер путей (`type/id/slug/date`, `meta.*`, `refs.*`) для expr/path evaluation.
    - `update/internal/engine/internal/issues` — фабрика `domainvalidation.Issue` с entity-контекстом.

- `internal/application/commands/update/internal/model`
  - Entry point: `internal/application/commands/update/internal/model/types.go` — публичные для пакета структуры состояния команды (функции отсутствуют).
  - Ответственность:
    - Типы patch-опций (`WriteOperation`, `BodyOperation`) и нормализованного request-состояния.
    - Типы schema read/write-модели (`EntityTypeSpec`, `MetaField`, `SectionSpec`, `PathPattern`, write-path specs).
    - Типы workspace snapshot и candidate entity (`WorkspaceEntity`, `TargetMatch`, `Candidate`, `ResolvedRef`).
  - Подпакеты: отсутствуют.

- `internal/application/commands/update/internal/support`
  - Entry point: `internal/application/commands/update/internal/support/yaml.go` — `FirstContentNode`, `FindDuplicateMappingKey`, `ToStringMap`, `ToSlice`, `ParseYAMLValue`, `EncodeYAMLNode`; `collections.go` — `SortedMapKeys`, `SortedUniqueStrings`; `values.go` — `DeepCopy`, `LiteralEqual`, `NormalizeValue`, `ValidationIssue`, `WithValidationIssues`.
  - Ответственность:
    - YAML AST helpers (включая deep duplicate-key traversal) и typed YAML value parsing.
    - Value helpers для нормализации/сравнения скаляров и коллекций в patch/validation flow.
    - Унифицированная сборка `validation.issues` payload для schema и runtime ошибок.
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
    - Декларация поддерживаемых команд (текущий `Default`: `validate`, `query`, `get`, `add`, `update`).
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
  - Entry point: `tests/integration/run_cases_test.go` — `TestValidateCases`, `TestQueryCases`, `TestGetCases`, `TestAddCases`, `TestUpdateCases`, `TestDeleteCases`; `tests/integration/delete_multirun_test.go` — `TestDeleteHappy02DryRunMatchesRealRevision`.
  - Ответственность:
    - Data-first запуск интеграционных кейсов `validate`, `query`, `get`, `add`, `update`, `delete` из структуры `tests/integration/cases/<command>/<group>/<NNNN_outcome_case-id>`.
    - Детерминированный обход групп и кейсов (лексикографическая сортировка на каждом уровне).
    - Валидация соглашения нейминга `NNNN_ok_*` / `NNNN_err_*` с проверкой соответствия `expect.exit_code`, `case.json.id` и `case.json.command`.
    - Подготовка временного workspace/schema и запуск CLI как subprocess через собранный бинарник (`exec.CommandContext`).
    - Проверка `exit_code`, `stderr` и `json`-ответа против golden-ожиданий.
    - Для mutating-сценариев проверка `workspace.out` (полный набор файлов + содержимое) против фактического состояния workspace после команды.
    - Дополнительный dynamic black-box тест эквивалентности `delete` dry-run и real-run по `target.revision` на независимых чистых workspace-копиях.
  - Подпакеты:
    - `tests/integration/cases/validate/10_contract/*` — контрактные сценарии (`json`, `warnings-as-errors`, exit code).
    - `tests/integration/cases/validate/20_schema/*` — schema-level сценарии и ошибки загрузки/валидации схемы.
    - `tests/integration/cases/validate/30_instance_builtin/*` — built-in проверки entity и fail-fast/фильтрация по type.
    - `tests/integration/cases/validate/40_instance_meta_content/*` — проверки `meta.fields` и `content.sections` с `required_when`.
    - `tests/integration/cases/validate/50_path_pattern_expr/*` — сценарии `path_pattern.cases[].when` и strict/safe семантики.
    - `tests/integration/cases/validate/60_entity_ref_context/*` — сценарии `entity_ref`, `ref.*`, `ref.dir_path`.
    - `tests/integration/cases/validate/70_global_uniqueness/*` — глобальные проверки уникальности (`slug`/`id`) и смешанные invalid-сценарии.
    - `tests/integration/cases/query/10_basic/*` — базовые сценарии `query` (default output, `--type`, ошибки аргументов).
    - `tests/integration/cases/query/20_select/*` — projection/selector сценарии (`--select`, merge, null для отсутствующих секций, invalid selector).
    - `tests/integration/cases/query/30_where/*` — `--where-json` happy/negative сценарии (ops, logical forms, type/enum validation).
    - `tests/integration/cases/query/40_sort_pagination/*` — сортировка (`effective_sort`, hidden tail) и offset-pagination границы.
    - `tests/integration/cases/get/10_contract/*` — контракт `get` (`default projection`, invalid args/selectors, schema errors, readable-invalid target).
    - `tests/integration/cases/get/20_select/*` — выборка селекторов `get` (`meta/refs/content.sections`, null-policy для отсутствующей секции).
    - `tests/integration/cases/get/30_lookup/*` — поиск target по `id` (`not found`, duplicate id, unrelated invalid docs).
    - `tests/integration/cases/get/40_blocking/*` — blocking read-failures (`frontmatter parse`, unresolved requested ref, duplicate section labels).
    - `tests/integration/cases/add/10_happy/*` — happy-path `add` (`created`, `dry-run`, deterministic `id max+1`, canonical serialization, `revision`).
    - `tests/integration/cases/add/20_args/*` — CLI-ошибки `add` (`missing required args`, конфликт whole-body и section writes).
    - `tests/integration/cases/add/30_contract/*` — write-contract ошибки (`unknown path`, `set-file` вне section paths, запрет built-in writes).
    - `tests/integration/cases/add/40_validation/*` — full validation-fail для кандидата (`required`/`required_when` на итоговой сущности).
    - `tests/integration/cases/add/50_conflict/*` — workspace-конфликт canonical path (`PATH_CONFLICT`) без изменения файловой системы.
    - `tests/integration/cases/update/10_happy/*` — happy-path `update` (meta/ref/section patch, whole-body file/stdin, clear-content, dry-run, repair invalid entity).
    - `tests/integration/cases/update/20_noop/*` — no-op сценарии (идемпотентный set/unset, `updated=false`, стабильный `revision`).
    - `tests/integration/cases/update/30_args/*` — ошибки аргументов и конфликтов patch-режимов (`--id`, duplicate paths, whole-body conflicts).
    - `tests/integration/cases/update/40_contract/*` — нарушения write-контракта (`forbidden/unknown paths`, `--set-file` ограничения, type/yaml parse mismatch).
    - `tests/integration/cases/update/50_validation/*` — пост-валидационные ошибки (`required`, enum, entity_ref target/type, dry-run validation parity).
    - `tests/integration/cases/update/60_lookup/*` — lookup ошибки target (`ENTITY_NOT_FOUND`).
    - `tests/integration/cases/update/70_concurrency/*` — optimistic concurrency сценарии (`CONCURRENCY_CONFLICT` и positive match).
    - `tests/integration/cases/update/80_fs/*` — fs ошибки path conflict и write-failure в commit фазе.
    - `tests/integration/cases/update/90_infra/*` — инфраструктурные ошибки (`READ_FAILED`, `SCHEMA_PARSE_ERROR`).
    - `tests/integration/cases/delete/10_happy/*` — happy-path `delete` (`deleted=true`, dry-run, `expect-revision`, tolerant parseable-invalid target).
    - `tests/integration/cases/delete/20_args/*` — ошибки аргументов `delete` (`--id` обязателен).
    - `tests/integration/cases/delete/30_lookup/*` — lookup-диагностики (`ENTITY_NOT_FOUND`, `AMBIGUOUS_ENTITY_ID`, `REVISION_UNAVAILABLE`).
    - `tests/integration/cases/delete/40_concurrency/*` — optimistic concurrency (`CONCURRENCY_CONFLICT` при mismatch `--expect-revision`).
    - `tests/integration/cases/delete/50_refs/*` — reverse-ref блокировки (`DELETE_BLOCKED_BY_REFERENCES`) и tolerant поведение при непарсабельных документах.
    - `tests/integration/cases/delete/60_infra/*` — инфраструктурные ошибки загрузки схемы (`SCHEMA_PARSE_ERROR`).
    - `tests/integration/cases/delete/70_fs/*` — fs-ошибки на этапе удаления (`WRITE_FAILED`) после прохождения pre-checks.
    - `tests/integration/cases/delete/80_help/*` — контракт `delete --help`.

## Текущий статус команд

- `validate` — расширена поддержка `expressions`, `entity_ref` и `path_pattern.cases[].when` (json-контракт).
- `query` — реализован read-only pipeline (index из стандартной схемы `entity`, `where-json`, projection, deterministic sort, offset pagination, json-contract).
- `get` — реализован baseline read-one pipeline (target lookup по `id`, schema-driven selectors, tolerant target read, refs/content projection, json-контракт).
- `add` — реализован baseline create pipeline (raw-schema write-contract, pre-write full validation, deterministic `id/date/revision`, dry-run и атомарная запись, json-контракт).
- `delete` — реализован baseline delete pipeline (lookup по exact `id`, `--expect-revision`, reverse-ref blocking checks, `dry-run`, fs delete и json-контракт).
- `update` — реализован baseline update pipeline (patch `--set/--set-file/--unset`, whole-body `--content-file|--content-stdin|--clear-content`, pre-commit full validation, `--expect-revision`, dry-run и атомарная запись с возможным move по `path_pattern`, json-контракт).
