# План реализации команды `update`

## 1. Источники и границы

Этот план составлен только по двум документам:

- `SPEC_UTILITY_CLI_API_BASELINE_RU.md`
- `SPEC_STANDARD_RU_REVISED_V3.md`

Цель: реализовать baseline-команду `spec-cli update`, которая частично обновляет существующую сущность, валидирует итоговое состояние и пишет результат атомарно.

Вне текущего плана:

- любые документы вне двух перечисленных выше;
- расширения CLI вне baseline-профиля;
- orchestration/streaming-сценарии;
- раскрытие filesystem path в публичном контракте команды.

## 2. Что именно должно уметь `update`

### 2.1. Внешний контракт

Команда должна поддерживать вызов:

```bash
spec-cli update [options] --id <id>
```

Опции baseline:

- `--id <id>`
- `--set <path=value>`
- `--set-file <path=filepath>`
- `--unset <path>`
- `--content-file <path>`
- `--content-stdin`
- `--clear-content`
- `--expect-revision <token>`
- `--dry-run`

Обязательные правила:

1. `--id` обязателен.
2. Должна быть задана хотя бы одна patch-операция.
3. `--content-file`, `--content-stdin`, `--clear-content` взаимоисключающие.
4. `--set-file` допускается только для `content.sections.<name>`; использование `--set-file` для `meta.<field>` и `refs.<field>` должно завершаться `WRITE_CONTRACT_VIOLATION`.
5. `--set` / `--set-file` / `--unset` для `content.sections.<name>` нельзя смешивать с whole-body операциями.
6. Для типов без блока `content` опции `--content-file`, `--content-stdin`, `--clear-content` должны завершаться `WRITE_CONTRACT_VIOLATION`.
7. `update` не должен зависеть от `schema.model` как runtime-источника write-контракта.
8. Допустимые path для `--set` выводятся напрямую из raw schema типа сущности:
   - `meta.<field>` только для полей из `meta.fields`, где `schema.type != entity_ref`;
   - `refs.<field>` только для полей из `meta.fields`, где `schema.type = entity_ref`;
   - `content.sections.<name>` только для секций из `content.sections`.
9. Допустимые path для `--unset` определяются тем же writable namespace, что и для `--set`.
10. Нормативно запрещенные пути записи определяются baseline-правилами write-namespace и raw schema целевого типа.
11. До применения patch должна выполняться path/value-проверка для каждого write-path.
12. При несовпадении `--expect-revision` запись не производится.
13. После применения patch итоговая сущность должна пройти полную валидацию.
14. При невалидном post-patch состоянии запись не производится.
15. При отсутствии фактических изменений команда должна вернуть успешный `no-op` с `updated: false`, `noop: true`, `changes[]: []`.
16. При фактическом изменении `updated_date` выставляется автоматически.
17. Если канонический путь сущности изменился, внутреннее перемещение должно быть частью той же атомарной операции.
18. Успешный `--dry-run` должен проходить тот же pipeline, что и обычный `update`, возвращать тот же success-payload, но не писать на диск и выставлять `dry_run: true`.

### 2.2. Содержательная семантика

Нужно реализовать именно частичное обновление, а не "прочитать файл и заново сгенерировать его целиком из модели" без учета структуры исходного документа.

Это означает два разных режима:

1. Whole-body режим.
   `--content-file`, `--content-stdin`, `--clear-content` заменяют весь body как literal string.

2. Path-based режим.
   `--set` и `--unset` изменяют только конкретные writable slots:
   - `meta.<field>`
   - `refs.<field>`
   - `content.sections.<name>`
   `--set-file` допустим только для `content.sections.<name>` и задает тело секции literal-содержимым файла.

Для `content.sections.<name>` значение всегда означает тело секции без heading.

### 2.3. Что считается изменением

Нужно разделить:

1. Пользовательские изменения.
   Это изменения writable-полей и/или body.

2. Производные изменения.
   Это пересчет `updated_date`, `revision`, возможное внутреннее перемещение файла.

`changes[]` должен описывать именно фактически примененные пользовательские изменения. Производные изменения в `changes[]` включать не нужно.

## 3. Зависимости и внутренние компоненты

Для `update` нужен следующий минимальный набор внутренних подсистем.

### 3.1. Schema loader + internal write model

