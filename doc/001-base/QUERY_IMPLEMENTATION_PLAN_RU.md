# План реализации команды `query`

Этот документ самодостаточен. Для реализации `spec-cli query` достаточно требований, примеров и решений, зафиксированных ниже. Обращаться к другим спецификациям не требуется.

## 1. Назначение и границы

`query` это единственная команда CLI для чтения множества сущностей.

Команда должна:

- читать сущности из workspace;
- уметь рано сужать выборку по `--type`;
- поддерживать структурный фильтр `--where-json`;
- поддерживать проекцию через `--select`;
- поддерживать детерминированную сортировку через `--sort`;
- поддерживать только offset-пагинацию через `--limit` и `--offset`;
- возвращать нормативный машинный ответ в JSON.

Команда не должна:

- возвращать внутренние filesystem path сущностей;
- использовать cursor pagination;
- зависеть от write-контракта `add/update/delete`.

## 2. Нормативный CLI-контракт

### 2.1. Синтаксис

```bash
spec-cli query [options]
```

### 2.2. Опции команды

- `--type <entity_type>`: раннее сужение по типу, опция повторяемая.
- `--where-json <json>`: структурный фильтр в JSON.
- `--select <field>`: включить поле из read-namespace, опция повторяемая.
- `--sort <field[:asc|desc]>`: сортировка, опция повторяемая.
- `--limit <n>`: размер страницы, default `100`.
- `--offset <n>`: смещение страницы, default `0`.

### 2.3. Релевантные общие правила CLI

- Релевантные global options:
  - `--workspace <path>`: корень workspace, default `.`
  - `--schema <path>`: путь к schema file, default `spec.schema.yaml`
  - `--format <json|text>`: формат вывода, default `text`
  - `--require-absolute-paths`: если задан, любой явно переданный относительный filesystem path должен завершаться `INVALID_ARGS`
- CLI неинтерактивен.
- Для data-команд нормативным машинным режимом считается `--format json`.
- В `--format json` данные пишутся в `stdout`, диагностика в `stderr`.
- Любой нераспознанный аргумент команды должен завершаться ошибкой `INVALID_ARGS`.
- `--limit` и `--offset` обязаны быть целыми числами `>= 0`.
- Значение `-` для stdin или stdout не считается относительным путем, но для `query` это правило практически не используется.

### 2.4. Коды завершения

- `0`: успех.
- `1`: ожидаемый доменный fail команды.
- `2`: неверные CLI-аргументы или невалидный запрос.
- `3`: ошибка чтения или записи файлов.
- `4`: ошибка схемы.
- `5`: внутренняя ошибка утилиты.

Для `query` практически нужны такие ветви:

- `0` для валидного выполнения, включая `matched = 0`;
- `2` для `INVALID_ARGS`, `INVALID_QUERY`, `ENTITY_TYPE_UNKNOWN`;
- `3` для ошибок чтения workspace;
- `4` для невалидной или неполной схемы;
- `5` для дефектов реализации.

### 2.5. Формат ошибки в JSON

Любая ошибка в `--format json` должна иметь shape:

```json
{
  "result_state": "invalid",
  "error": {
    "code": "INVALID_ARGS",
    "message": "Unknown selector 'meta.unknown'",
    "exit_code": 2,
    "details": {}
  }
}
```

Минимально обязательны поля:

- `result_state`
- `error.code`
- `error.message`
- `error.exit_code`

## 3. Нормативный read-namespace

`query` и `get` используют один и тот же read-namespace.

### 3.1. Допустимые пути для `--select`

Built-in поля:

- `type`
- `id`
- `slug`
- `created_date`
- `updated_date`
- `revision`

Meta-пути:

- `meta`
- `meta.<name>`

Refs-пути:

- `refs`
- `refs.<field>`
- `refs.<field>.type`
- `refs.<field>.id`
- `refs.<field>.slug`

Content-пути:

- `content.raw`
- `content.sections`
- `content.sections.<name>`

### 3.2. Общие правила namespace

- Один и тот же path используется в `--select`, `--sort`, `where-json.field` и JSON-ответе.
- Object-node selector должен возвращать весь соответствующий подграф.
- Пересекающиеся selectors должны merge-иться без дублирования.
- Если запрошенная секция `content.sections.<name>` отсутствует, в ответе должно быть поле со значением `null`.
- В read-контракте `refs.<field>` возвращается как expanded object, а не как scalar id.

