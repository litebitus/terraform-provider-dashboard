# Dashboard DSL schema changelog

Each version is the format a dashboard document names in its `schema_version`, and the schema file of that name. A version not yet released is headed `## <version> — unreleased`; the release workflow stamps it with the provider release that ships it and the date.

## 1.1 — unreleased

- A value mapping is its `text` and an optional `color`, which puts the value in a legend.

### Deprecated

- `value_mappings`: a value mapped to a string, such as `"0": none`; write it as `"0": { text: none }`.

## 1.0 — provider v0.1.0, 2026-11-06

- The first format: a dashboard's `rows`, their `panels` and the panels' `queries`, with `variables`, `target_schemas` and `id`.
