# Подробный план реализации `get`

Основание для плана:

- `SPEC_UTILITY_CLI_API_BASELINE_RU.md`
- `SPEC_STANDARD_RU_REVISED_V3.md`

Ограничение этого плана: он опирается только на два документа выше и не использует иные источники.

## 1. Цель

Реализовать baseline-команду:

```bash
spec-cli get [options] --id <id>
```

Команда должна:

- возвращать одну сущность по точному `id`;
- использовать канонический read-namespace, общий с `query`;
- не требовать полной валидности всего workspace;
- не скрывать существующую, но невалидную сущность под видом `ENTITY_NOT_FOUND`;
- завершаться top-level ошибкой, если целевую сущность нельзя детерминированно распарсить, определить ее `type` или вычислить запрошенные read-поля;
- поддерживать нормативный JSON-контракт baseline.

## 2. Нормативный контракт `get`

### Вход

- обязательный `--id <id>`;
- повторяемый `--select <field>`;
- если `--select` не задан, default projection:
  - `type`
  - `id`
  - `slug`
  - `revision`
  - `meta`

### Выход при успехе

```json
{
  "result_state": "valid",
  "target": {
    "match_by": "id",
    "id": "FEAT-8"
  },
  "entity": {
    "type": "feature",
    "id": "FEAT-8",
    "slug": "retry-window",
    "revision": "sha256:def456",
    "meta": {
      "status": "active"
    }
  }
}
```

### Нормативные правила

- неизвестный selector -> `INVALID_ARGS`;
- сущность не найдена -> `ENTITY_NOT_FOUND`;
- если выбран `content.sections.<name>` и секция отсутствует, поле должно быть возвращено со значением `null`;
- `get` требует валидную и читаемую схему;
- ошибки схемы должны завершать команду top-level ошибкой схемы;
- нарушение уровня данных у целевой сущности само по себе не блокирует `get`, если нужные read-поля вычислимы;
- отсутствие значения, включая отсутствующее обязательное `meta.<name>`, трактуется как absent value;
- `get` не должен возвращать `ENTITY_NOT_FOUND` для существующей, но невалидной сущности;
- если целевая сущность не может быть детерминированно обработана для текущего запроса, partial success недопустим.

## 3. Объем реализации

В реализацию `get` должны войти следующие возможности.

### 3.1. Загрузка и интерпретация схемы

- чтение YAML-схемы по effective path;
- проверка, что схема пригодна для построения read-namespace;
- извлечение из схемы:
  - допустимых `entity`-типов;
  - описаний `meta.fields`;
  - описаний `content.sections`;
  - типов `entity_ref`;
  - read-selector-ов, доступных в baseline-модели.

### 3.2. Канонический read-namespace

Поддержать пути:

- built-in:
  - `type`
  - `id`
  - `slug`
  - `created_date`
  - `updated_date`
  - `revision`
- meta:
  - `meta`
  - `meta.<name>`
- refs:
  - `refs`
  - `refs.<field>`
  - `refs.<field>.type`
  - `refs.<field>.id`
  - `refs.<field>.slug`
- content:
  - `content.raw`
  - `content.sections`
  - `content.sections.<name>`

### 3.3. Поиск сущности по `id`

- сканирование workspace;
- парсинг frontmatter у кандидатов;
- извлечение `id` до полной валидации документа;
- сопоставление точного `id`;
- обработка неоднозначности, если найдено более одного документа с одним и тем же `id`.

### 3.4. Детеминированное чтение целевой сущности

Для целевой сущности нужно уметь:

- распарсить `YAML frontmatter`;
- определить `type` по полю `type`;
- отделить built-in поля от `meta.fields`;
- извлечь raw body документа;
- построить нормализованную модель секций по правилам стандарта;
- вычислить `refs` по успешно резолвленным `entity_ref`;
- вычислить `revision` по фактическому состоянию документа.

### 3.5. Формирование ответа по `--select`

