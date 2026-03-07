# Backlog интеграционных кейсов `validate` для покрытия стандарта

Дата фиксации: `2026-03-07`.

Документ задает практический чеклист недостающих интеграционных кейсов:
`group/case-id -> что положить в schema/workspace -> что фиксировать в response.json`.

Источник требований: `spec/SPEC_STANDARD_RU_REVISED_V3.md`.

## 1. Базовый шаблон ожиданий

### 1.1 SchemaError-кейсы (`20_schema`)

- `exit_code`: `4`.
- `response.json`:
  - `result_state = "invalid"`;
  - `error.code = "SCHEMA_INVALID"`;
  - `error.exit_code = 4`;
  - `error.message` не пустой.

### 1.2 InstanceError-кейсы (`30+`)

- `exit_code`: `1`.
- `response.json`:
  - `result_state = "invalid"`;
  - есть `summary` с `errors > 0`;
  - есть минимум один `issue` с:
    - `class = "InstanceError"`;
    - `standard_ref` нужного раздела;
    - `code` и `message` не пустые.

### 1.3 Invalid args (`10_contract`)

- `exit_code`: `2`.
- Для `ndjson` ожидать массив из одной записи:
  - `record_type = "error"`;
  - `result_state = "invalid"`;
  - `error.code = "INVALID_ARGS"`;
  - `error.exit_code = 2`.

## 2. P0 backlog (в первую очередь)

### 2.1 `20_schema/0002_validate_schema_closed_world_top_level_extra_key_json`

- Цель: закрытая модель ключей верхнего уровня схемы.
- `spec.schema.yaml`: добавить запрещенный верхнеуровневый ключ (`extra: true` или `x-extra: true`) вместе с валидными `version/entity`.
- `workspace.in`: один валидный `.md` документ.
- `response.json`: шаблон SchemaError.
- `standard_ref`: `4`, `14.4`.

### 2.2 `20_schema/0003_validate_schema_closed_world_entity_unknown_key_json`

- Цель: запрет лишних ключей в `entity.<type_name>`.
- `spec.schema.yaml`: в `entity.doc` добавить `unknown_key: value`.
- `workspace.in`: один валидный `.md`.
- `response.json`: шаблон SchemaError.
- `standard_ref`: `5.2`, `14.4`.

### 2.3 `20_schema/0004_validate_schema_id_prefix_duplicate_across_types_json`

- Цель: глобальная уникальность `id_prefix` между типами.
- `spec.schema.yaml`: два типа с одинаковым `id_prefix`.
- `workspace.in`: пусто или один валидный файл.
- `response.json`: шаблон SchemaError.
- `standard_ref`: `7.3`, `14.4`.

### 2.4 `20_schema/0005_validate_schema_path_pattern_unconditional_not_last_json`

- Цель: безусловный `cases`-кейс должен быть последним.
- `spec.schema.yaml`: `path_pattern.cases` с кейсом без `when` не в конце.
- `workspace.in`: один валидный `.md`.
- `response.json`: шаблон SchemaError.
- `standard_ref`: `8.3`, `14.4`.

### 2.5 `20_schema/0006_validate_schema_path_pattern_missing_unconditional_case_json`

- Цель: в `cases` должен быть ровно один безусловный кейс.
- `spec.schema.yaml`: только условные кейсы (ни одного без `when`) или два безусловных.
- `workspace.in`: один валидный `.md`.
- `response.json`: шаблон SchemaError.
- `standard_ref`: `8.3`, `14.4`.

### 2.6 `20_schema/0007_validate_schema_unknown_placeholder_in_pattern_json`

- Цель: запрет неподдерживаемых `{...}`.
- `spec.schema.yaml`: `path_pattern: "docs/{unknown}.md"` или `const: "{bad:token}"`.
- `workspace.in`: один валидный `.md`.
- `response.json`: шаблон SchemaError.
- `standard_ref`: `9.1`, `12.2`, `14.4`.

### 2.7 `20_schema/0008_validate_schema_required_true_with_required_when_json`

- Цель: запрет `required: true` вместе с `required_when`.
- `spec.schema.yaml`: поле `meta.fields[]` или `content.sections[]` с этой комбинацией.
- `workspace.in`: один валидный `.md`.
- `response.json`: шаблон SchemaError.
- `standard_ref`: `11.5`, `14.4`.

### 2.8 `20_schema/0009_validate_schema_required_when_invalid_operator_json`

