# Dashboard DSL schema changelog

Each version is the format a dashboard document names in its `schema_version`, and the schema file of that name. A version not yet released is headed `## <version> — unreleased`; the release workflow stamps it with the provider release that ships it and the date.

## 1.4 — unreleased

- A `bar` panel takes a `value_order`, its bars in `ascending` or `descending` order of their values.

## 1.3 — provider v0.1.3, 2026-11-12

- A `table`'s query takes a `value_column_header`, the header of its value column, and a `value_format`, a layout of the YAML DSL standard's format function its value is written by.

## 1.2 — provider v0.1.2, 2026-11-11

- A `metrics` source takes a `reporting_filter`, the series it emits while it reports.

## 1.1 — provider v0.1.1, 2026-11-08

- A value mapping is its `text` and an optional `color`, which puts the value in a legend.
- The first threshold may leave out `value`: its color then holds from the lowest.
- A panel with `thresholds` holds no value mapping with a `color`.

### Deprecated

- `value_mappings`: a value mapped to a string, such as `"0": none`; write it as `"0": { text: none }`.

## 1.0 — provider v0.1.0, 2026-11-06

- The first format: a dashboard's `rows`, their `panels` and the panels' `queries`, with `variables`, `target_schemas` and `id`.