`update` не должен зависеть от `schema.model` как runtime-источника write-контракта.
Реализация должна выводить write-контракт напрямую из raw schema типа сущности и иметь внутреннее представление, достаточное для:

- описания writable path и их `kind` / `value_kind`;
- вывода допустимых path для `--set`;
- вывода допустимых path для `--unset`, совпадающих с тем же writable namespace;
- вывода нормативно запрещенных форм записи по baseline-правилам write-namespace и raw schema.

Практически это лучше сделать так:

1. Загружать raw schema.
2. Строить internal schema model один раз.
3. Для `update` использовать уже нормализованный internal write model.

Это снимет расхождение между raw schema как источником истины, `schema` как discovery-командой и runtime-валидацией `update`.

### 3.2. Repository/index layer

Нужен доступ к workspace-индексам:

- `id -> entity location`
- `(type, slug) -> entity`
- `path -> entity`
- индекс сущностей для резолва `entity_ref`

`update` не сможет корректно работать без этих индексов, потому что ему нужны:

- поиск сущности по `--id`;
- проверка `entity_ref`;
- проверка уникальности `slug`;
- проверка path conflict при возможном move.

### 3.3. Markdown entity parser

Парсер должен уметь:

1. Выделять `YAML frontmatter`.
2. Разбирать его по YAML 1.2.2 с запретом duplicate keys.
3. Разделять built-in поля и schema-driven metadata.
4. Держать body в raw-виде.
5. Строить нормализованную модель секций:
   - `label`
   - `title`
   - диапазон заголовка
   - диапазон тела секции

Без этого нельзя реализовать section patch без неявной перезаписи всего документа.

### 3.4. Serializer

Нужен один детерминированный serializer, который отвечает за:

- порядок frontmatter-полей;
- формат delimiters frontmatter;
- newline policy;
- сборку документа после patch;
- вычисление `revision` по точным байтам сериализованного документа.

Для всех write-команд следует использовать один и тот же serializer. Иначе `revision` и поведение `--expect-revision` начнут расходиться между командами.

### 3.5. Validator

Нужны две стадии валидации:

1. Preflight write validation.
   Проверяет корректность аргументов patch и их базовую совместимость с write-контрактом.

2. Full post-patch validation.
   Проверяет итоговую сущность по стандарту и baseline, включая:
   - built-in поля;
   - `meta.fields`;
   - `entity_ref`;
   - `content.sections`;
   - `path_pattern`;
   - уникальность `slug`;
   - корректность канонического пути;
   - path conflict.

## 4. Предлагаемый runtime pipeline `update`

### Шаг 1. Разбор CLI-аргументов

На этом шаге нужно:

1. Проверить обязательность `--id`.
2. Проверить, что задана хотя бы одна patch-операция.
3. Проверить mutually-exclusive группы.
4. Собрать все операции в одну нормализованную структуру:
   - `set_ops[]`
   - `set_file_ops[]`
   - `unset_ops[]`
   - `body_op` (`replace_file`, `replace_stdin`, `clear`, либо `none`)

Сразу же надо ловить чисто синтаксические ошибки CLI:

- некорректный формат `path=value`;
- пустой path;
- повтор одного и того же path;
- одновременное использование одного path в `set` и `unset`.

Это слой `INVALID_ARGS`, а не доменная write-валидация.

### Шаг 2. Загрузка схемы и построение write-контракта

Нужно:

1. Прочитать effective schema.
2. Построить/получить internal schema model.
3. Найти тип сущности только после чтения самой сущности, но shared schema infrastructure должна быть готова уже здесь.

Важно: для `update` набор допустимых write-path зависит от типа конкретной сущности, поэтому финальная path-валидация откладывается до чтения target entity.

### Шаг 3. Поиск target entity по `--id`

Нужно:

1. Поднять индекс workspace.
2. Найти сущность по `id`.
3. Если сущность не найдена, завершать с `ENTITY_NOT_FOUND`.

Команда должна опираться на внутренний repository path, но этот path не должен попадать в публичный JSON-ответ.

### Шаг 4. Парсинг текущего документа

Нужно получить in-memory представление:

- `type`
- `id`
- `slug`
- `created_date`
- `updated_date`
- metadata fields
- raw body
- normalized sections
- текущий internal path