### 3.3. Пути, допустимые для `--sort`

Для `--sort` разрешены только leaf-пути, имеющие упорядочиваемое значение:

- built-in поля: `type`, `id`, `slug`, `created_date`, `updated_date`, `revision`
- `meta.<name>`
- `refs.<field>.type`
- `refs.<field>.id`
- `refs.<field>.slug`
- `content.raw`
- `content.sections.<name>`

Для `--sort` запрещены object-path:

- `meta`
- `refs`
- `refs.<field>`
- `content.sections`

Если в `--sort` передан path вне этого списка, команда должна завершаться `INVALID_ARGS`.

### 3.4. Пути, допустимые в `where-json`

Leaf-поля фильтра:

- `type`
- `id`
- `slug`
- `revision`
- `created_date`
- `updated_date`
- `meta.<name>`
- `refs.<field>.type`
- `refs.<field>.id`
- `refs.<field>.slug`
- `content.sections.<name>`

Дополнительные правила:

- `content.raw` в `where-json.field` запрещён;
- `content.sections.<name>` допускает только `contains`, `exists`, `not_exists`;
- фильтрация по `content.sections.<name>` используется как lexical discovery / coarse prefilter, а не semantic match.

Object-path в `where-json.field` запрещён.

## 4. Минимальный schema contract, необходимый для `query`

Источник истины для допустимых типов, selectors и типизации полей это загруженная стандартная схема (`schema.entity`).

Для реализации `query` достаточно такого минимального shape:

```yaml
version: "0.0.3"
entity:
  feature:
    id_prefix: FEAT
    path_pattern: "features/{slug}.md"
    meta:
      fields:
        status:
          schema:
            type: string
            enum: [draft, active, deprecated]
        owner:
          schema:
            type: entity_ref
            refTypes: [service]
    content:
      sections:
        summary: {}
        implementation: {}
```

Из этой модели `query` должен вывести:

- список допустимых `entity_type`;
- список допустимых selectors для `--select`;
- список допустимых leaf-полей для `--sort`;
- список допустимых leaf-полей для `where-json`;
- типы полей для проверки совместимости операторов;
- enum-значения для `meta.<name>`, если они объявлены;
- допустимые `refs.<field>` на основе metadata-полей типа `entity_ref`;
- допустимые `content.sections.<name>` на основе `content.sections`.

Правило:

- допустимые поля определяются только из `schema.entity` (+ встроенный read-contract);
- фактические документы workspace не должны расширять контракт.

## 5. Нормативный язык `--where-json`

### 5.1. Общий shape

`--where-json` принимает JSON-объект. Поддерживаются логические и leaf-узлы.

Логические узлы:

```json
{ "op": "and", "filters": [ ... ] }
{ "op": "or", "filters": [ ... ] }
{ "op": "not", "filter": { ... } }
```

Leaf-узел:

```json
{ "field": "meta.status", "op": "eq", "value": "active" }
```

Для `exists` и `not_exists` поле `value` запрещено:

```json
{ "field": "content.sections.summary", "op": "exists" }
```

### 5.2. Поддерживаемые операторы

- `eq`
- `neq`
- `in`
- `not_in`
- `exists`
- `not_exists`
- `gt`
- `gte`
- `lt`
- `lte`
- `contains`

### 5.3. Синтаксические правила валидации

- `--where-json` должен парситься как валидный JSON.
- Узел обязан содержать либо логическую форму, либо leaf-форму, но не обе сразу.
- В `and` и `or` поле `filters` обязательно и должно быть непустым массивом.
- В `not` поле `filter` обязательно и должно быть ровно одним вложенным узлом.
- В leaf-узле поля `field` и `op` обязательны.
- Для `exists` и `not_exists` поле `value` запрещено.
- Для всех остальных leaf-операторов поле `value` обязательно.
- Для `in` и `not_in` `value` обязано быть JSON-массивом.

Любое нарушение этих правил должно завершаться `INVALID_QUERY`.

### 5.4. Семантика операторов

