# Dashboard provider

A Terraform provider that renders dashboard DSL documents at plan time. A document says what a dashboard shows in words no UI owns; the provider answers it in the dashboard JSON of one target schema, or refuses it, naming why. It writes nothing: the caller puts the JSON where the UI loads it. Functions need Terraform 1.8 or later.

## Rendering

| | Signature | Answers |
|---|---|---|
| function `provider::dashboard::render` | `(spec string, options map(string)...)`, at most one map of options, `target_schema` its one key | the dashboard JSON, a string |
| data source `dashboard_render` | `spec`, `target_schema` optional | `json`, the dashboard JSON |

- `spec` is the document's text, which states everything about the dashboard, its identity included, in its `id`.
- Both render through one path and differ in one thing: the data source also reads the provider's `default_target_schemas`, which Terraform never shows a function.
- A render is a function of the document and its target schema alone, the same on every run: an identity the UI assigns never enters it, so a plan shows no change until the document does.
- A problem fails the plan on the input it lies with: the document's at its position, `dashboard:"<title>":<line>:<column>`, and a target schema that cannot be chosen on `target_schema`, or the options for the function.

### Choosing the target schema

The target schemas and their identifiers are the DSL's, in [dashboard-dsl.md](dashboard-dsl.md). A render chooses one, highest priority first:

1. The call's `target_schema`: a whole identifier, or a filter of the document's `target_schemas` by whole leading words.
2. The document's `target_schemas`: the one left is used.
3. The provider's `default_target_schemas`, data source only: consulted when several remain, or when the document declares none and the call names no whole identifier.

```hcl
provider "dashboard" {
  default_target_schemas = {
    grafana = "grafana-v2-13"
  }
}
```

- A default's key is a filter by whole leading words and its value the whole identifier it picks among the candidates the key matches; the most specific key that matches a candidate applies.
- A choice that leaves none or several is refused, naming what remains.
- The chosen identifier resolves among the target schemas the provider carries: a minor names itself, a major the newest carried minor of it. One the provider does not carry is refused, listing those it does.

| Document declares | `target_schema` | Function | Data source, default `grafana-v2-14` |
|---|---|---|---|
| `[grafana-v2-13]` | — | 13 | 13 |
| `[grafana-v2-13, grafana-v2-14]` | — | refused, two remain | 14 |
| `[grafana-v2-13, grafana-v2-14]` | `grafana-v2-13` | 13 | 13 |
| `[grafana-v2-13, grafana-v2-14]` | `grafana` | refused, two remain | 14 |
| `[grafana-v2-14]` | `grafana-v2-13` | refused, not declared | refused, not declared |
| — | `grafana-v2-13` | 13 | 13 |
| — | `grafana` | refused, not a whole identifier | 14 |
| — | — | refused | 14, the only default |

## Reading a document

A document is read in two steps, and every problem either finds is reported:

1. `dashboarddsl` holds its shape to the schema its `schema_version` names, which the `schemas` package resolves among the schema files it embeds.
2. `dashboarddsl` holds it to the DSL's rules a schema cannot state, its queries and the filter parsed as DQL and a variable's `choices` read by the DQL compiler's call parser.

