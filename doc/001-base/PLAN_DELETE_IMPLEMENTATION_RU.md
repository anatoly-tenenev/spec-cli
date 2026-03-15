# План реализации `delete` для baseline CLI API

## 1. Основание

План составлен только по двум источникам:

- `SPEC_UTILITY_CLI_API_BASELINE_RU.md`
- `SPEC_STANDARD_RU_REVISED_V3.md`

Ключевые нормативные опоры:

- baseline `§2`, `§5.2`, `§5.3`, `§6`, `§7.5`, `§7.8`, `§7.9`;
- standard `§5.3`, `§6.3`, `§7`, `§8`, `§11.1`, `§11.4`, `§14.2`.

## 2. Что должна делать команда

`delete` удаляет одну сущность по точному `id` и в успешном JSON-ответе возвращает:

- `result_state: "valid"`
- `dry_run`
- `deleted: true`
- `target.id`
- `target.revision`

Из baseline напрямую следует только три обязательных аргументных правила:

- `--id` обязателен;
- `--expect-revision` работает как optimistic concurrency guard;
- `--dry-run` должен поддерживаться.

## 3. Зафиксированные продуктовые решения

Ниже не прямые цитаты baseline, а принятые решения для закрытия неоднозначностей `delete`. Они должны считаться частью implementation plan.

1. Если сущность не найдена, `delete` должен возвращать `ENTITY_NOT_FOUND`.
Причина: `delete`, как и `get`, адресует одну сущность по точному `id`; отдельной альтернативной семантики baseline не задает.

2. `delete` не должен требовать полностью валидный workspace.
Причина: baseline явно допускает tolerant-режим для `get`; для удаления это тоже нужно, иначе поврежденные документы могут сделать удаление невозможным.

3. Целевая сущность может быть удалена, даже если она невалидна по данным, если ее можно детерминированно распарсить настолько, чтобы получить `type`, `id`, файловый путь и `revision`.
Причина: это следует из tolerant-подхода `get` и из практического смысла команды удаления.

4. Удаление должно блокироваться, если после него останутся битые ссылки `entity_ref` на удаленный `id`.
Причина: standard `§6.3`, `§11.4`, `§12.3`, `§14.2` требует ссылочную целостность и dataset-conformance.

5. Для блокировки удаления используется отдельный доменный код ошибки `DELETE_BLOCKED_BY_REFERENCES` с `result_state: "invalid"` и `error.exit_code = 1`.
Причина: baseline задает `CONCURRENCY_CONFLICT`, но не задает код для случая нарушения ссылочной целостности после удаления.

6. `dry-run` должен проходить тот же pipeline, что и обычный `delete`, и возвращать тот же success-payload с единственным отличием `dry_run: true`.
Причина: именно так baseline формулирует `dry-run` для `add` и `update`; для `delete` это наиболее консистентная трактовка.

7. `target.revision` должен вычисляться тем же механизмом, что и у `get`/`update`, а не отдельным ad hoc hashing файловых байтов.
Причина: baseline `§5.3` задает единый контракт `revision`.

## 4. Границы реализации

Что входит в первую реализацию:

- CLI-парсинг команды и опций;
- поиск цели по `id`;
- проверка `--expect-revision`;
- проверка входящих ссылок на удаляемую сущность;
- `dry-run`;
- физическое удаление файла;
- JSON-ответ baseline-формы;
- text-first help для команды.

Что не нужно добавлять в baseline-реализацию:

- tombstone/archive-механику;
- soft-delete;
- раскрытие filesystem path в публичном ответе;
- очистку пустых директорий как часть контракта;
- полную валидацию всего workspace как обязательный precondition.

## 5. План реализации

### Шаг 1. Подключить CLI-поверхность команды

- Зарегистрировать команду `delete`.
- Поддержать опции `--id`, `--expect-revision`, `--dry-run`.
- Подключить общие глобальные опции `--workspace`, `--schema`, `--format`, `--require-absolute-paths`.
- Для отсутствующего `--id` возвращать `INVALID_ARGS` с `exit_code = 2`.
- Для `--format json` использовать нормативный JSON-конверт.
- Для `--format text` оставить минимальный человеко-читаемый вывод без расширения контракта.