- `eq`: literal equality.
- `neq`: literal inequality.
- `in`: текущее значение поля должно быть равно одному из элементов массива `value`.
- `not_in`: текущее значение поля не должно быть равно ни одному элементу массива `value`.
- `exists`: true, если значение присутствует.
- `not_exists`: true, если значение отсутствует.
- `gt`, `gte`, `lt`, `lte`: обычное сравнение значений.
- `contains`:
  - для строк: поиск подстроки;
  - для массивов: наличие literal-элемента.

### 5.5. Совместимость типов

Нужно валидировать не только форму JSON, но и совместимость `field`, `op` и `value`.

Правила:

- `gt/gte/lt/lte` допустимы только для чисел и дат формата `YYYY-MM-DD`.
- Даты сравниваются лексикографически как строки.
- `contains` допустим для строк и массивов.
- Если оператор применяется к несовместимому типу, это `INVALID_QUERY`, а не `false`.
- Для enum-полей `eq`, `neq`, `in`, `not_in` должны принимать только значения из enum, с учётом регистра.
- `content.sections.<name>` допускает только `contains`, `exists`, `not_exists`.
- `content.raw` не допускается в `where-json.field`.

Практическая таблица:

- строки (кроме `content.sections.<name>`): `eq`, `neq`, `in`, `not_in`, `contains`, `exists`, `not_exists`
- даты `YYYY-MM-DD`: `eq`, `neq`, `in`, `not_in`, `gt`, `gte`, `lt`, `lte`, `contains`, `exists`, `not_exists`
- числа: `eq`, `neq`, `in`, `not_in`, `gt`, `gte`, `lt`, `lte`, `exists`, `not_exists`
- массивы: `eq`, `neq`, `in`, `not_in`, `contains`, `exists`, `not_exists`
- `content.sections.<name>`: `contains`, `exists`, `not_exists`

### 5.6. Семантика отсутствующего значения

- `exists` возвращает `true`, если значение присутствует.
- `not_exists` возвращает `true`, если значение отсутствует.
- Все остальные leaf-операторы при отсутствии значения возвращают `false`.

### 5.7. Примеры допустимых фильтров

По типу и статусу:

```json
{
  "op": "and",
  "filters": [
    { "field": "type", "op": "eq", "value": "feature" },
    { "field": "meta.status", "op": "eq", "value": "active" }
  ]
}
```

По дате:

```json
{
  "field": "updated_date",
  "op": "gte",
  "value": "2026-03-01"
}
```

Проверка наличия секции:

```json
{
  "field": "content.sections.summary",
  "op": "exists"
}
```

Лексический prefilter по секции:

```json
{
  "field": "content.sections.summary",
  "op": "contains",
  "value": "retry"
}
```

## 6. Нормативный ответ `query`

### 6.1. Успешный JSON-ответ

```json
{
  "result_state": "valid",
  "items": [
    {
      "type": "feature",
      "id": "FEAT-8",
      "slug": "retry-window",
      "meta": {
        "status": "active"
      }
    }
  ],
  "matched": 1,
  "page": {
    "mode": "offset",
    "limit": 100,
    "offset": 0,
    "returned": 1,
    "has_more": false,
    "next_offset": null,
    "effective_sort": ["type:asc", "id:asc"]
  }
}
```

Обязательные поля успешного ответа:

- `result_state`
- `items`
- `matched`
- `page.mode`
- `page.limit`
- `page.offset`
- `page.returned`
- `page.has_more`
- `page.next_offset`
- `page.effective_sort`

### 6.2. Правила ответа

- При успехе `result_state` всегда равно `valid`.
- Если совпадений нет, команда всё равно успешна: `result_state = "valid"`, `matched = 0`, `items = []`.
- Если `--select` не задан, default projection это `type`, `id`, `slug`.
- Если `--sort` не задан, default sort это `type:asc`, затем `id:asc`.
- Если `--limit 0`, команда возвращает агрегаты страницы без элементов.

### 6.3. Зафиксированные решения для неоднозначных мест

Чтобы документ был достаточным без внешних уточнений, дополнительно фиксируются такие правила:

- При `--limit 0` поле `items` обязательно и равно `[]`.
- Если `offset >= matched`, то:
  - `items = []`
  - `page.returned = 0`
  - `page.has_more = false`
  - `page.next_offset = null`
