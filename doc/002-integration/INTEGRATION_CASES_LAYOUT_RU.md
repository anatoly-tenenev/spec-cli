# Data-first структура интеграционных кейсов `spec-cli`

Документ фиксирует соглашение по интеграционным тестам в формате **реальных данных**,
чтобы основная читаемость была в файловой структуре кейсов, а не в Go-коде.

Связанный документ: [SPEC_UTILITY_CLI_PROTOTYPE_RU.md](../001-base/SPEC_UTILITY_CLI_PROTOTYPE_RU.md).

## 1. Принципы

- Интеграционные тесты строятся как `data-first`: кейс описывается файлами.
- Go-тесты и раннер минимальны: загрузить кейс, выполнить CLI, сравнить ожидаемое.
- Кейсы сгруппированы **по командам**: `validate`, `query`, `add`, `update`.
- Каждый кейс содержит входной workspace, схему, ожидаемый ответ CLI.
- Для модифицирующих команд (`add`, `update`) кейс дополнительно содержит ожидаемый `workspace.out`.
- Каталог с кейсами располагается отдельно от обычных `testdata`: `tests/integration/cases`.

## 2. Каноническое расположение

```text
tests/
  integration/
    run_cases_test.go
    runner/
      load_case.go
      run_cli.go
      assert_response.go
      assert_workspace.go
      normalize.go

    cases/
      validate/
        <XXXX_case-id>/
          case.json
          spec.schema.yaml
          workspace.in/
          response.json

      query/
        <XXXX_case-id>/
          case.json
          spec.schema.yaml
          workspace.in/
          response.json

      add/
        <XXXX_case-id>/
          case.json
          spec.schema.yaml
          workspace.in/
          workspace.out/
          response.json

      update/
        <XXXX_case-id>/
          case.json
          spec.schema.yaml
          workspace.in/
          workspace.out/
          response.json
```

## 3. Контракт директории кейса

Директория `tests/integration/cases/<command>/<XXXX_case-id>/` обязана содержать:

- `case.json` — мета-описание запуска и проверок.
- `spec.schema.yaml` — схема, с которой запускается команда.
- `workspace.in/` — входное состояние workspace.
- `response.json` — ожидаемый ответ команды (канонизированный JSON).

Дополнительно для модифицирующих команд:

- `workspace.out/` — ожидаемое состояние workspace после выполнения.

Опционально (если нужен отдельный контроль):

- `stderr.txt` — ожидаемый stderr.
- `notes.md` — комментарии к кейсу.

## 4. Формат `case.json`

`case.json` задает только сценарий исполнения; фактические данные лежат в соседних файлах.

Пример:

```json
{
  "id": "add_doc_minimal",
  "description": "add creates a new document in empty workspace",
  "command": "add",
  "args": [
    "add",
    "--workspace", "${WORKSPACE}",
    "--schema", "${SCHEMA}",
    "--format", "json",
    "--type", "doc",
    "--slug", "intro"
  ],
  "expect": {
    "exit_code": 0,
    "response_file": "response.json",
    "stderr_file": ""
  },
  "workspace": {
    "input_dir": "workspace.in",
    "output_dir": "workspace.out",
    "assert_output": true
  }
}
```

Правила полей:

- `command`: одно из `validate|query|add|update`.
- `args`: аргументы CLI без имени бинарника.
- `${WORKSPACE}` и `${SCHEMA}` — служебные плейсхолдеры раннера.
- `expect.exit_code`: ожидаемый код завершения.
- `expect.response_file`: путь к ожидаемому ответу (обычно `response.json`).
- `workspace.assert_output`:
  - `false` для read-only (`validate`, `query`),
  - `true` для модифицирующих (`add`, `update`).

## 5. Представление `response.json`

`response.json` обязателен во всех кейсах.

- Для `--format json` хранится обычный top-level JSON-объект ответа.
- Для `--format ndjson` хранится **каноничное JSON-представление NDJSON**:
  - массив записей в ожидаемом порядке;
  - каждая запись соответствует одной строке NDJSON.

Пример для NDJSON:

```json
[
  { "record_type": "item", "result_state": "valid", "item": { "id": "d1" } },
  { "record_type": "summary", "result_state": "valid", "summary": { "returned": 1 } }
]
```

Так все кейсы читаются одинаково: один тип ожидаемого файла (`response.json`) независимо от формата вывода.

## 6. Правила сравнения

### 6.1 Ответ команды

Раннер обязан сравнивать:

- структуру и значения `response.json`;
- обязательные инварианты контракта (`result_state`, `error.*`, `record_type`, `revision` по сценарию);
- `exit_code`.

Допускается нормализация нестабильных значений (например, временные поля), но только явным правилом в `runner/normalize.go`.

### 6.2 Workspace

Если `workspace.assert_output=true`, раннер сравнивает `workspace.out` с фактическим workspace после команды:

- сравнение структуры директорий и файлов;
- сравнение содержимого текстовых файлов побайтно;
- запрет на неожиданные лишние файлы.

## 7. Нейминг кейсов

Формат имени директории кейса:

`<XXXX>_<case-id>`

где:

- `XXXX` — обязательный 4-значный числовой префикс (`0001`, `0002`, ...);
- `<case-id>` — смысловой идентификатор сценария.

Рекомендуемый формат `case-id`:

`<intent>_<scope>_<expected>`

Примеры:

- `0001_validate_full_ok_minimal`
- `0002_query_by_tag_valid`
- `0101_add_doc_valid_minimal`
- `0203_update_title_conflict_invalid`

Требования:

- обязательный префикс `XXXX_` перед `case-id`;
- только `lower_snake_case`;
- имя должно отражать ожидаемое поведение;
- имя стабильно во времени (используется в логах и CI-репортах).

## 8. Минимальный набор кейсов (first pass)

Для каждой команды:

- `success` базовый путь;
- `invalid_args`;
- `domain_error`;
- `schema_error`;
- формат `json` и формат `ndjson`.

Дополнительно:

- для `add/update`: минимум 1 кейс с `workspace.out`;
- для `query`: минимум 1 кейс с `items` и 1 кейс с пустым результатом;
- для `validate`: минимум 1 кейс с issue-ами.

## 9. Почему не `testdata`

Для обычных unit-тестов `testdata` подходит, но здесь цель другая:

- сделать интеграционные сценарии визуально самодокументируемыми;
- отделить большие data-кейсы от пакетных unit-тестов;
- упростить ревью через файловые diff по входу/выходу.

Поэтому кейсы хранятся как first-class артефакты в `tests/integration/cases`.

## 10. Checklist на новый кейс

1. Создать директорию `tests/integration/cases/<command>/<XXXX_case-id>/`.
2. Добавить `case.json`.
3. Добавить `spec.schema.yaml`.
4. Подготовить `workspace.in/`.
5. Добавить `response.json`.
6. Для `add/update` добавить `workspace.out/` и включить `assert_output=true`.
7. Запустить интеграционный раннер и проверить, что кейс проходит без ручных допущений.