### Шаг 2. Собрать минимальный runtime-контекст

- Загрузить и распарсить schema-файл из `--schema`.
- Использовать raw schema как runtime-источник правил, а не `schema.model`.
- Подготовить из schema список типов сущностей, их `id_prefix` и ссылочных полей:
  - scalar `entity_ref`;
  - `array`, где `items.type = entity_ref`.
- Если schema отсутствует, не читается или не парсится так, что нельзя надежно резолвить типы и ссылки, завершать top-level ошибкой схемы с `exit_code = 4`.

### Шаг 3. Построить tolerant-индекс сущностей workspace

- Просканировать workspace на предмет реализаций сущностей.
- Для каждого кандидата попытаться прочитать frontmatter и извлечь минимум:
  - filesystem path для внутренней работы;
  - `type`;
  - `id`;
  - `slug`, если доступен;
  - набор raw-значений полей frontmatter, нужных для reverse-ref проверки;
  - `revision`.
- Тип сущности определять только по `type` из frontmatter, как требует standard `§5.3`.
- `id` трактовать как глобально уникальный идентификатор в масштабе dataset.
- Непарсибельные посторонние документы не должны блокировать `delete`; они игнорируются, если не являются целевой сущностью.
- Целевая сущность может быть удалена, даже если она невалидна по данным, если документ детерминированно дает `type`, `id` и `revision`.

### Шаг 4. Найти целевую сущность

- Выполнить exact-match по `id`.
- Если совпадения нет, вернуть `ENTITY_NOT_FOUND`.
- Если найдено более одного совпадения по одному `id`, вернуть top-level ошибку `AMBIGUOUS_ENTITY_ID` с `result_state: "invalid"` и `error.exit_code = 1`.
- Если целевой документ существует, но его нельзя дочитать настолько, чтобы вычислить `revision`, вернуть top-level ошибку `REVISION_UNAVAILABLE` с `result_state: "invalid"` и `error.exit_code = 1`.

### Шаг 5. Проверить concurrency guard

- Если передан `--expect-revision`, сравнить его с фактическим `target.revision`.
- При несовпадении вернуть `CONCURRENCY_CONFLICT` без удаления.
- Сравнение должно выполняться до любых изменений на диске.

### Шаг 6. Выполнить reverse-ref проверку

- По всем parseable-документам workspace проверить, есть ли ссылки на `target.id`.
- Проверять нужно как минимум:
  - поля `meta.<field>`, где schema объявляет `schema.type: entity_ref`;
  - массивы, где schema объявляет `type: array` и `items.type: entity_ref`.
- Блокирующей считается любая точная raw-ссылка на `target.id` в объявленном ссылочном слоте parseable-документа, даже если сам документ в целом невалиден.
- Если найдены входящие ссылки, вернуть top-level ошибку `DELETE_BLOCKED_BY_REFERENCES`.
- В ответе ошибки нужно вернуть структурированные детали по блокирующим сущностям без раскрытия их filesystem path.
- Формат `error.details`:
  - `blocking_refs[].source_id`
  - `blocking_refs[].source_type`
  - `blocking_refs[].field`
- Полную post-delete валидацию всего workspace делать не нужно; для baseline достаточно гарантировать, что удаление не оставляет новых битых `entity_ref`.

### Шаг 7. Выполнить удаление

- В обычном режиме удалить только один целевой файл.
- Удаление должно быть наблюдаемо атомарным на уровне результата команды: при неуспехе команда не должна сообщать успех.
- Если операция удаления на файловой системе не удалась, вернуть top-level ошибку чтения/записи с `exit_code = 3`.
- Очистку пустых parent-directory не делать частью baseline-контракта.

### Шаг 8. Реализовать `dry-run`