Тут важный нюанс: `update` должен уметь исправлять невалидную сущность, если документ все еще можно детерминированно разобрать и сопоставить с типом. Поэтому не нужно требовать полной валидности текущего состояния до patch.

Fail-fast только если невозможно:

- прочитать файл;
- распарсить frontmatter;
- определить `type`;
- определить структуру, достаточную для применения patch.

### Шаг 5. Вычисление текущего `revision` и проверка `--expect-revision`

Нужно:

1. Взять текущий persisted-документ как источник истины для текущего `revision`, без канонической пересборки parsed model.
2. Применить к его содержимому только штатную newline-policy сериализатора.
3. Посчитать `revision` по точным байтам этого представления.
4. Если задан `--expect-revision` и токен не совпал, вернуть `CONCURRENCY_CONFLICT` без записи.

Сравнение должно идти до любого изменения in-memory state.

### Шаг 6. Path/value preflight validation

После определения `type` нужно поднять type-specific write-контракт:

- `set_paths`
- `unset_paths`, совпадающие с тем же writable namespace
- `path_specs`
- `forbidden_path_patterns`, выведенные из baseline-правил write-namespace и raw schema

Для каждого patch-элемента:

1. Проверить, что path разрешен для данной операции.
2. Проверить, что path не попадает под forbidden patterns.
3. Проверить, что операция соответствует kind path:
   - `meta`
   - `ref`
   - `section`

Разбор значений:

1. `--set meta.<field>=...`
   RHS разбирается как YAML value-node с тем же профилем типизации, что и frontmatter.
   Допустимы все формы значения, разрешенные `schema.type` целевого поля.
   Для `schema.type: array` допустим YAML array.
   `type: object` не поддерживается стандартом и не должен приниматься.

2. `--set refs.<field>=...`
   RHS трактуется как literal string `id`.

3. `--set content.sections.<name>=...`
   RHS трактуется как literal string тела секции без heading.

4. `--set-file`
   Допустим только для `content.sections.<name>`.
   Для `meta.<field>` и `refs.<field>` должен возвращаться `WRITE_CONTRACT_VIOLATION`.
   Содержимое файла сначала читается "как есть", без trim и без escape-интерпретации.
   Затем применяется та же path-specific семантика, что и для `--set`.

5. `--unset`
   Проверяется только допустимость path и операции.

На этом шаге не нужно пытаться выполнить всю schema-валидацию. Достаточно:

- подтвердить shape значения;
- убедиться, что его можно применить к данному path;
- отсечь явные write-contract violations.

Полная проверка `enum`, `const`, `required_when`, `entity_ref`, `path_pattern` и секций должна идти после применения patch к целостному кандидату.

### Шаг 7. Применение patch к in-memory модели

#### 7.1. Metadata patch

Для `meta.<field>`:

1. `set` заменяет значение поля.
2. `unset` удаляет ключ из frontmatter.
3. Если поле отсутствует, `unset` считается успешным `no-op`.

#### 7.2. Ref patch

Для `refs.<field>`:

1. `set` заменяет строковый `id` в frontmatter-поле соответствующего `entity_ref`.
2. `unset` допускается для любого `refs.<field>`, входящего в writable namespace `update`.
3. Если поле отсутствует, `unset` считается успешным `no-op`.

Во внутренней модели `refs.<field>` не должно жить как отдельный persisted object. Persisted form остается исходным frontmatter key со строкой `id`.

#### 7.3. Section patch

Для `content.sections.<name>`:

1. `set` заменяет тело существующей секции, сохраняя heading.
2. `set` для отсутствующей секции создает секцию.
3. `unset` удаляет секцию целиком.
4. Если удаляемая секция отсутствует, операция считается успешным `no-op`.

Создание отсутствующей секции должно использовать:

- канонический порядок из `content.sections` схемы;
- heading вида `## <title> {#<label>}`;
- первый допустимый `title` из схемы, если `title` задан списком;
- сам `label`, если `title` не задан.

Ключевой момент реализации: path-based section patch должен сохранять остальной body максимально нетронутым.

Нужен алгоритм вставки:

1. Найти ближайшую уже существующую schema-section до новой секции по каноническому порядку.
2. Если нашли, вставить после нее.
3. Иначе найти ближайшую следующую schema-section и вставить перед ней.
4. Если ни одной schema-section нет, добавить в конец body по штатной spacing policy serializer.

