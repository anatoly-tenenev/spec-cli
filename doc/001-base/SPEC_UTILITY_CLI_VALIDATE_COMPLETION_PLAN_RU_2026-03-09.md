# План завершения реализации `spec-cli validate` (финализация)

Дата: 2026-03-09  
Статус: рабочий план исполнения

## 1. Цель

Завершить реализацию `validate` до состояния, соответствующего `SPEC_STANDARD_RU_REVISED_V3.md` и `SPEC_UTILITY_CLI_API_RU.md`, без деградаций по `expressions` и `entity_ref`.

Итоговое состояние:

- `required_when` и `path_pattern.cases[].when` работают по полной модели выражений;
- `entity_ref` резолвится детерминированно и формирует `ref`-контекст;
- соблюдены правила статической согласованности схемы (включая `path_pattern` и strict/safe операторы);
- `summary.validator_conformant` выставляется строго по контракту;
- `json/ndjson` формат и классы диагностик (`SchemaError`/`InstanceError`/`ProfileError`) соответствуют спецификации.

## 2. База и разрыв

### 2.1. Что уже зафиксировано

- Есть MVP-документ: `SPEC_UTILITY_CLI_VALIDATE_BASE_IMPLEMENTATION_RU.md` (без полноценного `expressions` и `entity_ref`).
- Есть план по expressions: `SPEC_UTILITY_CLI_VALIDATE_EXPRESSIONS_IMPLEMENTATION_PLAN_RU.md`.
- В `V3` и свежих предложениях добавлены/уточнены требования по статическому анализу схемы (`path_pattern`, `exists` guard, запрет strict для potentially-missing).

### 2.2. Что нужно закрыть дополнительно (помимо `entity_ref`)

- Статический запрет `eq`/`in` для potentially-missing операндов:
  - в `required_when`;
  - в `path_pattern.cases[].when`.
- Статическая проверка `path_pattern.cases[].use`:
  - `{meta:*}`/`{ref:*}` только если значение statically-safe;
  - `exists` в `when` учитывается как guard только в разрешенных формах;
  - `exists` без необходимости для `use`-плейсхолдеров считается ошибкой схемы.
- Разделение schema-time и runtime ошибок в строгом соответствии с `14.4`.
- Полная синхронизация `validator_conformant` с профилем реализации (включая детерминированный резолв `entity_ref/ref`).

## 3. Область работ

### 3.1. `entity_ref` и `ref`-контекст

- Схемный уровень:
  - проверка `schema.type: entity_ref`;
  - проверка `refTypes` (структура/типизация);
  - валидация ссылок `ref.<field>.<part>` только для `entity_ref`-полей.
- Runtime-уровень:
  - индекс сущностей датасета для резолва ссылок;
  - детерминированный алгоритм выбора целевой сущности;
  - проверка `refTypes` по фактическому `type` цели;
  - заполнение `ref.<field>.id|type|slug|dir_path`.
- Семантика выражений:
  - `meta.<entity_ref_field>` трактуется как `ref.<field>.id` (по результату резолва);
  - `exists` для `meta.<entity_ref_field>` и `ref.*` зависит от успешного резолва.

### 3.2. Expressions + static analysis

- Компиляция выражений в AST (schema-time).
- Валидация структуры операторов: `eq`, `eq?`, `in`, `in?`, `all`, `any`, `not`, `exists`.
- Классификация ссылок: `ALWAYS_AVAILABLE` vs `POTENTIALLY_MISSING`.
- Проверка strict-операторов в двух режимах:
  - `REQUIRED_WHEN`;
  - `PATH_WHEN`.
- Анализ `path_pattern.cases[].when` + `use` как единой конструкции:
  - извлечение `guard-set`;
  - проверка statically-safe использования плейсхолдеров.

### 3.3. Pipeline `validate`

Порядок внутри сущности:

1. Parse frontmatter + builtins.
2. Резолв `entity_ref` и построение runtime-context.
3. `required_when` для `meta.fields`.
4. Проверка значений `meta.fields` (`type/const/enum` и т.д.).
5. `required_when` для `content.sections`.
6. `path_pattern.cases[].when` + выбор кейса + проверка фактического пути.

Финализация:

- расчет `issues[]`, `summary`, `result_state`, exit code;
- `summary.validator_conformant=false`, если профиль реализации не детерминирован или покрытие требований неполное.

## 4. Этапы реализации

### Этап 0. Выравнивание контрактов (короткий)

- Зафиксировать целевые разделы стандарта/CLI API как source-of-truth.
- Утвердить стабильный набор `code` для новых диагностик.
- Согласовать режим внедрения статических правил (`warn` -> `enforce` или сразу `enforce`).