- `dry-run` должен проходить шаги 2-6 полностью.
- На шаге 7 физическое удаление не выполнять.
- Успешный `dry-run` должен вернуть:
  - `result_state: "valid"`
  - `dry_run: true`
  - `deleted: true`
  - `target.id`
  - `target.revision`

### Шаг 9. Сформировать ответ и ошибки

- Успех в JSON должен соответствовать baseline-примеру из `§7.8`.
- Для missing target использовать:
  - `result_state: "not_found"`
  - `error.code: "ENTITY_NOT_FOUND"`
- Для missing target использовать `error.exit_code = 1`.
- Для mismatch revision:
  - top-level `error.code: "CONCURRENCY_CONFLICT"`
- Для mismatch revision использовать `result_state: "invalid"` и `error.exit_code = 1`.
- Для ambiguous target:
  - `result_state: "invalid"`
  - `error.code: "AMBIGUOUS_ENTITY_ID"`
  - `error.exit_code: 1`
- Для unavailable revision:
  - `result_state: "invalid"`
  - `error.code: "REVISION_UNAVAILABLE"`
  - `error.exit_code: 1`
- Для blocked delete:
  - `result_state: "invalid"`
  - `error.code: "DELETE_BLOCKED_BY_REFERENCES"`
  - `error.exit_code: 1`
- Во всех ошибках сохранять baseline-поля:
  - `error.code`
  - `error.message`
  - `error.exit_code`

### Шаг 10. Дописать help

- В текстовой справке команды явно описать:
  - что удаление идет по точному `id`;
  - роль `--expect-revision`;
  - поведение `--dry-run`;
  - то, что удаление может быть заблокировано входящими ссылками.

## 6. Предлагаемая внутренняя декомпозиция

Минимальный набор внутренних частей:

- `DeleteCommand` или эквивалентный handler;
- общий `SchemaLoader`;
- общий tolerant `WorkspaceEntityIndex`;
- общий `RevisionService`;
- `ReverseReferenceChecker`;
- `DeleteExecutor` для реального удаления и dry-run режима;
- общий `JsonErrorBuilder`.

Если в кодовой базе уже есть `get` и `update`, для `delete` нужно переиспользовать в первую очередь:

- логику поиска сущности по `id`;
- логику вычисления `revision`;
- общую схему top-level error;
- общую работу с глобальными CLI-опциями.

## 7. Тест-план

### 7.1. Минимальный fixture contract

Для `delete` нужен не один workspace, а набор fixture-наборов, потому что часть сценариев требует специально сломанного состояния:

- `fixtures/delete/base`
- `fixtures/delete/duplicate-id`
- `fixtures/delete/repairable-invalid-target`
- `fixtures/delete/blocking-invalid-source`
- `fixtures/delete/revision-unavailable`
- `fixtures/delete/broken-schema`
- `fixtures/delete/broken-workspace`
- `fixtures/delete/io-failure` или эквивалентный runtime-harness

В тестовый набор нужно включать только те fixture'ы, которые реально используются хотя бы одним кейсом или общим harness.

При описании и реализации каждого отдельного тест-кейса нужно убирать из его fixture-сценария все документы, вспомогательные файлы и fixture-наборы, которые этим кейсом не используются. Иначе тест-план начинает маскировать лишние зависимости и усложняет локализацию причины падения.

Минимальная схема `fixtures/delete/base` должна содержать как минимум три типа:

- `service` без обязательных исходящих ссылок, пригодный как safe target для успешного удаления;
- `feature` с scalar-ссылкой `entity_ref` на `service`;
- `release` или эквивалентный тип с массивом `entity_ref` на `feature`.

Дополнительно в базовом workspace нужен хотя бы один документ, где `target.id` встречается в обычном string-поле или в body, но не в schema-объявленном ссылочном слоте. Это отдельный негативный guard против ложноположительной реализации reverse-ref проверки через grep по тексту.

Для сценариев с `--expect-revision` harness должен заранее вычислять revision-токены по точным persisted-байтам fixture-документов и использовать их как константы, например:

