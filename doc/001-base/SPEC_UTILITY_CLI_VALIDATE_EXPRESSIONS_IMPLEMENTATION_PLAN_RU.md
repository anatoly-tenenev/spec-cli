# План реализации `expressions` в `spec-cli validate`

## 1. Цель

Документ задает реалистичный план перехода от MVP (`SPEC_UTILITY_CLI_VALIDATE_BASE_IMPLEMENTATION_RU.md`) к полной поддержке выражений стандарта в команде `validate`.

Целевой результат:

- `required_when` вычисляется по стандарту для `meta.fields[]` и `content.sections[]`;
- `path_pattern.cases[].when` вычисляется по той же модели выражений;
- соблюдается семантика `missing`, строгих/безопасных операторов и контекста `meta/ref`;
- диагностики попадают в правильные классы (`SchemaError` / `InstanceError` / `ProfileError`);
- `summary.validator_conformant=true` только при детерминированном профиле и полном покрытии требований.

## 2. Нормативные опоры

Реализация должна строго опираться на:

- `SPEC_STANDARD_RU_REVISED_V3.md`:
  - раздел `8` (`path_pattern`, `cases[].when`);
  - раздел `9` (плейсхолдеры и `ref:*`);
  - раздел `11.5` (модель обязательности);
  - раздел `11.6` (операторы выражений, `missing`, типизация);
  - раздел `12.3` (резолв `entity_ref` и контекст `ref`);
  - раздел `14.3`/`14.4` (`Validator-conformant`, классы диагностик).
- `SPEC_UTILITY_CLI_API_RU.md`:
  - раздел `5.1 validate` (`summary.validator_conformant`, CLI-контракт и выходные форматы).

## 3. Область реализации

### 3.1. Что включаем

- Выражения-объекты с одним оператором:
  - `eq`, `eq?`, `in`, `in?`, `all`, `any`, `not`, `exists`.
- Булевый `required_when` (literal `true/false`) и объектная форма.
- Вычисление `path_pattern.cases[].when` слева направо.
- Полный контекст выражений:
  - `meta.<field_name>` (включая встроенные поля);
  - `ref.<field_name>.<part>` (`id|type|slug|dir_path`).
- Семантика `meta.<entity_ref_field>` как алиас `ref.<field_name>.id` (по результату резолва).
- Поведение `eq`/`in` при `missing` как `InstanceError`.

### 3.2. Что не включаем этим планом

- Новые операторы/DSL сверх стандарта.
- Ослабление строгой типизации.
- Изменение формата `json/ndjson`-контракта `validate`.

## 4. Архитектура

### 4.1. Новый слой: `Expression Engine`

Ввести отдельный слой с двумя фазами:

1. `compile` (один раз на схему):
   - парсинг и нормализация выражений в AST;
   - структурная валидация (оператор, кардинальность, типы литералов, непустые списки);
   - статическая проверка ссылок `meta.*`/`ref.*` против схемы;
   - привязка `standard_ref` для будущих диагностик.
2. `evaluate` (для каждой сущности):
   - вычисление AST в контексте конкретной реализации;
   - возврат булевого результата или доменной ошибки (диагностики).

Рекомендованные модули:

- `validation/expressions/ast.*`
- `validation/expressions/compiler.*`
- `validation/expressions/evaluator.*`
- `validation/expressions/context.*`

### 4.2. Контекст вычисления

Контекст на сущность должен строиться после:

1. разбора frontmatter;
2. проверки встроенных полей (`type/id/slug/date`);
3. резолва `entity_ref` (минимум: индекс целей + `refTypes` проверки + `dir_path`).

Структура контекста:

- `meta`: map literal-полей frontmatter;
- `refs`: map резолвленных ссылок с атрибутами `id/type/slug/dir_path`;
- `presence`:
  - для `meta.<non_entity_ref>`: ключ существует в YAML (`null` считается присутствием);
  - для `meta.<entity_ref>` и `ref.*`: присутствие только при успешном резолве ссылки.

### 4.3. Семантика вычисления (ядро)

Ввести внутренний тип результата операнда:

- `Scalar(value)` (`string|number|boolean|null`);
- `Missing`.

Базовые правила:

- `eq` / `in`:
  - если любой операнд `Missing` -> `InstanceError` (раздел `11.6`), выражение считается невычислимым для данной сущности;
- `eq?` / `in?`:
  - если любой операнд `Missing` -> результат `false`, без отдельной ошибки;
- `exists`:
  - использует правила присутствия из `11.6`;
- `all` / `any`:
  - short-circuit слева направо;
- `not`:
  - отрицание булевого результата дочернего выражения.

Строгая типизация сравнения:

- допустимы только скаляры;
- `integer` и `number` сравниваются по числовому значению;
- `null` равно только `null`;
- любые `array/object` в литералах выражений -> `SchemaError` на фазе компиляции.

## 5. Интеграция в текущий `validate` pipeline

### 5.1. Изменения по шагам пайплайна

1. `Load schema`:
   - добавить `compileExpressions(schema)` и сбор schema-диагностик.
2. `Parse documents`:
   - без изменений контракта, но сохранить данные для выражений в `EntityRuntimeContext`.