Unknown sections и прочий body-контент трогать не нужно.

#### 7.4. Whole-body patch

Если задан one-shot body operation:

- `--content-file` -> body заменяется содержимым файла;
- `--content-stdin` -> body заменяется содержимым stdin;
- `--clear-content` -> body становится пустой строкой.

В этом режиме section-level patch отсутствует по контракту CLI.
Для типов без блока `content` такие операции должны быть отсеяны на preflight-этапе как `WRITE_CONTRACT_VIOLATION`.

### Шаг 8. Вычисление semantic no-op

После применения patch, но до автоматического обновления `updated_date`, нужно сравнить исходное и новое пользовательское состояние:

- built-in writable-semantic subset: `slug` сюда не входит, потому что он readonly для `update` по write-контракту;
- metadata fields;
- raw body;
- persisted ref values.

Если пользовательское состояние не изменилось:

1. Пометить кандидата как `noop`.
2. Не менять `updated_date`.
3. `changes[]` должен быть пустым.

Важно: `noop` нельзя возвращать до полной post-patch валидации. Иначе команда сможет "успешно" завершиться на уже невалидной сущности, если patch ничего фактически не поменял.

### Шаг 9. Автоматическое обновление `updated_date`

Если semantic diff не пуст:

1. Взять текущую календарную дату из инжектируемого clock source.
2. Записать ее в `updated_date`.

Из-за дневной гранулярности возможна ситуация, когда значение `updated_date` до и после совпадает строково, если апдейт происходит в тот же день. Это нормально: факт изменения будет отражен в `revision` и в содержимом документа.

### Шаг 10. Полная post-patch валидация кандидата

Здесь нужно прогнать тот же validation pipeline, который требуется стандартом для entity-level проверки:

1. Built-in поля:
   - `type`
   - `id`
   - `slug`
   - `created_date`
   - `updated_date`

2. `meta.fields`:
   - `required`
   - `required_when`
   - `schema.type`
   - `schema.const`
   - `schema.enum`
   - array constraints
   - `entity_ref`

3. `content.sections`:
   - required/required_when
   - наличие label
   - допустимость title

4. `path_pattern`:
   - выбор кейса
   - подстановка плейсхолдеров
   - проверка итогового канонического пути

5. Workspace-level invariants:
   - глобальная уникальность `id`
   - уникальность `slug` внутри типа
   - path conflict
   - успешный резолв ссылок

Если кандидат невалиден:

1. Не писать на диск.
2. Вернуть top-level `error`.
3. По возможности вложить diagnostics в `error.details.validation.issues[]`.

Если кандидат валиден и ранее был помечен как `noop`:

1. Не писать на диск.
2. Вернуть успешный ответ с `noop: true`.
3. Оставить `changes[]` пустым.

### Шаг 11. Вычисление итогового path и стратегии move/write

Канонический path нужно вычислять только для валидного post-patch кандидата.

Дальше два случая.

#### Случай A. Path не изменился

Нужно:

1. Сериализовать документ.
2. Посчитать итоговый `revision`.
3. При `dry-run` не писать файл.
4. При обычном запуске записать через temp-file + atomic rename поверх исходного файла.

#### Случай B. Path изменился

Нужно:

1. Проверить конфликт target path.
2. Подготовить parent directories.
3. Выполнить внутреннее перемещение как часть одной write-транзакции.

Практический план транзакции:

1. Сериализовать новый документ во временный файл на том же filesystem.
2. Если target path занят другой сущностью, вернуть `PATH_CONFLICT`.
3. Подготовить rollback plan.
4. Переместить/заменить файл так, чтобы при ошибке можно было восстановить исходное состояние.
5. После успешного commit удалить старый path, если он отличается от нового.

Минимальное требование здесь не "идеальная lock-free многопроцессная транзакция", а гарантия baseline:

- при неуспехе итоговое состояние на диске не меняется;
- публичный контракт не раскрывает внутренние path-операции.

### Шаг 12. Формирование `changes[]`

`changes[]` должен строиться по semantic diff между исходным и post-patch состоянием до/после.

Правила:

1. Не включать записи без фактического изменения.
2. Для scalar/ref path использовать:
   - `field`
   - `op`
   - `before`
   - `after`