- При пользовательском `--sort` реализация обязана дополнять список скрытым хвостом `type:asc`, `id:asc`, если итоговый список не заканчивается ровно этим хвостом.
- Если пользователь уже указал `type` или `id` раньше в списке сортировок с другим направлением, это не запрещено; скрытый хвост всё равно добавляется только как tie-breaker в конец.
- Для сортировки отсутствующее значение считается меньше присутствующего при `asc` и больше присутствующего при `desc`.
- Сортировка должна быть детерминированной и стабильной относительно одинаковых ключей.

## 7. Карта ошибок команды

### 7.1. `INVALID_ARGS`

Использовать в случаях:

- неизвестный selector в `--select`;
- неизвестный или object-path в `--sort`;
- невалидный синтаксис `--sort`;
- `--limit` или `--offset` не являются целыми числами `>= 0`;
- неизвестные CLI-аргументы.

### 7.2. `INVALID_QUERY`

Использовать в случаях:

- `--where-json` не парсится как JSON;
- логическая структура фильтра невалидна;
- пустой `filters` в `and` или `or`;
- неизвестный оператор;
- неизвестное поле в `where-json`;
- object-path в `where-json.field`;
- некорректная комбинация `field/op/value`;
- type mismatch;
- неверное enum-значение;
- `exists/not_exists` содержит `value`;
- `in/not_in` не получил массив;
- `gt/gte/lt/lte` применён не к числу и не к дате.
- `content.raw` использован в `where-json.field`;
- для `content.sections.<name>` использован оператор вне `contains|exists|not_exists`.

Рекомендованная машинная диагностика для policy-ban в `error.details`:

- `arg = "--where-json"`
- для `content.raw`: `reason = "forbidden_field"`, `field = "content.raw"`
- для запрещённого оператора на `content.sections.<name>`: `reason = "forbidden_operator_for_field"`, `field`, `operator`

### 7.3. `ENTITY_TYPE_UNKNOWN`

Использовать, если любое значение `--type` отсутствует в `schema.entity`.

## 8. Что должно существовать в runtime

Для реализации `query` нужны следующие примитивы.

### 8.1. Загрузка схемы

Должен существовать загрузчик, который отдаёт effective стандартную схему и умеет валидировать минимальную структуру, нужную `query`.

Минимально обязательны:

- `entity`
- `entity.<type>.meta.fields`
- `entity.<type>.meta.fields.<name>.schema.type`
- `entity.<type>.meta.fields.<name>.schema.enum` (если есть)
- `entity.<type>.content.sections`, если тип поддерживает секции

### 8.2. Чтение workspace

Должен существовать перечислитель сущностей workspace, который для каждой сущности умеет получить:

- built-in поля;
- metadata;
- ссылки для expanded read-view;
- raw content;
- разобранные `content.sections`.

### 8.3. Построение full read-view

Для каждой сущности нужен канонический read-view, из которого потом делаются:

- фильтрация;
- сортировка;
- final projection.

Рекомендованный full read-view:

```json
{
  "type": "feature",
  "id": "FEAT-8",
  "slug": "retry-window",
  "revision": "sha256:def456",
  "created_date": "2026-03-10",
  "updated_date": "2026-03-10",
  "meta": {
    "status": "active",
    "owner": "platform"
  },
  "refs": {
    "container": {
      "type": "service",
      "id": "SVC-2",
      "slug": "billing-api"
    }
  },
  "content": {
    "raw": "## Summary\nRetry window...",
    "sections": {
      "summary": "Retry window...",
      "implementation": "Use backoff..."
    }
  }
}
```

### 8.4. Единый JSON writer

Нужен общий writer для:

- успешных JSON-ответов;
- error-ответов с `result_state`, `error.code`, `error.message`, `error.exit_code`.

## 9. План реализации

### Этап 1. Нормализовать вход команды

Собрать `QueryRequest`:

- `types[]`
- `where_json_raw`
- `selects[]`
- `sorts[]`
- `limit`
- `offset`

Сразу валидировать:

- defaults для `limit` и `offset`;
- целочисленность и неотрицательность `limit/offset`;
- базовый синтаксис `field[:asc|desc]` в `--sort`.

