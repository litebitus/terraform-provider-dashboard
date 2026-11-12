# Dashboard DSL

A dashboard is written in the dashboard DSL: what it shows, in words no UI owns. One document describes a dashboard completely for every UI, and a renderer answers it in one UI's format. A renderer draws what its UI supports and refuses the rest, naming the dashboard, the panel and why, so a dashboard never draws less than its document says. A new UI is a new renderer, and during a migration both render the same documents side by side.

## Definition

| File | Defines |
|---|---|
| `schemas/dashboard-dsl-v<major>.<minor>.schema.json` | the document's shape: every field, its type and values, and what each panel kind requires, in JSON Schema draft 2020-12 |
| `yaml-dsl.yml` | the DSL as the [YAML DSL standard](https://github.com/litebitus/yaml-dsl-spec/blob/main/docs/spec.md) defines one: which files are dashboards, where their schema is, and variables as the `variable` scope, declared under `variables` and referenced as `${name}` |

A workspace whose dashboards stand elsewhere adds the `dashboard` block to its own `yaml-dsl.yml`, naming the published schema in `schema_search_paths`.

## Versions

| Change | Schema version |
|---|---|
| a breaking change | a new major, `2.0` |
| an additive change, such as a new optional field | a new minor, `1.1`, its file beside the last |

- A released schema is frozen: any change to it, a description's wording included, is a new minor.
- A document's `schema_version` states a major, `"1"`, read against the newest 1.x its reader carries, or a minor, `"1.0"`, read against that minor.
- Each release attaches every minor's schema, and each major's as its newest minor, such as `dashboard-dsl-v1.schema.json`, for a modeline to name.

## Target schemas

A target schema is the dashboard format of one UI at one version, the schema a render is written to. Its identifier is its kind, then its version:

```
grafana-v2-13
└───┬────┘ └┬┘
   kind   version
```

- The kind names the UI and its format, such as `grafana-v2`, Grafana's V2 Resource model.
- The version is a major, `13`, which reads as the newest minor of it a renderer carries, or a minor, `13.3`, which pins that minor.
- A document's `target_schemas` lists the target schemas it renders for, at most one per kind; it is optional. A document names majors, `[grafana-v2-13]`, so it follows the UI's minor releases with the provider and changes only when the UI moves to a new major.
- A render names its target schema in full or by the leading words of an identifier, whole words only: `grafana` and `grafana-v2` match `grafana-v2-13`, `graf` matches nothing. With `target_schemas` declared, the name filters them; without, it names a whole identifier.
- A renderer carries each target schema's JSON Schema, and every render is checked against it.

## Document

A document is a dashboard: a dashboard holds rows, a row holds panels, and a panel holds queries.

### Dashboard

| Field | Holds |
|---|---|
| `schema_version` | the version of the DSL the document is written in, required |
| `target_schemas` | the target schemas the dashboard renders for, at most one per kind; optional |
| `id` | the dashboard's identity in the UI it renders for, unique there; optional, the UI assigning one where it can; a shared template sets it from its parameters |
| `title` | the dashboard's title, required |
| `description` | what the dashboard is for; optional |
| `tags` | the dashboard's tags; optional |
| `time_range` | how far back the dashboard opens, a duration, required |
| `refresh_interval` | how often it queries again, a duration, required |
| `timezone` | the zone times show in: `utc` or `browser`, required |
| `source` | where the dashboard's queries read from, required |
| `filter` | a DQL filter every query also applies, with no stages; optional |
| `links` | links shown with the dashboard, each a `title` and a `url`; optional |
| `annotations` | marks on the time axis, each a `title`, a `query` whose items mark it and an optional `source`; optional |
| `variables` | variable name → the variable's attributes; the dashboard shows its variables in name order |
| `rows` | the rows, top to bottom, at least one |

### Source

| Field | Holds |
|---|---|
| `kind` | `metrics` or `logs` |
| `index_pattern` | for `logs`, the indices the queries read |
| `time_field` | for `logs`, the field the time range filters on |
| `reporting_filter` | for `metrics`, a DQL filter selecting the series the source emits while it reports; optional |

### Variable

A query names a variable as `${<name>}`, the one form a reference takes; the UI substitutes the chosen value. A name is letters, digits and underscores, beginning with a letter or an underscore.

| Field | Holds |
|---|---|
| `choices` | where the variable's choices come from, a call to one of the functions below |
| `default` | the value the dashboard opens with; optional |
| `multiple` | whether several values can be chosen at once; optional, `false` by default |
| `include_all` | whether an "all" choice matches every value; optional, `false` by default |

| Call | Choices |
|---|---|
| `field_values(field)` | the values the field takes |
| `field_values(field, "query")` | the values the field takes among the items a DQL query answers |
| `list("value", …)` | the values as written |
| `free_text()` | whatever the viewer types |
| `range(field)` | a range between a numeric field's least and greatest values |

A call is written as in DQL's stages: the function's name, then its arguments in parentheses, separated by commas. An argument is a field name, a number, or a quoted string.

### Row

| Field | Holds |
|---|---|
| `title` | the row's title |
| `collapsed` | whether the row opens collapsed; optional, `false` by default |
| `repeat_for` | a variable; the row repeats once per chosen value; optional |
| `panels` | the row's panels, left to right, at least one |

### Panel

| Field | Holds |
|---|---|
| `title` | the panel's title |
| `description` | what the panel shows; optional |
| `kind` | what the panel draws, one of the kinds below |
| `width` | the panel's share of the row, in percent; optional, the panels without one sharing what the rest leave equally |
| `height` | the panel's height in pixels; the renderer rounds it to its UI's grid |
| `repeat_for` | a variable; the panel repeats once per chosen value; optional |
| `unit` | the unit of the values: `bytes`, `count`, `percent`, `per_second` or `seconds`; every kind but `text` |
| `decimals` | how many decimals a value shows; optional |
| `min`, `max` | the range values are drawn against; required for `gauge` |
| `calculation` | the one value of each series the panel shows: `first`, `last`, `max`, `mean`, `min` or `sum`; required for `bar`, `gauge`, `pie` and `stat` |
| `thresholds` | the colors a value takes, from the lowest up: each a `color` that holds from its `value` up to the next threshold; the first may leave out `value`, its color then holding from the lowest; optional |
| `value_mappings` | value → what is shown in its place: its `text`, and a `color`, optional; optional |
| `legend` | `show` and, for a shown legend, `position`, `bottom` or `right`; optional |
| `axis` | `scale`, `linear` or `log`, and `label`; optional |
| `stacked` | whether series stack; optional, `false` by default |
| `orientation` | for `bar`, `vertical` or `horizontal` |
| `value_order` | for `bar`, the order of its bars by their values, `ascending` or `descending`; optional, the queries' order where left out |
| `location_field` | for `map`, the field holding each item's location |
| `content` | the Markdown a `text` panel shows, required for `text` |
| `links` | links from the panel, each a `title` and a `url`, which may name variables; optional |
| `queries` | the panel's queries; every kind but `text` |

A color is one of `blue`, `green`, `grey`, `orange`, `purple`, `red` or `yellow`. Where the first threshold states a `value`, a value below it takes no threshold color, and a renderer draws it in its UI's default.

A panel colors its values from one source:

| Source | A value's color | The legend lists |
|---|---|---|
| `thresholds` | the threshold it falls in; a mapping in the panel states no `color` | the thresholds' ranges |
| value mappings' `color` | its mapping's | each mapped value's `text`, in its color |

### Query

| Field | Holds |
|---|---|
| `query` | the query, in DQL |
| `source` | where it reads from, when it is not the dashboard's `source`; optional |
| `legend_format` | how a series is named, by its labels; optional |
| `min_interval` | the shortest step a time series is drawn at, a duration; optional |
| `value_column_header` | for `table`, the header of the column the query's values fill; optional |
| `value_format` | for `table`, the layout the query's value is written by, in the [YAML DSL standard's format function](https://github.com/litebitus/yaml-dsl-spec/blob/main/docs/spec.md), its one verb taking the value, such as `%{%Y-%02m-%02d %02H:%02M:%02S}C` for a time's date and time; optional |

### Kinds

| `kind` | Draws |
|---|---|
| `area` | each series as a filled line over time |
| `bar` | each series' value as a bar |
| `gauge` | one value against a range |
| `heatmap` | a distribution over time, from the buckets the query answers |
| `histogram` | a distribution of values |
| `map` | each item at its location |
| `pie` | each series' share of the whole |
| `records` | the items the query answers, as a list; its columns, order and size are the query's `fields`, `sort` and `limit` |
| `stat` | one value, large |
| `state_timeline` | each series' state over time as a band |
| `status_history` | each series' value at each time as a cell |
| `table` | the series and their values as rows; several queries' values side by side, a row for each value of the fields they share |
| `tag_cloud` | each value of a field, sized by its count |
| `text` | Markdown |
| `time_series` | each series as a line over time |

```yaml
schema_version: "1"
title: Mock Service
time_range: 6h
refresh_interval: 1m
timezone: utc
source:
  kind: metrics
variables:
  env:
    choices: field_values(env)
    default: dev
  job:
    multiple: true
    choices: field_values(job, "__name__:mock_requests_total and env:${env}")
rows:
  - title: Traffic
    panels:
      - title: Requests by job
        kind: time_series
        height: 300
        unit: per_second
        queries:
          - query: __name__:mock_requests_total and env:${env} and job:${job} | rate(5m) | sum() by job
            legend_format: "{{job}}"
```

### Rules

Beyond its shape, a document holds to these:

- Every `${name}`, in a query, the filter, a `legend_format` or a link's `url`, and every `repeat_for`, names a declared variable.
- Thresholds rise from the lowest, and `min` is below `max`.
- The stated widths of a row's panels add up to at most the row, leaving room for any panel without one.
- `orientation`, `value_order`, `location_field` and `content` stand only on the kind each is for, and a `text` panel holds its content and none of the fields that draw values.
- A query's `value_column_header` and `value_format` stand only on a `table`, and a `value_format` is a layout the format function reads, holding one verb.

## Queries

A `query` is DQL with pipe stages, the language of the [DQL compiler](https://github.com/litebitus/dql-compiler/blob/main/docs/design.md). A renderer compiles it, in the latest grammar version its DQL compiler supports, into the backend its source names; a query that does not compile is refused with the compiler's message. A variable reference, `${env}`, passes through to the UI, which substitutes it. A `legend_format` names a series by its labels, `{{job}}`, and may hold text and variable references beside them.

## Renderers

### Grafana

Grafana's target schemas are of kind `grafana-v2`, Grafana's V2 Resource dashboard. A document's `id` is the dashboard's name in Grafana; without one, Grafana assigns a name and keeps it with the dashboard's file. A `metrics` source reads the UI's default data source; Grafana renders no `logs` source. Its default for a value below the first threshold is the theme's text color.

| Refused | Why |
|---|---|
| a `logs` source | Grafana reads no OpenSearch data source |
| `range()` | Grafana has no range variable |
| `tag_cloud` | Grafana has no such panel |
| an `id` other than lowercase letters, digits and hyphens, beginning and ending with a letter or digit, at most 40 | Grafana takes no other name |
| a panel field the kind's Grafana panel does not draw, such as `stacked` on a `stat` | the panel would draw less than the document says |
| a variable's choices query with stages or arithmetic | Grafana reads a variable's values from a series selector |
| a `value_format` with a verb other than `%C`, or a `%C` with a width | Grafana writes a value as text only through its date unit, which writes a time |
| a `%C` date format with a time offset | Grafana writes a time in the dashboard's time zone |
| a date verb no Moment.js token writes alike, such as `%y`, `%3m` or `%z` | Grafana's date unit writes a date by Moment.js tokens |
| a `value_format` whose text holds `]` | Grafana's date format writes text between brackets |

### OpenSearch Dashboards

OpenSearch Dashboards' render is saved objects: the dashboard, one visualization per panel, and a control per variable. A `logs` source reads its index pattern; OpenSearch Dashboards renders no `metrics` source. A query's filter is the visualization's search, in DQL as written, and its stages are the visualization's aggregations.

| Refused | Why |
|---|---|
| a `metrics` source | OpenSearch Dashboards reads no Prometheus |
| a variable referenced other than as `<field>:${name}` | a control filters every panel on its field; OpenSearch Dashboards substitutes no text |
| `list()` and `free_text()` | a control's choices are a field's values or range |
| several queries in a panel that differ in their stages | a visualization runs one search; queries sharing their stages split it, one bucket each |
| a `legend_format` other than the split field's name | a series is named by its split's value |
| `unit` and `decimals` that differ between panels on one field | values are formatted per field on the index pattern |
| `repeat_for`, `links`, `annotations`, `state_timeline`, `status_history`, `value_mappings` | OpenSearch Dashboards has no equivalent |

## TODO

- Library panels: a panel defined once and referenced from several dashboards, so an edit to its definition reaches every dashboard that references it.