3. Для `content.sections.<name>` использовать:
   - `field`
   - `op`
   - `before_present`
   - `after_present`
   - `before_hash`
   - `after_hash`

4. Hash секции считать по точному строковому телу секции без heading.
5. Формат section hash должен совпадать с форматом `revision` токена.
6. Для whole-body операций добавлять synthetic entry с полями `field`, `op`, `before_hash`, `after_hash`, где `field` равен `"content.raw"`.
7. `content.raw` в `changes[]` не является write-path и используется только как diff-representation ответа.

Для `refs.<field>` в `changes[]` нужно показывать именно scalar `id`, а не expanded read-view.

### Шаг 13. Формирование success response

Успешный ответ должен включать:

- `result_state: "valid"`
- `dry_run`
- `updated`
- `noop`
- `changes[]`
- `entity`
- `validation`

В `entity` нужно возвращать:

- built-in поля;
- `revision`;
- `meta`;
- `refs` только в краткой форме `{ "id": "<target_id>" }`.

Если операция была `noop`:

- `updated` вернуть `false`;
- `noop` вернуть `true`;
- `changes[]` вернуть пустым.

## 5. Детали по serializer и persisted shape

### 5.1. Frontmatter

Для детерминированного write-поведения `update` должен использовать тот же канонический порядок frontmatter-полей, что уже зафиксирован для `add`:

1. `type`
2. `id`
3. `slug`
4. `created_date`
5. `updated_date`
6. затем `meta.fields` в порядке объявления в raw schema

Этот порядок является обязательным для serializer `update`.

### 5.2. Persisted refs

В persisted document ссылочное поле остается обычным frontmatter-полем со строковым `id`.

Read-view `refs.<field>` как объект не должен протекать в формат хранения.

### 5.3. Body serialization

Spacing/newline policy serializer должна быть зафиксирована так:

- итоговый Markdown сериализуется с platform-native newline policy;
- на Unix-подобных системах используется `\n`;
- на Windows используется `\r\n`;
- если body непустой, между closing frontmatter delimiter и body должна быть ровно одна пустая строка;
- если body пустой, после frontmatter не должен добавляться лишний пустой блок;
- при вставке новой schema-section она должна отделяться от соседнего контента ровно одной пустой строкой;
- файл всегда должен заканчиваться trailing newline;
- неизвестные секции и нетронутый body не должны переразмечаться без необходимости.

Именно эти байты участвуют в `revision`.

## 6. Набор тестов

Тесты нужно строить не только как unit, но и как contract/integration на уровне CLI.

### 6.1. Аргументы и write-контракт

Нужны кейсы:

1. Нет `--id`.
2. Нет patch-операций.
3. Смешаны mutually-exclusive whole-body опции.
4. Смешаны whole-body и section patch.
5. Повтор path.
6. Один path одновременно в `set` и `unset`.
7. Path отсутствует в `set_paths`.
8. Path отсутствует в writable namespace `--unset`.
9. Path попадает под forbidden pattern.
10. `--set-file` используется вне `content.sections.<name>`.
11. `--content-file` / `--content-stdin` / `--clear-content` используется для типа без `content`.

### 6.2. Значения patch-операций

Нужны кейсы:

1. `meta.<field>` с валидным YAML scalar.
2. `meta.<field>` с валидным YAML array для поля `schema.type: array`.
3. `meta.<field>` с type mismatch.
4. `refs.<field>` с literal `id`.
5. `refs.<field>` с несуществующей сущностью.
6. `content.sections.<name>` через `--set`.
7. `content.sections.<name>` через `--set-file`.
8. `--unset content.sections.<name>` для отсутствующей секции.

### 6.3. No-op и derived fields

Нужны кейсы:

1. `set` того же самого значения.
2. `unset` отсутствующего необязательного metadata field.
3. `unset` отсутствующего `refs.<field>`.
4. `unset` отсутствующей секции.
5. Проверка, что при `noop` нет записи на диск.
6. Проверка, что `updated_date` не меняется при `noop`.
7. Проверка, что успешный `noop` возвращает `updated: false`, `noop: true`, `changes[]: []`.
8. Проверка, что `noop` на уже невалидной сущности не обходит post-patch validation.

### 6.4. Post-patch validation