Результат:

- единая точка формирования `INVALID_ARGS` для проблем аргументов.

### Этап 2. Построить `QuerySchemaIndex`

Из стандартной схемы (`schema.entity`) собрать индекс, который знают все последующие шаги:

- допустимые `entity_type`;
- допустимые selectors;
- допустимые sort fields;
- допустимые filter fields;
- тип каждого leaf-поля;
- enum-ограничения;
- mapping metadata `entity_ref` -> `refs.<field>.type|id|slug`;
- mapping `content.sections.<name>`.

Рекомендуемая структура:

```text
QuerySchemaIndex
  entityTypes
  selectorSpecs
  sortFieldSpecs
  filterFieldSpecs
  enumSpecs
```

### Этап 3. Реализовать projector для `--select`

Нужны:

- validator selectors через `QuerySchemaIndex`;
- canonicalization paths;
- merge пересекающихся selectors;
- projector из full read-view в response item.

Нормативные правила:

- default projection: `type`, `id`, `slug`;
- unknown selector -> `INVALID_ARGS`;
- object-node selector возвращает весь подграф;
- отсутствующая `content.sections.<name>` -> `null`.

### Этап 4. Реализовать parser и binder для `--where-json`

Разделить работу на два шага:

1. parse raw JSON -> untyped AST;
2. bind AST к `QuerySchemaIndex` -> typed AST.

После bind-шага должно быть известно:

- что все поля допустимы;
- что оператор допустим для типа поля;
- что `value` имеет корректную shape;
- что enum-значения валидны.

Любая ошибка этого этапа должна завершаться `INVALID_QUERY`.

### Этап 5. Реализовать evaluator фильтра

На вход evaluator получает:

- typed AST;
- full read-view сущности.

Нужен helper:

```text
resolve_read_value(entityView, path) -> { present, value }
```

Он должен одинаково работать для:

- built-in;
- `meta.<name>`;
- `refs.<field>.*`;
- `content.sections.<name>` для `where-json`;
- `content.raw` и `content.sections.<name>` для `--sort` / `--select`.

### Этап 6. Реализовать pipeline выборки

Рекомендуемый порядок:

1. загрузить схему (`schema.entity`);
2. построить `QuerySchemaIndex`;
3. провалидировать `--type`;
4. провалидировать `--select`;
5. провалидировать и bind-ить `--where-json`, если он задан;
6. перечислить сущности workspace;
7. рано отфильтровать по `--type`;
8. для оставшихся сущностей построить full read-view;
9. применить `where-json`;
10. отсортировать matched set;
11. применить `offset/limit`;
12. спроецировать `items` через `--select`;
13. собрать итоговый JSON-ответ.

Причина такого порядка:

- full read-view нужен для фильтрации и сортировки;
- projection должна выполняться в конце, чтобы выбор `--select` не влиял на фильтр и sort.

### Этап 7. Реализовать сортировку и пагинацию

Для сортировки нужны:

- `QuerySortTerm { path, direction }`;
- default sort `type:asc`, `id:asc`;
- hidden suffix `type:asc`, `id:asc`;
- comparator с детерминированной обработкой отсутствующих значений.

Для пагинации нужны:

- `matched`: количество элементов после фильтра и до paging;
- `returned`: количество элементов после paging;
- `has_more`: `matched > offset + returned`;
- `next_offset`: `offset + returned`, если `has_more`, иначе `null`;
- `effective_sort`: итоговый список sort terms после добавления скрытого хвоста.

### Этап 8. Сформировать ответ и error mapping

Успех:

- `result_state = "valid"`
- `items`
- `matched`
- `page`

Ошибки:

- `INVALID_ARGS`
- `INVALID_QUERY`
- `ENTITY_TYPE_UNKNOWN`
- инфраструктурные ошибки с корректным `exit_code`

### Этап 9. Обновить help для `query`

Справка по `query` обязана явно описывать:

- секции в фиксированном порядке: `Command`, `Syntax`, `Options`, `Rules`, `Examples`, `Schema`;
- канонический синтаксис команды;
- все опции;
- какие аргументы используют read-namespace;
- полный язык `--where-json`;
- допустимые логические узлы;
- допустимые leaf-операторы;
- допустимые поля;
- правила типизации;
- семантику отсутствующего значения;
- короткие копируемые примеры;
- effective path схемы и полный verbatim-текст загруженной схемы в секции `Schema`.