- поддержать default projection;
- валидировать каждый selector до чтения сущности;
- materialize-ить только запрошенный подграф;
- merge-ить пересекающиеся selector-ы без дублирования;
- возвращать object-node целиком при выборе object-path;
- возвращать `null` для отсутствующей `content.sections.<name>`.

## 4. Предлагаемая декомпозиция на слои

### Слой 1. CLI-обвязка команды

Ответственность:

- разбор аргументов `get`;
- проверка обязательности `--id`;
- сбор `--select`;
- выбор JSON/text-пути исполнения;
- маппинг ошибок в exit code и top-level error.

Результат:

- структура запроса вида:
  - `id`
  - `selectors`
  - `output_format`
  - `workspace_path`
  - `schema_path`

### Слой 2. Schema read-model

Ответственность:

- загрузить схему;
- собрать read-capabilities;
- определить допустимые selectors;
- подготовить правила вычисления `meta`, `refs`, `content.sections`.

Результат:

- нормализованная read-model, пригодная и для `get`, и для `query`.

### Слой 3. Entity locator

Ответственность:

- пройти по файлам workspace;
- уметь извлечь frontmatter-кандидаты;
- найти документ по `id`;
- отличать:
  - target не найден;
  - target найден;
  - target найден неоднозначно;
  - target физически найден, но не может быть надежно распарсен.

Результат:

- дескриптор найденного документа или доменная ошибка поиска.

### Слой 4. Entity read-engine

Ответственность:

- разобрать target-документ;
- вычислить built-in поля;
- собрать `meta`;
- резолвить `refs`;
- разобрать секции;
- вычислить `content.raw`;
- вычислить `revision`.

Результат:

- внутреннее read-представление сущности с поддержкой absent values.

### Слой 5. Selector projector

Ответственность:

- применить selectors к внутреннему read-представлению;
- собрать минимальный корректный JSON-подграф;
- корректно merge-ить пересечения;
- соблюдать special-case для `content.sections.<name> = null`.

Результат:

- `entity` payload для JSON-ответа.

### Слой 6. Error mapping

Ответственность:

- различать:
  - schema error;
  - invalid args;
  - entity not found;
  - blocking read error;
  - internal error;
- при наличии прикладывать `error.details.validation.issues[]`.

## 5. Пошаговый план реализации

## Этап 1. Зафиксировать контракт команды

Сделать:

- добавить/уточнить command spec в коде CLI;
- объявить `--id` обязательным;
- объявить `--select` повторяемым;
- зафиксировать default projection.

Проверка готовности:

- вызов без `--id` дает `INVALID_ARGS`;
- вызов с пустым списком selectors использует default projection.

## Этап 2. Построить общий validator selectors для read-namespace

Сделать:

- реализовать нормализатор selector-ов;
- валидировать selector по read-model схемы;
- различать object-node и leaf-node;
- подготовить merge-правила для пересечений.

Нужно поддержать:

- `meta` и `meta.<name>`;
- `refs`, `refs.<field>`, `refs.<field>.<part>`;
- `content.raw`, `content.sections`, `content.sections.<name>`;
- built-in пути.

Проверка готовности:

- неизвестный selector -> `INVALID_ARGS`;
- `meta` + `meta.status` не ломают форму результата;
- `content.sections` + `content.sections.summary` корректно объединяются.

## Этап 3. Реализовать поиск target по `id`

Сделать:

- просканировать workspace;
- на уровне быстрого чтения frontmatter извлекать `id`;
- не требовать полной валидности документа для попадания в кандидаты;
- обрабатывать случаи:
  - `0` кандидатов;
  - `1` кандидат;
  - `>1` кандидатов.

Проверка готовности:

- отсутствующий `id` -> `ENTITY_NOT_FOUND`;
- duplicate `id` -> blocking error, а не произвольный выбор одного файла.

## Этап 4. Реализовать tolerant parse target-документа

