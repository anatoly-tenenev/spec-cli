# AGENTS.md

## Цель проекта
`spec-cli` — machine-first CLI-утилита на Go для работы со spec-документами.
Текущий прототип обязан покрывать команды `validate`, `query`, `add`, `update` с контрактами из `doc/001-base/SPEC_UTILITY_CLI_PROTOTYPE_RU.md`.

## Технологии и baseline
- Язык: Go (`go 1.24+`).
- Формат поставки: один бинарник `spec-cli`.
- Предпочтительно использовать стандартную библиотеку.
- Внешние зависимости добавлять только при явной пользе и с обоснованием в PR/коммите.

## Архитектурные правила
- Следовать стилю `Hexagonal + Command Bus`.
- CLI-слой (`internal/cli`) только парсит аргументы и роутит команды.
- Use-case логика в `internal/application/commands/<command>`.
- Доменные типы/ошибки в `internal/domain`.
- Вывод в `json/ndjson` держать в `internal/output`.
- Не смешивать форматирование ответа и бизнес-логику в одном месте.

## Контрактные инварианты
- Поддерживаемые форматы: `--format json` и `--format ndjson`.
- В каждом ответе/записи должен быть `result_state`.
- Ошибки должны включать: `error.code`, `error.message`, `error.exit_code`.
- Для `ndjson` использовать `record_type` (`result|item|issue|summary|error`) по сценарию команды.
- Для сущностей возвращать `revision` как opaque token.

## Коды ошибок и exit codes
- Использовать единый набор кодов ошибок из доменного слоя.
- Базовый маппинг exit codes:
  - `0` success
  - `1` domain error
  - `2` invalid args/query
  - `3` read/write error
  - `4` schema error
  - `5` internal error

## Структура репозитория
- Entry point: `cmd/spec-cli/main.go`
- Основные слои: `internal/cli`, `internal/application`, `internal/domain`, `internal/output`, `internal/contracts`
- Спецификация прототипа: `doc/001-base/SPEC_UTILITY_CLI_PROTOTYPE_RU.md`
- Локальная рабочая спецификация: `spec/SPEC_STANDARD_RU_REVISED_V3.md` (директория `spec/` в `.gitignore`)
- Индекс документации (точка входа): `doc/README.md`
- Индекс кодовой базы (agent map): `doc/CODEBASE_INDEX_RU.md`

## Команды разработки
- Форматирование: `make fmt`
- Статика: `make vet`
- Тесты: `make test`
- Сборка: `make build`

## Ожидания к изменениям
- Сохранять machine-stable контракт ответов.
- При добавлении/изменении команды обновлять контрактные тесты и snapshot/golden файлы.
- При добавлении/изменении документации в `doc/` обновлять `doc/README.md` в том же изменении.
- При любых изменениях структуры/ролей кодовой базы поддерживать `doc/CODEBASE_INDEX_RU.md` в актуальном состоянии в том же изменении.
- Не делать интерактивных prompt по умолчанию.
- Не раскрывать внутренние filesystem path сущностей в API-ответах.

## Правила структуры кода (entrypoint-first)
- Для любой директории с прикладной логикой держать явно видимый главный файл (entrypoint) в корне директории.
- Главный файл должен быть определяем по имени роли: например `handler.go` (команда), `app.go` (сборка приложения), `router.go`, `loader.go`, `runner.go`.
- В главном файле держать orchestration и публичный API пакета; детали реализации выносить в соседние файлы или подпакеты.
- Для команд use-case entrypoint фиксирован: `internal/application/commands/<command>/handler.go`.
- Для сложных команд детали выносить в `internal/application/commands/<command>/internal/...` с предметными именами пакетов (`options`, `schema`, `workspace`, `engine`, `support` и т.п.).
- Не использовать `common` как универсальный пакет; prefer предметные имена по назначению.
- Новую директорию с нетривиальной логикой создавать только вместе с явным entrypoint-файлом и понятной ролью директории.
- При code review и рефакторинге проверять “можно ли за 5 секунд понять главный файл директории”; если нет — структура считается неудачной.

## Стандарт реализации команд (по умолчанию)
- `handler.go` должен быть тонким orchestration-слоем: parse options -> load inputs/schema -> run use-case -> собрать `json/ndjson` ответ.
- Для `validate` использовать текущую структуру как baseline:
  - `internal/model` — внутренние типы команды.
  - `internal/options` — парсинг командных опций и нормализация путей.
  - `internal/schema` — загрузка/проверка schema.
  - `internal/workspace` — scan кандидатов и parse frontmatter/content.
  - `internal/engine` — основной pipeline проверки и issue aggregation.
  - `internal/support` — узкие pure helper-функции (yaml/collections/values).
- Business-логика, коды ошибок и issue-коды не менять без явной задачи на изменение поведения.
- Валидация должна оставаться детерминированной: стабильный порядок обхода/сортировок и стабильный формат ответа.
- После любых изменений в команде обязательно прогонять минимум `make vet` и `make test`.

## Правило работы с документацией
- Перед поиском деталей по проекту сначала проверять `doc/README.md`.
- Если добавлен новый документ, агент обязан добавить его в индекс с кратким описанием назначения.
- Нумерацию каталогов вида `NNN-*` использовать только для документации этапов (stage/milestone).
- Общезначимую документацию (индексы, карты, общие соглашения) размещать в корне `doc/` без номерного префикса.
