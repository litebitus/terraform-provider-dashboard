# Changelog

## Unreleased

## v0.1.1 — 2026-11-08

- Dashboard DSL
  - `schemas/dashboard-dsl-v1.1.schema.json`: a value mapping is its `text` and an optional `color`, which puts the value in a legend.
  - The first threshold may leave out `value`, its color then holding from the lowest.
  - A panel colors its values from one source: a panel with `thresholds` holds no value mapping with a `color`.
  - Problems fence every key and value they name and word schema failures by the YAML DSL standard's table.
  - A value failing a `not`, or matching none of a definition's forms, carries the schema's description as its schema note, `[schema]` and the description as written.
  - A problem is reported as a compiler reports one, located at `dashboard:"<title>":<line>:<column>`.

### Deprecated

- Dashboard DSL
  - `value_mappings`: a value mapped to a string, such as `"0": none`; write it as `"0": { text: none }`.

## v0.1.0 — 2026-11-06

- Rendering
  - `render` function and `dashboard_render` data source: render a dashboard DSL document into the dashboard JSON of its target schema.
  - Grafana target schema `grafana-v2-13.3`, Grafana 13.3's V2 Resource dashboard; every render is checked against its JSON Schema.
  - Provider `default_target_schemas`: the data source's target schema when the call and the document leave several or none.
- Dashboard DSL
  - `schemas/dashboard-dsl-v1.0.schema.json`: the document schema, published with every release beside `dashboard-dsl-v1.schema.json`, the newest 1.x.
  - `target_schemas`: the target schemas a document renders for.
  - `id`: the dashboard's identity in the UI it renders for, assigned by the UI when left out.
  - `yaml-dsl.yml`: the DSL's definition for YAML DSL readers.
- Development
  - `make build`, which tests first, `make test`, `make docs`, `make target-schemas`, `make snapshot` and `make install`.
  - CI runs the tests on every push and pull request, the Grafana integration test included.
  - The integration test pulls its Grafana images from the repository's copies in the GitHub Container Registry, mirrored from Docker Hub once per release.
  - A dispatched snapshot workflow publishes a signed pre-release that a configuration installs by pinning its exact version.
  - A dispatched release workflow stamps the changelog, tags the version, and publishes the signed archives the Terraform Registry reads, with the DSL's and the target schemas' JSON Schemas.