- Цель: валидатор выражений `required_when`.
- `spec.schema.yaml`: `required_when: { xor: [...] }` или неверная кардинальность `eq`.
- `workspace.in`: один валидный `.md`.
- `response.json`: шаблон SchemaError.
- `standard_ref`: `11.6`, `14.4`.

### 2.9 `20_schema/0010_validate_schema_type_object_unsupported_json`

- Цель: запрет `schema.type: object`.
- `spec.schema.yaml`: `meta.fields[].schema.type: object`.
- `workspace.in`: один валидный `.md`.
- `response.json`: шаблон SchemaError.
- `standard_ref`: `12.2`, `14.4`.

### 2.10 `20_schema/0011_validate_schema_ref_types_unknown_entity_json`

- Цель: `refTypes` должен ссылаться на существующий тип.
- `spec.schema.yaml`: `schema.type: entity_ref`, `refTypes: [ghost_type]`.
- `workspace.in`: один валидный `.md`.
- `response.json`: шаблон SchemaError.
- `standard_ref`: `12.2`, `14.4`.

### 2.11 `30_instance_builtin/0002_validate_frontmatter_missing_json`

- Цель: frontmatter обязателен для Markdown реализации.
- `workspace.in`: файл `.md` без блока `---`.
- `spec.schema.yaml`: простой валидный тип.
- `response.json`: шаблон InstanceError.
- `standard_ref`: `11`, `14.4`.

### 2.12 `30_instance_builtin/0003_validate_type_unknown_json`

- Цель: `type` должен совпадать с ключом `entity`.
- `workspace.in`: `type: unknown`.
- `spec.schema.yaml`: только `entity.doc`.
- `response.json`: шаблон InstanceError (`standard_ref = 5.3`).
- `standard_ref`: `5.3`, `11`, `14.4`.

### 2.13 `30_instance_builtin/0004_validate_id_prefix_mismatch_with_type_json`

- Цель: согласованность `type` и `id_prefix`.
- `workspace.in`: `type: doc`, `id: SRV-1`.
- `spec.schema.yaml`: `doc.id_prefix = DOC`, `service.id_prefix = SRV`.
- `response.json`: шаблон InstanceError.
- `standard_ref`: `5.3`, `11.1`, `14.4`.

### 2.14 `30_instance_builtin/0005_validate_slug_regex_invalid_json`

- Цель: regex для `slug`.
- `workspace.in`: `slug: Bad_Slug`.
- `spec.schema.yaml`: валидный тип `doc`.
- `response.json`: шаблон InstanceError (`standard_ref = 11.2`).
- `standard_ref`: `11.2`, `14.4`.

### 2.15 `30_instance_builtin/0006_validate_created_date_calendar_invalid_json`

- Цель: календарная корректность даты.
- `workspace.in`: `created_date: 2026-02-30`.
- `spec.schema.yaml`: валидный тип `doc`.
- `response.json`: шаблон InstanceError (`standard_ref = 11.3`).
- `standard_ref`: `11.3`, `14.4`.

### 2.16 `40_instance_meta_content/0003_validate_entity_ref_null_not_missing_json`

- Цель: `null` для `entity_ref` не равен отсутствию ключа.
- `spec.schema.yaml`: опциональное поле `owner` с `schema.type: entity_ref`.
- `workspace.in`: `owner: null`.
- `response.json`: шаблон InstanceError (`type mismatch`).
- `standard_ref`: `11.4`, `12.3`, `14.4`.

### 2.17 `40_instance_meta_content/0004_validate_content_title_mismatch_case_sensitive_json`

- Цель: проверка `content.sections[].title` с учетом регистра.
- `spec.schema.yaml`: `sections: [{ name: goal, title: "Goal" }]`.
- `workspace.in`: секция `## [goal](#goal)` (строчные буквы в title).
- `response.json`: шаблон InstanceError.
- `standard_ref`: `13.1`, `13.2`, `14.4`.

### 2.18 `40_instance_meta_content/0005_validate_content_auto_anchor_not_allowed_json`

- Цель: запрет авто-ярлыков без явной маркировки.
- `spec.schema.yaml`: обязательная секция `name: goal`.
- `workspace.in`: заголовок `## Goal` без `(#goal)` и без `{#goal}`.
- `response.json`: шаблон InstanceError (`content.required_missing`).
- `standard_ref`: `13.2`, `14.4`.

### 2.19 `50_path_pattern_expr/0004_validate_path_pattern_placeholder_reuse_mismatch_json`

- Цель: повторный плейсхолдер должен давать одно и то же значение.
- `spec.schema.yaml`: `path_pattern: "docs/{slug}/{slug}.md"`.
- `workspace.in`: файл `docs/a/b.md`, `slug: a`.
- `response.json`: шаблон InstanceError (несовпадение пути).
- `standard_ref`: `9.3`, `8.1`, `14.4`.