- `REV_SVC_1`
- `REV_FEAT_1`
- `REV_FEAT_2`
- `REV_INVALID_FEAT_9`

Для ошибок файловой системы нужен отдельный сценарий, где поиск target, вычисление `revision`, проверка `--expect-revision` и reverse-ref анализ уже успешно завершены, но сама filesystem-операция удаления целевого файла падает. Нормативна не техника инъекции сбоя, а наблюдаемое поведение: команда не должна вернуть успех и не должна оставлять частично измененное состояние.

### 7.2. Общие правила проверки

Во всех успешных JSON-кейсах нужно дополнительно проверять:

- exit code `0`;
- `result_state = "valid"`;
- наличие `dry_run`, `deleted`, `target.id`, `target.revision`;
- `deleted = true`;
- отсутствие `error`;
- отсутствие filesystem path в success-payload;
- `target.revision` совпадает с revision, вычисленным по тем же persisted-байтам, что и в `get`/`update`.

Во всех успешных кейсах без `--dry-run` нужно дополнительно проверять:

- физически удален ровно один ожидаемый файл;
- остальные документы workspace не изменились;
- для fixture'ов, которые по замыслу сценария должны оставаться полностью валидными после удаления, последующий `spec-cli validate --workspace <tmp> --schema <schema> --format json` завершается успешно;
- для tolerant-fixture'ов с заранее существующей нерелевантной невалидностью нужно проверять не полный success `validate`, а то, что `delete` не создал новых битых `entity_ref` и не ухудшил ссылочную целостность dataset относительно исходного состояния.

Во всех успешных кейсах с `--dry-run` нужно дополнительно проверять:

- `dry_run = true`;
- файловая система не меняется;
- повторный запуск той же команды без `--dry-run` на чистом fixture дает тот же success-payload, кроме различия `dry_run`.

Во всех error-кейсах нужно дополнительно проверять:

- файл цели не удален;
- `error.code`, `error.message`, `error.exit_code` присутствуют;
- filesystem path не протекают в публичный ответ;
- для `ENTITY_NOT_FOUND` используется `result_state = "not_found"`;
- для остальных доменных ошибок используется `result_state = "invalid"`.

Для `DELETE_BLOCKED_BY_REFERENCES` нужно дополнительно проверять:

- `error.code = "DELETE_BLOCKED_BY_REFERENCES"`;
- `error.details.blocking_refs[]` присутствует и не пуст;
- каждый элемент `blocking_refs[]` содержит только `source_id`, `source_type`, `field`;
- список не раскрывает filesystem path блокирующих документов.

### 7.3. Обязательные black-box кейсы

#### Happy path

- `DLT-OK-01`: успешное удаление существующей сущности без входящих ссылок и без `--expect-revision`.
- `DLT-OK-02`: успешное удаление существующей сущности с корректным `--expect-revision`; нужно явно проверить, что guard не ломает happy path.
- `DLT-OK-03`: успешный `dry-run` для удаляемой сущности без входящих ссылок; payload должен совпадать с обычным успехом, кроме `dry_run: true`.
- `DLT-OK-04`: удаление parseable, но data-invalid целевой сущности должно быть возможно, если у нее вычислим `revision` и на нее нет входящих ссылок.

#### Аргументы и lookup

- `DLT-ARG-01`: отсутствие `--id` возвращает `INVALID_ARGS` и `exit_code = 2`.
- `DLT-LOOKUP-01`: точный `id` не найден и команда возвращает `ENTITY_NOT_FOUND` с `result_state: "not_found"`.
- `DLT-LOOKUP-02`: в dataset найдено более одного документа с тем же `id`, и команда возвращает `AMBIGUOUS_ENTITY_ID`.
- `DLT-LOOKUP-03`: target найден, но `revision` нельзя надежно вычислить, и команда возвращает `REVISION_UNAVAILABLE`.

#### Concurrency