Сделать:

- парсить `YAML frontmatter` по правилам стандарта;
- требовать, чтобы `type` можно было определить детерминированно;
- допускать отсутствие необязательных или даже обязательных с точки зрения полной валидации полей, если они не нужны для текущего `get`;
- отделять blocking parse failure от non-blocking validation failure.

Blocking cases для `get`:

- frontmatter не удается распарсить;
- нельзя определить `type`;
- невозможно вычислить хотя бы одно из запрошенных read-полей;
- структура документа не позволяет детерминированно построить нужный read-path.

Non-blocking cases для `get`:

- у сущности отсутствует обязательное `meta.<name>`, но оно не нужно или может быть представлено как absent;
- отсутствует обязательная секция, но запрошенные поля остаются вычислимы;
- у сущности есть иные нарушения полной валидации, не мешающие текущей проекции.

## Этап 5. Реализовать вычисление built-in и `meta`

Сделать:

- читать `type`, `id`, `slug`, `created_date`, `updated_date` из frontmatter;
- собирать `meta` как объект из не-built-in полей frontmatter, объявленных в `meta.fields` схемы;
- поддержать absent values:
  - отсутствующее `meta.<name>` не является автоматической ошибкой `get`;
  - object `meta` содержит только фактически присутствующие и вычислимые поля.

Проверка готовности:

- `--select meta` возвращает подграф только разрешенных metadata полей;
- missing `meta.status` не ломает `get`.

## Этап 6. Реализовать вычисление `refs`

Сделать:

- определить по схеме, какие `meta`-поля являются `entity_ref`;
- резолвить их по `id` на целевые сущности;
- отдавать expanded read-view:
  - `refs.<field>.id`
  - `refs.<field>.type`
  - `refs.<field>.slug`
- `refs.<field>` возвращать как объект-подграф;
- если ссылка отсутствует или не резолвится, трактовать read-value как absent, если это не делает невозможным вычисление запрошенного поля.

Важно:

- raw значение ссылки в frontmatter остается в `meta.<field>`;
- expanded форма возвращается только в `refs`.

Проверка готовности:

- `--select meta.container --select refs.container.id` возвращает raw id и expanded object в разных ветках;
- неуспешный резолв ссылки не должен автоматически ломать `get`, если `refs` не запрашивались;
- если `refs.container.id` запрошен и вычислить его нельзя, команда должна завершиться ошибкой.

## Этап 7. Реализовать `content.raw` и `content.sections`

Сделать:

- выделить raw body после frontmatter;
- реализовать нормализацию секций по стандарту;
- поддержать два синтаксиса label:
  - `[<title>](#<label>)`
  - `<title> {#<label>}`
- извлекать текст секции по label;
- `content.sections.<name>` интерпретировать как тело секции без heading;
- `content.sections` возвращать объект всех вычислимых секций по каноническому namespace;
- при запросе конкретной отсутствующей секции возвращать `null`.

Отдельно проверить:

- автоматическое вычисление label из заголовка без явной маркировки недопустимо;
- duplicate labels в target должны считаться blocking case, если затронуты запросом `content.sections`.

Проверка готовности:

- `content.raw` возвращает исходное body;
- `content.sections.summary` возвращает тело секции;
- отсутствующая `summary` возвращается как `null`;
- duplicate label при запросе `content.sections.summary` завершает команду ошибкой.

## Этап 8. Реализовать `revision`

Сделать:

- вычислять строковый revision-token по фактическому состоянию документа;
- гарантировать изменение revision при изменении frontmatter или body;
- подключить `revision` в общее read-представление.

Проверка готовности:

- два разных состояния одного документа дают разные `revision`;
- одинаковый документ дает стабильный `revision`.

## Этап 9. Собрать projector ответа

Сделать:

- материализовать подграф по selectors;
- merge-ить пересечения;
- не включать невыбранные ветки;
- для object-node возвращать весь подграф;
- различать absent key и `null` special-case для `content.sections.<name>`.