Definition of Done:

- единая таблица требований и маппинг на проверки валидатора;
- список `code`/`class`/`standard_ref` утвержден.

### Этап 1. Compiler + static schema checks

- Реализовать AST и компиляцию выражений.
- Реализовать статическую проверку ссылок `meta.*`/`ref.*`.
- Реализовать классификацию `potentially-missing`.
- Добавить schema-time проверки:
  - запрет strict `eq`/`in` для potentially-missing;
  - правила `exists` guard для `path_pattern`.

Definition of Done:

- некорректные схемы отсекаются до прохода по документам;
- нарушения, выводимые из схемы, классифицируются как `SchemaError`.

### Этап 2. `entity_ref` runtime core

- Реестр сущностей и детерминированный резолв ссылок.
- Проверки `refTypes` и неоднозначностей.
- Формирование `ref`-контекста с `dir_path`.
- Профиль резолва как обязательная зависимость конформантного режима.

Definition of Done:

- все `entity_ref` либо резолвятся однозначно, либо дают корректный `InstanceError`;
- при недоступном/недетерминированном профиле формируется `ProfileError`.

### Этап 3. Evaluator runtime semantics

- Реализовать поведение `Scalar|Missing`.
- Поведение операторов:
  - `eq`/`in`: strict;
  - `eq?`/`in?`: safe (`missing -> false`);
  - `all/any/not/exists` с short-circuit.
- Поддержать alias `meta.<entity_ref>` -> `ref.<field>.id`.

Definition of Done:

- unit-тесты покрывают все операторы и граничные случаи `missing`.

### Этап 4. Интеграция в `validate`

- Подключить evaluator в `required_when` (`meta.fields`, `content.sections`).
- Подключить вычисление `path_pattern.cases[].when`.
- Обеспечить единый контекст подстановок и выражений.

Definition of Done:

- результат валидации детерминирован при одинаковом входе;
- нет silent-fallback при strict-ошибках.

### Этап 5. Вывод и совместимость контракта CLI

- Проверить `json/ndjson` форматы (issue records + summary).
- Проверить `result_state`, exit codes, `warnings-as-errors`, `fail-fast`.
- Жестко проверить `summary.validator_conformant`.

Definition of Done:

- контракт `5.1 validate` соблюден для всех поддерживаемых режимов.

### Этап 6. Тестирование и hardening

- Unit:
  - compiler/evaluator;
  - классификация potentially-missing;
  - guard-логика `path_pattern`.
- Integration:
  - сценарии `entity_ref` + `ref.dir_path`;
  - `required_when`, `content.sections.required_when`;
  - сложные `path_pattern` с fallback.
- Contract:
  - golden-tests для `json/ndjson`;
  - проверка классов диагностик и `standard_ref`.
- Производительность:
  - компиляция выражений один раз на схему;
  - O(1) lookup для refs и schema-полей.

Definition of Done:

- регрессионный набор проходит;
- нет ухудшения SLA на типичных workspace.

## 5. Минимальный набор новых диагностик (рекомендация)

- `schema.expression.invalid_operator`
- `schema.expression.invalid_arity`
- `schema.expression.invalid_operand_type`
- `schema.expression.invalid_reference`
- `schema.required_when.strict_potentially_missing`
- `schema.path_when.strict_potentially_missing`
- `schema.path_pattern.placeholder_not_guarded`
- `schema.path_pattern.unused_exists_guard`
- `instance.entity_ref.unresolved`
- `instance.entity_ref.ref_type_mismatch`
- `profile.entity_ref_resolution_unavailable`

Важно: конкретные `code` можно адаптировать к текущему naming, но `class` и `standard_ref` должны остаться нормативно корректными.

## 6. Критерии готовности (финальные)

- Реализация полностью закрывает `expressions` и `entity_ref` без MVP-деградаций.
- Статические противоречия схемы ловятся как `SchemaError` до прохода по данным.
- Runtime-проблемы данных остаются `InstanceError`.
- `summary.validator_conformant` отражает реальную полноту профиля и проверок.
- Формат и поведение `validate` совместимы с `SPEC_UTILITY_CLI_API_RU.md`.

## 7. Порядок выполнения (рекомендуемый)

1. Этап 1 (compiler + static checks).
2. Этап 2 (`entity_ref` runtime core).
3. Этап 3 (evaluator semantics).
4. Этап 4 (pipeline integration).
5. Этап 5 (contract/output hardening).
6. Этап 6 (full test sweep + performance tuning).