- `DLT-CONC-01`: `--expect-revision` не совпадает с фактическим `target.revision`, и команда возвращает `CONCURRENCY_CONFLICT` без удаления.
- `DLT-CONC-02`: успешный `dry-run` с корректным `--expect-revision` и успешный non-`dry-run` запуск на двух идентичных чистых копиях одного fixture возвращают одинаковый `target.revision` в success-payload.

#### Reverse references

- `DLT-REF-01`: удаление блокируется, если на цель указывает scalar `entity_ref`.
- `DLT-REF-02`: удаление блокируется, если на цель указывает элемент массива `entity_ref`.
- `DLT-REF-03`: parseable, но в целом невалидный документ с raw-ссылкой на `target.id` в schema-объявленном ссылочном слоте тоже блокирует удаление.
- `DLT-REF-04`: простое текстовое вхождение `target.id` в обычном string-поле или body не блокирует удаление, если этот слот не объявлен как `entity_ref`.
- `DLT-REF-05`: при `DELETE_BLOCKED_BY_REFERENCES` ответ возвращает минимальный `blocking_refs[]` без путей и с корректными `source_id`, `source_type`, `field`.
- `DLT-REF-06`: `dry-run` не обходит reverse-ref проверку; при наличии блокирующих ссылок он тоже должен завершаться `DELETE_BLOCKED_BY_REFERENCES`.

#### Tolerant workspace и I/O

- `DLT-TOL-01`: непарсибельные посторонние документы, не являющиеся target, игнорируются и не блокируют `delete`; parseable документы не подпадают под это послабление и участвуют в reverse-ref анализе даже если они в целом невалидны.
- `DLT-SCHEMA-01`: ошибка схемы возвращается, если `--schema` отсутствует, не читается или не позволяет надежно определить ссылочные слоты.
- `DLT-IO-01`: ошибка файловой системы, возникшая уже после успешного прохождения всех доменных проверок и непосредственно во время удаления целевого файла, возвращает top-level I/O error с `exit_code = 3`; успех не должен сообщаться.

#### Публичный контракт

- `DLT-RESP-01`: success JSON не раскрывает filesystem path.
- `DLT-RESP-02`: error JSON при `DELETE_BLOCKED_BY_REFERENCES` и других отказах тоже не раскрывает filesystem path.
- `DLT-HELP-01`: текстовая `help` для `delete` явно описывает точный `id`, роль `--expect-revision`, поведение `--dry-run` и блокировку по входящим ссылкам.

### 7.4. Рекомендуемые компонентные тесты

Кроме black-box сценариев, полезно зафиксировать несколько узких компонентных тестов, чтобы не ловить regressions только через интеграционный слой:

- extractor ссылочных слотов из raw schema различает scalar `entity_ref`, `array<entity_ref>` и обычные string/array поля;
- tolerant workspace-индекс игнорирует непарсибельные посторонние документы, но не скрывает duplicate `id`;
- reverse-ref checker смотрит только в schema-объявленные ссылочные слоты и не считает совпадения по произвольному тексту;
- delete executor в обычном и `dry-run` режиме использует один и тот же pipeline до commit-фазы;
- mapping ошибок стабильно различает `ENTITY_NOT_FOUND`, `AMBIGUOUS_ENTITY_ID`, `REVISION_UNAVAILABLE`, `CONCURRENCY_CONFLICT`, `DELETE_BLOCKED_BY_REFERENCES` и I/O fail.

## 8. Критерии готовности

Реализацию `delete` можно считать завершенной, если одновременно выполнены условия:

- проходят все обязательные black-box кейсы из раздела 7.3;
- команда стабильно находит цель по `id`;
- `--expect-revision` реально защищает от stale delete;
- удаление не оставляет новых битых `entity_ref`;
- `dry-run` повторяет обычный pipeline без записи на диск;
- JSON-успех и JSON-ошибки соответствуют baseline-контракту;
- в пользовательский контракт не протекают filesystem path внутренней реализации;
- хотя бы один отдельный тест доказывает отсутствие ложноположительных reverse-ref срабатываний по обычному тексту вне `entity_ref`-слотов.