The text is read, and its shape validated, by the [YAML DSL standard](https://github.com/litebitus/yaml-dsl-spec)'s packages, and every problem is written by its diagnostic renderer, as a compiler writes an error, with no color and no division to width:

- The source is named `dashboard:` and the document's `title` as a JSON string, such as `dashboard:"Mock Service"`; a document with no string `title` is named `dashboard:{title}`.
- A problem with a value is placed at the value, a field that is not allowed at its key, and a problem with a whole block, such as a row's widths, at the block.
- Problems are apart by a blank line, in document order, and a last line counts them: `1 error`, `2 errors`.

## Target schemas

A renderer carries its target schemas as entries of a manifest beside their JSON Schemas, `grafanarenderer/target-schemas/`. An entry states what its schema is generated from and what a render for it writes:

| Field | Holds |
|---|---|
| `grafana_release` | the Grafana release tag the schema is generated from |
| `cue_file`, `cue_expression` | Grafana's CUE file at that tag, and the definition in it the schema is |
| `api_version` | the resource's `apiVersion` |
| `plugin_version` | every panel's visualization version |

- `make target-schemas` fetches each entry's CUE file from Grafana's repository at its release and converts the definition to JSON Schema with a pinned `cue`, writing `<identifier>.schema.json`.
- A render is checked against its target's schema before it is answered; a render that does not conform fails as a defect of the provider, naming where.
- A new Grafana version is a new entry and its generated schema; the renderer changes only where the new version's format does.

## Grafana

A render is a V2 Resource dashboard of the target schema's `api_version`. Its `metadata.name` is the document's `id`; without one the render names none, and Grafana assigns a name and keeps it with the dashboard's file.

### Queries

- Every query compiles to PromQL. A panel's, an annotation's and a variable's choices query each also apply the dashboard's filter.
- A query names no data source and reads the UI's default.
- A variable's choices query is a filter with no stages: Grafana reads label values from a series selector.

### Layout

- The rows are Grafana's rows layout, each row's panels on a grid of their own, left to right. A collapsed row opens collapsed; a `repeat_for` is the row's or the panel's repeat by variable.
- The grid is 24 columns. A stated width is its share of the row, rounded to columns; the panels without one share what is left equally, a column left over going to the first of them.
- A height is rounded to the nearest number of grid rows, each 30 pixels with 8 between.
- Each panel is an element, `panel-<n>`, numbered in document order, which its row's grid places.

### Panels

| `kind` | Grafana visualization | Draws, beside `unit` | Queries answer |
|---|---|---|---|
| `area`, `time_series` | `timeseries`, an area filled at 25% opacity | `axis`, `decimals`, `legend`, `min`, `max`, `stacked`, `thresholds`, `value_mappings` | ranges |
| `bar` | `bargauge` | `calculation`, `decimals`, `min`, `max`, `thresholds`, `value_mappings` | ranges |
| `gauge`, `stat` | `gauge`, `stat` | `calculation`, `decimals`, `min`, `max`, `thresholds`, `value_mappings` | ranges |
| `pie` | `piechart` | `calculation`, `decimals`, `legend`, `value_mappings` | ranges |
| `heatmap` | `heatmap` | `decimals`, `legend` | heatmap buckets |
| `histogram` | `histogram` | `axis`, `decimals`, `legend` | ranges |
| `map` | `geomap`, each item placed by looking its `location_field` up in Grafana's country gazetteer | `decimals`, `thresholds` | a table, instant |
| `records`, `table` | `table` | `decimals`, `thresholds`, `value_mappings` | a table, instant |
| `state_timeline`, `status_history` | `state-timeline`, `status-history` | `decimals`, `legend`, `thresholds`, `value_mappings` | ranges |
| `text` | `text`, Markdown | `content` | — |

- A panel that states no legend shows one, as a list at the bottom.
- A `table` hides Grafana's evaluation time with an `organize` transformation, which also heads each query's value column with its `value_column_header`. A query's value column is Grafana's `Value`, or `Value #` and its refId where the panel has several queries.
- A `table` with several queries merges them first with a `merge` transformation, which pairs rows on the fields every query's frame holds.
- A `bar` panel with a `value_order` reduces each series to a row by its `calculation` with a `reduce` transformation, its name in the row's text column, and sorts the rows by the calculation's column with a `sortBy` transformation. The bar gauge then draws each row as a bar, named by its text column, in the rows' order.
- A `value_format` is the unit of its query's value column, Grafana's custom date unit `time:`: its text in brackets, and each of `%C`'s date verbs as the Moment.js token that writes it alike, unpadded or zero-padded. `%Y` is `YYYY`, which pads a year below 1000 to four digits.
- A `value_format`'s query is scaled from the digits of a second `%C`'s precision states to milliseconds, the unit Grafana's date unit reads. Grafana writes the time in the dashboard's `timezone`.
- A value below the lowest threshold is drawn in the theme's text color; `grey` is `#808080`, every other color Grafana's own of that name.

| `unit` | Grafana unit |
|---|---|
| `bytes` | `bytes` |
| `count` | `short` |
| `percent` | `percent` |
| `per_second` | `suffix:/s` |
| `seconds` | `s` |

### Variables

| `choices` | Grafana variable |
|---|---|
| `field_values(field)` | a query variable, `label_values(field)` |
| `field_values(field, "query")` | a query variable, `label_values(<selector>, field)` |
| `list("value", …)` | a custom variable |
| `free_text()` | a text variable |

A query variable reads its values again on every dashboard load and offers them in alphabetical order.

What Grafana refuses is the DSL's to state, under its renderers in [dashboard-dsl.md](dashboard-dsl.md).

## Releases

A release is a version tag with the archives the Terraform Registry reads: one zip per platform, a SHA-256 checksums file signed with the provider's GPG key, the registry manifest declaring protocol 6, the DSL's schemas, and the JSON Schema of every target schema the provider carries, each major's as its newest carried minor.

A snapshot is a pre-release of the same shape, tagged `v<next patch>-snapshot.<UTC timestamp>` from `main` when the snapshot workflow is dispatched. The Registry serves it like a release, and Terraform installs it only when a configuration pins that exact version.

The Registry pages are `docs/index.md`, `docs/functions/` and `docs/data-sources/`, generated by `make docs` from the definitions, `templates/` and `examples/`.