## 10. Тестовая матрица

Минимальный набор тестов.

### 10.1. Базовый happy path

- запрос без `--where-json`;
- default projection;
- default sort;
- `matched = 0`;
- несколько типов сущностей без `--type`;
- раннее сужение по одному `--type`;
- раннее сужение по нескольким `--type`.

### 10.2. `--select`

- один leaf selector;
- несколько leaf selectors;
- object-node selector `meta`;
- object-node selector `refs.container`;
- merge пересекающихся selectors;
- unknown selector -> `INVALID_ARGS`;
- `content.sections.<name>` отсутствует -> `null`;
- `refs.<field>` возвращается как expanded object.

### 10.3. `--where-json`

- `eq`, `neq`;
- `in`, `not_in`;
- `exists`, `not_exists`;
- `gt`, `gte`, `lt`, `lte` на дате;
- `gt`, `gte`, `lt`, `lte` на числе;
- `contains` на `content.sections.<name>`;
- `contains` на массиве, если схема содержит массивы;
- `and`, `or`, `not`;
- вложенные логические выражения;
- отсутствие значения для leaf-операторов кроме `exists/not_exists`;
- unknown operator -> `INVALID_QUERY`;
- unknown field -> `INVALID_QUERY`;
- `content.raw` в `where-json.field` -> `INVALID_QUERY`;
- `content.sections.<name>` с `eq/neq/in/not_in/gt/gte/lt/lte` -> `INVALID_QUERY`;
- object-path в `where-json.field` -> `INVALID_QUERY`;
- пустой `and/or` -> `INVALID_QUERY`;
- `exists/not_exists` с `value` -> `INVALID_QUERY`;
- `in/not_in` без массива -> `INVALID_QUERY`;
- type mismatch -> `INVALID_QUERY`;
- неверное enum-значение -> `INVALID_QUERY`.

### 10.4. Сортировка

- sort по одному полю;
- sort по нескольким полям;
- default sort;
- пользовательский sort + hidden tail;
- sort по отсутствующим значениям;
- object-path в `--sort` -> `INVALID_ARGS`;
- невалидное направление сортировки -> `INVALID_ARGS`.

### 10.5. Пагинация

- `limit = 0`;
- `limit > 0`;
- `offset = 0`;
- `offset` внутри диапазона;
- `offset >= matched`;
- корректность `returned`;
- корректность `has_more`;
- корректность `next_offset`;
- корректность `effective_sort`.

### 10.6. Ошибки и инфраструктура

- неизвестный `entity_type` -> `ENTITY_TYPE_UNKNOWN`;
- невалидный JSON в `--where-json` -> `INVALID_QUERY`;
- невалидный `--limit` -> `INVALID_ARGS`;
- невалидный `--offset` -> `INVALID_ARGS`;
- ошибка чтения файла workspace -> exit code `3`;
- ошибка схемы -> exit code `4`.

## 11. Рекомендуемый порядок поставки

Чтобы быстрее получить рабочий вертикальный срез без переписывания:

1. `QueryRequest` и парсинг аргументов.
2. `QuerySchemaIndex`.
3. Full read-view builder.
4. `--select` projector.
5. sort и pagination без `--where-json`.
6. parser, binder и evaluator `--where-json`.
7. error mapping.
8. `help query`.
9. негативные и граничные тесты.

## 12. Definition of Done

`query` считается реализованной, когда одновременно выполнены условия:

- команда принимает все опции из этого документа;
- все selectors, sort fields и filter fields валидируются через стандартную схему (`schema.entity`, `meta.fields`, `content.sections`);
- `--where-json` поддерживает весь описанный здесь язык;
- `content.raw` запрещён в `where-json`, а для `content.sections.<name>` разрешены только `contains|exists|not_exists`;
- сортировка и offset-пагинация детерминированы;
- `items`, `matched` и `page.*` соответствуют контракту;
- ошибки маппятся в правильные `code` и `exit_code`;
- `help query` описывает read-namespace и язык фильтра без внешней документации;
- тесты из минимальной матрицы проходят.