Проверка готовности:

- `--select meta` возвращает весь `meta`;
- `--select refs.container` возвращает весь объект ссылки;
- `--select content.sections` возвращает объект секций;
- `--select content.sections.summary` возвращает только нужную ветку.

## Этап 10. Реализовать error policy и exit codes

Сделать:

- `INVALID_ARGS` -> exit `2`;
- schema-loading / schema-shape failure -> exit `4`;
- `ENTITY_NOT_FOUND` -> exit `1`;
- blocking data-read error -> доменная top-level ошибка с exit `1`;
- unexpected internal error -> exit `5`.

Проверка готовности:

- ни один blocking case не отдает partial `entity`;
- ошибки схемы не маскируются под `ENTITY_NOT_FOUND`;
- существующая, но невалидная сущность не маскируется под `ENTITY_NOT_FOUND`.

## 6. Предлагаемая внутренняя модель данных

Ниже минимальная модель, которую удобно держать в коде.

### Read schema model

- `entity_types`
- `meta_fields_by_type`
- `ref_fields_by_type`
- `content_sections_by_type`
- `allowed_selectors`

### Parsed entity descriptor

- `document_path`
- `raw_frontmatter`
- `raw_body`
- `frontmatter_map`
- `entity_type`
- `entity_id`

### Resolved read entity

- `type`
- `id`
- `slug`
- `created_date`
- `updated_date`
- `revision`
- `meta`
- `refs`
- `content.raw`
- `content.sections`

### Absent value policy

- обычное отсутствующее leaf-значение не материализуется в JSON, если его не требует special-case;
- `content.sections.<name>` при явном запросе материализуется как `null`, если секция отсутствует;
- object-node не должен включать невычислимые дочерние поля, если они не запрошены и не требуются для построения самого object-node.

## 7. Решения, которые лучше сразу сделать общими с `query`

Чтобы не делать второй раз ту же работу, для `get` сразу стоит вынести в общий пакет/модуль:

- загрузку read-model из схемы;
- validator selector-ов;
- parser read-namespace path;
- projector JSON-подграфа;
- tolerant entity reader;
- `refs` resolver;
- parser `content.sections`.

Причина:

- baseline прямо задает единый read-namespace для `query` и `get`;
- логика селекторов и материализации должна быть одинаковой;
- расхождение между `get` и `query` по shape ответа станет источником regressions.

## 8. Интеграционные тесты

Ниже набор тестов, который покрывает baseline-поведение `get`.

## Базовые сценарии успеха

### 1. `get_default_projection_ok`

Дано:

- валидная схема;
- валидная сущность `FEAT-8`.

Шаг:

- `spec-cli get --format json --id FEAT-8`

Ожидание:

- exit code `0`;
- `result_state = "valid"`;
- `target.match_by = "id"`;
- `target.id = "FEAT-8"`;
- в `entity` есть:
  - `type`
  - `id`
  - `slug`
  - `revision`
  - `meta`

### 2. `get_custom_select_ok`

Шаг:

- `spec-cli get --format json --id FEAT-8 --select meta.status --select refs.container.id --select content.sections.summary`

Ожидание:

- exit code `0`;
- в `entity` есть только запрошенные ветки;
- `meta.status` присутствует;
- `refs.container.id` присутствует;
- `content.sections.summary` присутствует.

### 3. `get_selector_merge_ok`

Шаг:

- `spec-cli get --format json --id FEAT-8 --select meta --select meta.status --select content.sections --select content.sections.summary`

Ожидание:

- exit code `0`;
- `meta` возвращен один раз как объект;
- `content.sections` возвращен как корректный объект;
- форма JSON не содержит дублей и конфликтов веток.

### 4. `get_select_object_node_meta_ok`

Шаг:

- `spec-cli get --format json --id FEAT-8 --select meta`

Ожидание:

- `entity.meta` содержит весь допустимый и вычислимый metadata-подграф.

### 5. `get_select_object_node_refs_ok`

Шаг:

- `spec-cli get --format json --id FEAT-8 --select refs`

Ожидание:

- `entity.refs` содержит все вычислимые expanded refs.

### 6. `get_select_content_raw_ok`

Шаг:

- `spec-cli get --format json --id FEAT-8 --select content.raw`

Ожидание:

- возвращается raw body документа без frontmatter.

## Сценарии отсутствующих значений

### 7. `get_missing_section_returns_null`

Дано:

- у target нет секции `summary`.

Шаг:

- `spec-cli get --format json --id FEAT-8 --select content.sections.summary`

Ожидание:

- exit code `0`;
- `entity.content.sections.summary = null`.

### 8. `get_missing_meta_field_is_absent_not_error`

Дано:

- у target отсутствует metadata-поле, обязательное по полной валидации.

Шаг:

- `spec-cli get --format json --id FEAT-8 --select meta`

Ожидание:

- exit code `0`;
- `get` успешен;
- отсутствующее поле не приводит к top-level ошибке.

### 9. `get_missing_ref_not_requested_does_not_block`

Дано:

- у target ссылка не резолвится.

Шаг:

- `spec-cli get --format json --id FEAT-8 --select meta`

Ожидание:

- exit code `0`;
- команда не падает только из-за сломанного `refs`, если `refs` не запрошены.

## Ошибки аргументов и схемы

### 10. `get_missing_id_arg`

Шаг:

- `spec-cli get --format json`

Ожидание:

- exit code `2`;
- `error.code = "INVALID_ARGS"`.

### 11. `get_invalid_selector`

Шаг:

- `spec-cli get --format json --id FEAT-8 --select meta.unknown`

Ожидание:

- exit code `2`;
- `error.code = "INVALID_ARGS"`.

### 12. `get_schema_missing`

Шаг:

- вызов с отсутствующей схемой.

Ожидание:

- exit code `4`;
- top-level ошибка схемы;
- без `entity`.

### 13. `get_schema_unparseable`

Шаг:

- вызов с YAML-схемой, которую нельзя распарсить.

Ожидание:

- exit code `4`;
- top-level ошибка схемы.

### 14. `get_schema_cannot_build_read_namespace`

Дано:

- схема формально загружена, но из нее нельзя корректно построить read-namespace.

Ожидание:

- exit code `4`;
- top-level ошибка схемы.

## Поиск target по `id`

### 15. `get_not_found`

Шаг:

- `spec-cli get --format json --id FEAT-404`

Ожидание:

- exit code `1`;
- `result_state = "not_found"`;
- `error.code = "ENTITY_NOT_FOUND"`.

### 16. `get_duplicate_id_conflict`

Дано:

- в workspace два документа с одинаковым `id`.

Шаг:

- `spec-cli get --format json --id FEAT-8`

Ожидание:

- команда не выбирает файл произвольно;
- top-level ошибка;
- без partial `entity`.

## Существующая, но невалидная сущность

### 17. `get_target_invalid_but_readable`

Дано:

- целевая сущность нарушает полную валидацию, например отсутствует обязательная секция.

Шаг:

- `spec-cli get --format json --id FEAT-8 --select type --select id --select slug --select meta`

Ожидание:

- exit code `0`;
- команда успешна;
- сущность не маскируется под `ENTITY_NOT_FOUND`.

### 18. `get_unrelated_invalid_document_does_not_block`

Дано:

- в workspace есть другой невалидный документ.

Шаг:

- `spec-cli get --format json --id FEAT-8`

Ожидание:

- exit code `0`;
- `get` по читаемому target успешен.

## Blocking read errors

### 19. `get_target_frontmatter_unparseable`

Дано:

- файл target существует, но frontmatter не парсится.

Шаг:

- `spec-cli get --format json --id FEAT-8`

Ожидание:

- top-level ошибка;
- без partial `entity`;
- при наличии diagnostics они попадают в `error.details.validation.issues[]`.

### 20. `get_target_type_cannot_be_determined`

Дано:

- target найден по `id`, но `type` отсутствует или не может быть детерминирован.

Ожидание:

- top-level ошибка;
- без partial `entity`.

### 21. `get_requested_ref_cannot_be_computed`

Дано:

- `refs.container.id` запрошен, но ссылка не резолвится.

Шаг:

- `spec-cli get --format json --id FEAT-8 --select refs.container.id`

Ожидание:

- top-level ошибка;
- без partial `entity`.

### 22. `get_requested_sections_cannot_be_determined`

Дано:

- в target дублирован ярлык секции или нарушена структура так, что нужную секцию нельзя определить однозначно.

Шаг:

- `spec-cli get --format json --id FEAT-8 --select content.sections.summary`

Ожидание:

- top-level ошибка;
- без partial `entity`.

## Проверка `refs`

### 23. `get_refs_resolution_ok`

Дано:

- у target есть валидная `entity_ref` ссылка.

Шаг:

- `spec-cli get --format json --id FEAT-8 --select refs.container`

Ожидание:

- `refs.container.id`, `refs.container.type`, `refs.container.slug` вычислены корректно.

### 24. `get_meta_ref_and_expanded_ref_are_distinct`

Шаг:

- `spec-cli get --format json --id FEAT-8 --select meta.container --select refs.container.id`

Ожидание:

- `meta.container` содержит raw id из frontmatter;
- `refs.container.id` содержит expanded read-view;
- значения возвращаются в разных ветках без смешения.

## Проверка `revision`

### 25. `get_revision_present_in_default_projection`

Шаг:

- `spec-cli get --format json --id FEAT-8`

Ожидание:

- `entity.revision` обязательно присутствует.

### 26. `get_revision_changes_on_document_change`

Дано:

- один и тот же документ в двух разных состояниях.

Ожидание:

- `revision` меняется при изменении frontmatter;
- `revision` меняется при изменении body.

## 9. Фикстуры для интеграционных тестов

Минимальный набор тестовых данных:

- одна валидная схема с типом, где есть:
  - обычные metadata-поля;
  - хотя бы одно `entity_ref`;
  - хотя бы две секции `content.sections`;
- одна валидная целевая сущность;
- одна целевая сущность без обязательной секции;
- одна целевая сущность с отсутствующим обязательным metadata-полем;
- одна целевая сущность с нерезолвящейся ссылкой;
- один документ с duplicate `id`;
- один документ с broken frontmatter;
- один документ с duplicate section label;
- один валидный target-linked документ для `refs` expansion.

## 10. Критерии готовности

Реализацию `get` можно считать завершенной, если выполняются все условия:

- JSON-контракт соответствует baseline;
- default projection реализован;
- selector validation реализован;
- `content.sections.<name> -> null` при отсутствии секции реализовано;
- существующая, но невалидная сущность читается, если проекция вычислима;
- `ENTITY_NOT_FOUND` возвращается только при реальном отсутствии target;
- blocking read-failures не приводят к partial success;
- интеграционные тесты покрывают success-path, absent values, schema errors, not found, duplicate id, blocking parse/read failures, `refs`, `revision`.

## 11. Рекомендуемый порядок разработки

Практический порядок:

1. CLI contract и `--id` / `--select`.
2. Schema read-model.
3. Selector validator и projector.
4. Поиск target по `id`.
5. Tolerant parse target.
6. Built-in + `meta`.
7. `revision`.
8. `refs`.
9. `content.raw` + `content.sections`.
10. Error mapping.
11. Интеграционные тесты.

Такой порядок снижает риск, потому что:

- shape read-namespace фиксируется до бизнес-логики;
- `get` и будущий `query` получают общий read-layer;
- интеграционные тесты можно наращивать поверх уже стабильного projector-а.