### 2.20 `60_entity_ref_context/0003_validate_entity_ref_ref_types_mismatch_json`

- Цель: `refTypes` ограничивает тип цели ссылки.
- `spec.schema.yaml`: поле `owner` как `entity_ref`, `refTypes: [service]`.
- `workspace.in`: `owner` указывает на существующий `domain` (не `service`).
- `response.json`: шаблон InstanceError.
- `standard_ref`: `6.2`, `12.3`, `14.4`.

## 3. P1 backlog (следующий приоритет)

### 3.1 `30_instance_builtin/0007_validate_frontmatter_not_first_line_json`

- `workspace.in`: перед `---` добавить пустую/текстовую строку.
- `response.json`: шаблон InstanceError.
- `standard_ref`: `11`, `14.4`.

### 3.2 `30_instance_builtin/0008_validate_frontmatter_duplicate_keys_json`

- `workspace.in`: повторить ключ (`slug`) дважды во frontmatter.
- `response.json`: шаблон InstanceError.
- `standard_ref`: `11`, `14.4`.

### 3.3 `30_instance_builtin/0009_validate_frontmatter_extra_key_forbidden_json`

- `workspace.in`: добавить ключ, не входящий во встроенные и `meta.fields`.
- `response.json`: шаблон InstanceError.
- `standard_ref`: `11`, `12.3`, `14.4`.

### 3.4 `40_instance_meta_content/0006_validate_required_when_all_any_not_json`

- `spec.schema.yaml`: задать `required_when` с `all`, `any`, `not`.
- `workspace.in`: две сущности для true/false веток.
- `response.json`: проверить, что обязательность срабатывает только в ожидаемой ветке.
- `standard_ref`: `11.6`, `12.3`, `13.2`.

### 3.5 `50_path_pattern_expr/0005_validate_path_pattern_strict_in_missing_error_json`

- `spec.schema.yaml`: `path_pattern.cases[].when` использует строгий `in` по опциональному полю.
- `workspace.in`: операнд отсутствует.
- `response.json`: шаблон InstanceError (`when_evaluation_failed`).
- `standard_ref`: `11.6`, `8.4`, `14.4`.

### 3.6 `50_path_pattern_expr/0006_validate_path_pattern_object_extra_key_schema_error_json`

- `spec.schema.yaml`: в объекте `path_pattern` кроме `cases` добавить `mode: strict`.
- `response.json`: шаблон SchemaError.
- `standard_ref`: `8.3`, `14.4`.

### 3.7 `50_path_pattern_expr/0007_validate_path_pattern_case_extra_key_schema_error_json`

- `spec.schema.yaml`: в одном `cases[]` добавить лишний ключ (`note`).
- `response.json`: шаблон SchemaError.
- `standard_ref`: `8.3`, `14.4`.

### 3.8 `60_entity_ref_context/0004_validate_entity_ref_global_resolve_without_ref_types_valid_json`

- `spec.schema.yaml`: `entity_ref` без `refTypes`.
- `workspace.in`: ссылка указывает на глобально уникальный `id` существующей сущности.
- `response.json`: `result_state = "valid"`, `errors = 0`.
- `standard_ref`: `12.3`, `11.1`.

### 3.9 `70_global_uniqueness/0003_validate_slug_same_value_across_types_is_allowed_json`

- `spec.schema.yaml`: типы `doc` и `service`.
- `workspace.in`: одинаковый `slug` у `doc` и `service`, остальные поля валидны.
- `response.json`: `result_state = "valid"`.
- `standard_ref`: `11.2`, `14.2`.

### 3.10 `80_profile_conformance/0001_validate_ndjson_issue_order_stable_json`

- Цель: зафиксировать детерминированный порядок NDJSON-записей при стабильном входе.
- `spec.schema.yaml` и `workspace.in`: несколько однотипных нарушений в разных файлах.
- `response.json`: каноничный NDJSON-массив в стабильном порядке (`issue...`, затем `summary`).
- `standard_ref`: `6.5`, `14.3`.

## 4. Рекомендуемый порядок реализации

1. Закрыть все `20_schema` P0-кейсы.
2. Закрыть `30_instance_builtin` P0-кейсы.
3. Закрыть `40/50/60` P0-кейсы.
4. Добавить `80_profile_conformance/0001...` и зафиксировать минимальные conformance-гарантии.
5. Довести P1-кейсы.