Нужны кейсы:

1. Patch делает обязательное поле отсутствующим.
2. Patch ломает `required_when`.
3. Patch ломает `entity_ref`.
4. Patch ломает `content.sections`.
5. Patch приводит к конфликту `slug`.
6. Patch меняет путь по `path_pattern`.
7. Patch приводит к path conflict.

### 6.5. Whole-body режим

Нужны кейсы:

1. `--content-file` заменяет весь body.
2. `--content-stdin` заменяет весь body.
3. `--clear-content` очищает body.
4. Whole-body patch делает сущность невалидной по секциям.
5. Whole-body операция для типа без `content` завершается `WRITE_CONTRACT_VIOLATION`.

### 6.6. Concurrency и revision

Нужны кейсы:

1. Верный `--expect-revision`.
2. Неверный `--expect-revision`.
3. `revision` меняется при изменении body.
4. `revision` меняется при изменении frontmatter.
5. `revision` не меняется при `noop`.
6. Проверка, что `--expect-revision` использует `revision` текущего persisted-документа, а не канонической пересборки parsed model.

### 6.7. Atomic write и move

Нужны кейсы:

1. Update без смены path.
2. Update со сменой path.
3. Dry-run без записи.
4. Имитация ошибки записи и проверка rollback.
5. Создание отсутствующих parent directories при move.

## 7. Последовательность реализации

Реализацию лучше делать не одной задачей, а короткими законченными срезами.

### Этап 1. Инфраструктура

1. Довести internal schema model до уровня, достаточного для `update`.
2. Довести repository/index layer.
3. Зафиксировать serializer и revision helper.

### Этап 2. Parse + preflight

1. Добавить CLI-arg parsing для `update`.
2. Добавить normalization patch operations.
3. Добавить path/value preflight validation.

### Этап 3. In-memory patch engine

1. Реализовать patch для `meta`.
2. Реализовать patch для `refs`.
3. Реализовать patch для section-level content.
4. Реализовать whole-body replace.
5. Добавить semantic diff и `changes[]`.

### Этап 4. Validation + persistence

1. Включить full post-patch validation.
2. Включить automatic `updated_date`.
3. Включить path recompute.
4. Включить atomic write / move transaction.
5. Включить `dry-run`.

### Этап 5. Контрактные тесты

1. Добавить unit-тесты parser/patch/serializer.
2. Добавить integration-тесты CLI.
3. Добавить regression-тесты на `revision`, `no-op`, `CONCURRENCY_CONFLICT`, path move.

## 8. Зафиксированные уточнения

Ниже собраны решения по ранее открытым местам спецификации. Они уже считаются частью этого плана и не требуют дополнительного выбора перед coding.

### 8.1. `changes[]` для whole-body операций

Для whole-body replace нужно добавлять synthetic entry с `field: "content.raw"` и полями `before_hash` / `after_hash` по raw body.
Формат этих hash-полей должен совпадать с форматом токена `revision`.
`content.raw` в этом контексте является только diff-representation ответа и не считается write-path.

### 8.2. `--content-*` для типов без `content`

Для типов без блока `content` опции `--content-file`, `--content-stdin`, `--clear-content` должны завершаться `WRITE_CONTRACT_VIOLATION`.

### 8.3. Успешный `dry-run`

`update --dry-run` должен проходить тот же pipeline, что и обычный `update`, возвращать тот же success-payload и отличаться только отсутствием записи на диск и значением `dry_run: true`.

### 8.4. Поведение `unset` для отсутствующего metadata/ref path

`unset` должен быть идемпотентным для всех допустимых path.
Если `meta.<field>` или `refs.<field>` отсутствует, операция считается успешным `no-op` без записи в `changes[]`.

### 8.5. Вычисление текущего `revision` для `--expect-revision`

Текущий `revision` перед проверкой `--expect-revision` нужно считать по текущему persisted-документу как источнику истины, без канонической пересборки parsed model.

### 8.6. Формы значений для `meta.<field>`

`meta.<field>` должен принимать любой YAML value-node, допустимый по `schema.type` целевого поля.
Для `schema.type: array` допустим YAML array.
`type: object` стандартом не поддерживается и в `update` приниматься не должен.

### 8.7. Scope `--set-file`