3. `Validate schema rules vs entity`:
   - `meta.fields.required_when` через expression evaluator;
   - `content.sections.required_when` через expression evaluator;
   - string `const/enum` с плейсхолдерами продолжают использовать общий контекст (`meta/ref`) и единый resolver значений.
4. `Path checks`:
   - выбор `path_pattern`-кейса через `cases[].when`;
   - при `InstanceError` в strict-операторе не переходить к следующему кейсу автоматически;
   - фиксировать проблему как нарушение сущности.
5. `Finalize summary`:
   - `validator_conformant=false`, если профиль резолва `entity_ref/ref` не загружен или недетерминирован.

### 5.2. Порядок вычислений внутри сущности

1. builtins + базовая типизация frontmatter;
2. `entity_ref` типы/резолв и построение `ref`-контекста;
3. `required_when` для `meta.fields`;
4. schema-check самих `meta.fields` (`type/const/enum/...`);
5. `required_when` для `content.sections`;
6. `path_pattern.cases[].when` и проверка пути.

Такой порядок исключает ложные `missing` для `ref.*` и устраняет циклические зависимости.

## 6. Диагностики

### 6.1. Классификация (обязательно)

- `SchemaError`:
  - неверная структура выражения;
  - неизвестный оператор;
  - неверная кардинальность аргументов;
  - недопустимые ссылки `meta.*`/`ref.*`;
  - нарушение ограничений типов литералов.
- `InstanceError`:
  - `eq`/`in` получили `missing`;
  - `exists`/ссылки указывают на нерезолвленный `entity_ref` в данных сущности;
  - ошибка выбора `path_pattern` из-за невычислимого strict-условия.
- `ProfileError`:
  - нет детерминированного профиля резолва `entity_ref/ref`.

### 6.2. Рекомендуемый минимальный набор code-id

- `schema.expression.invalid_operator`
- `schema.expression.invalid_arity`
- `schema.expression.invalid_operand_type`
- `schema.expression.invalid_reference`
- `instance.expression.missing_operand_strict`
- `instance.path_pattern.when_evaluation_failed`
- `profile.expression_context_unavailable`

Коды можно адаптировать к текущему naming, но семантика и класс должны сохраниться.

## 7. План внедрения по этапам

### Этап 1. Компилятор выражений (schema-time)

- AST + парсер операторов.
- Валидация структуры выражений и ссылок на схему.
- Подключение schema-диагностик в общий `issues[]`.

Критерий: некорректные выражения ловятся до прохода по документам.

### Этап 2. Контекст и резолв зависимостей

- Единый `EntityRuntimeContext`.
- Резолв `entity_ref` с детерминированным `dir_path`.
- Явная проверка/флаг профиля реализации.

Критерий: доступны `meta`/`ref` источники для evaluator.

### Этап 3. Evaluator + `required_when`

- Реализация семантики `eq/eq?/in/in?/all/any/not/exists`.
- Интеграция в обязательность `meta.fields` и `content.sections`.

Критерий: conditional required работает по стандарту, включая `missing`.

### Этап 4. `path_pattern.cases[].when`

- Канонизация `path_pattern` и вычисление кейсов.
- Корректная обработка strict-ошибок без silent fallback.

Критерий: выбранный шаблон всегда детерминирован и объясним диагностикой.

### Этап 5. Вывод, совместимость, hardening

- Финальная калибровка code-id и `standard_ref`.
- Проверка `json/ndjson` контракта и exit code.
- Оптимизация (кеш AST, O(1) lookup ссылок, short-circuit).

Критерий: регрессионный набор проходит, формат вывода не ломается.

## 8. Тест-план (минимум)

### 8.1. Unit: compiler/evaluator

- каждый оператор в happy-path;
- неверная кардинальность и типы литералов;
- `eq`/`in` + `missing` -> `InstanceError`;
- `eq?`/`in?` + `missing` -> `false`.

### 8.2. Integration: validate

- `meta.fields.required_when` на встроенных и пользовательских полях;
- `content.sections.required_when`;
- `path_pattern` с несколькими `cases` и смешанными strict/safe условиями;
- сценарии с `entity_ref` + `ref.<field>.dir_path`.

### 8.3. Contract

- `--format json` и `--format ndjson` (issue + summary);
- `summary.validator_conformant` при наличии/отсутствии профиля;
- `--fail-fast` и `--warnings-as-errors`.

## 9. Риски и меры

- Риск: недетерминированный резолв `entity_ref` ломает `ref`-контекст.
  - Мера: явная проверка профиля до validate-run, `ProfileError` + `validator_conformant=false`.
- Риск: каскадные ошибки при strict-операторах.
  - Мера: единый policy "одна первичная ошибка вычисления на выражение + ограничение вторичных сообщений".
- Риск: деградация производительности на больших workspace.
  - Мера: компиляция выражений один раз, кеш схемных lookup-ов, short-circuit.

## 10. Критерии готовности

- Все обязательные операторы выражений реализованы и покрыты тестами.
- `required_when` и `path_pattern.cases[].when` работают детерминированно.
- Классы диагностик соответствуют разделу `14.4`.
- `summary.validator_conformant` корректно отражает полноту профиля.
- Вывод `validate` остается совместимым с контрактом API (`json/ndjson`, exit codes).