`--set-file` в `update` должен быть разрешен только для `content.sections.<name>`.
Использование `--set-file` для `meta.<field>` и `refs.<field>` должно завершаться `WRITE_CONTRACT_VIOLATION`.

### 8.8. Порядок frontmatter

Serializer `update` должен использовать обязательный порядок frontmatter-полей:
`type`, `id`, `slug`, `created_date`, `updated_date`, затем `meta.fields` в порядке объявления в raw schema.

### 8.9. Spacing/newline policy serializer

Serializer `update` должен использовать minimal-normalizing policy:
platform-native newline policy, ровно одну пустую строку между frontmatter и непустым body, отсутствие лишнего пустого блока при пустом body, trailing newline и сохранение нетронутого body без лишней переразметки.

### 8.10. Success-payload для `noop`

Для успешного `noop` ответ должен возвращать `updated: false`, `noop: true`, `changes[]: []`.

### 8.11. Источник write-контракта `update`

`update` должен выводить write-контракт напрямую из raw schema типа сущности и не должен зависеть от `schema.model` как runtime-источника write-семантики.
Допустимые path для `--unset` совпадают с тем же writable namespace, что и для `--set`.

### 8.12. Источник истины для реализации

При реализации `update` в этом репозитории источником истины является настоящий план.
При расхождении с другими документами по `update` приоритет имеет этот план.

### 8.13. Shape `validation` в success-ответе

Успешный ответ `update` должен возвращать:

```json
"validation": {
  "ok": true,
  "issues": []
}
```

Детали ошибок валидации передаются только в error-envelope (`error.details.validation.issues[]`) при `VALIDATION_FAILED`.

### 8.14. Состав success-payload

Success-payload `update` не должен включать поля `target` и `file`.
Обязательный состав success-payload ограничивается полями из раздела 4, шаг 13.

### 8.15. Ошибка optimistic concurrency

Для несовпадения `--expect-revision` вводится отдельный доменный код `CONCURRENCY_CONFLICT`.
Для него используется `exit_code: 1` и `result_state: "invalid"`.

### 8.16. Совместимость whole-body с patch-операциями

Whole-body операции (`--content-file`, `--content-stdin`, `--clear-content`) можно комбинировать с patch-операциями `meta.*` и `refs.*`.
Запрещено только смешение whole-body операций с patch-операциями `content.sections.*`.

### 8.17. Правило hash для `changes[]`

Для `before_hash` / `after_hash` в `changes[]` используется формат токена `sha256:<hex>`.
Hash вычисляется по raw string value соответствующего patch-slot "как есть" (без platform newline-normalization).

### 8.18. Обработка `--expect-revision` по типам ошибок

Если token в `--expect-revision` синтаксически некорректен, команда завершается `INVALID_ARGS`.
Если token синтаксически корректен, но не совпадает с текущим revision, команда завершается `CONCURRENCY_CONFLICT`.

### 8.19. Резолв путей для file-based опций

Пути в `--set-file` и `--content-file` резолвятся относительно `cwd` процесса (как в `add`), а не относительно `--workspace`.

### 8.20. Output format в рамках этой задачи

В рамках текущей задачи `update` реализуется только для `--format json`.
Поддержка `ndjson` и соответствующих контрактных тестов в scope этой задачи не входит.

### 8.21. Shape `entity.refs` в success-ответе

В success-payload `entity.refs` возвращается только краткая форма:
`{ "<field>": { "id": "<target_id>" } }`.
Расширенные ref-данные (`type`, `slug`, `dir_path`, `meta`) в публичный ответ `update` не включаются.

## 9. Итог

Корректная реализация `update` в baseline сводится к одной связанной цепочке:

1. Нормализовать CLI patch.
2. Найти сущность по `id`.
3. Получить type-specific write-контракт.
4. Проверить `--expect-revision`.
5. Выполнить preflight write validation.
6. Применить patch к in-memory document model.
7. Определить `noop` или реальное изменение.
8. Обновить `updated_date` при необходимости.
9. Полностью провалидировать post-patch кандидата.
10. Пересчитать path/revision.
11. Выполнить atomic write или atomic move+write.
12. Вернуть machine-first success payload с `changes[]`.

Если делать реализацию именно в таком порядке, то требования baseline и стандарта укладываются в один детерминированный pipeline без неявных побочных эффектов.
