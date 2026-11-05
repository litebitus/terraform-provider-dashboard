# Terraform Provider Dashboard

Terraform functions and data sources that render dashboard DSL documents into each UI's dashboard format at plan time.

## Features

- `provider::dashboard::render` and the `dashboard_render` data source render a document into the dashboard JSON of its target schema, today Grafana's V2 Resource dashboard, `grafana-v2-13`.
- A document names the target schemas it renders for; the call, the document and the provider's `default_target_schemas` choose one, in that order, so moving to a new UI version is one setting.
- A document is held to the [JSON Schema](schemas/) its `schema_version` names, published with every release, and to the rules a schema cannot state; every problem fails the plan, each named at its place.
- Every render is checked against its target schema's JSON Schema, generated from the UI's own schema and published with every release.
- What a UI cannot draw is refused, naming the dashboard, the panel and why, so a plan never ships a dashboard that draws less than it says.
- Queries are DQL, compiled with the [DQL compiler](https://github.com/litebitus/dql-compiler) bundled in the release.
- [`yaml-dsl.yml`](yaml-dsl.yml) defines the DSL for the [YAML DSL editor](https://github.com/litebitus/vscode-yaml-dsl-editor) and every reader of the [YAML DSL standard](https://github.com/litebitus/yaml-dsl-spec).

The DSL is specified in [docs/dashboard-dsl.md](docs/dashboard-dsl.md) and the provider's design is in [docs/design.md](docs/design.md); the arguments are on the [Terraform Registry](https://registry.terraform.io/providers/litebitus/dashboard/latest/docs).

## Examples

| | Example | Documentation |
|---|---|---|
| `render` | [`examples/functions/render/`](examples/functions/render) | [docs/functions/render.md](docs/functions/render.md) |
| `dashboard_render` | [`examples/data-sources/dashboard_render/`](examples/data-sources/dashboard_render) | [docs/data-sources/render.md](docs/data-sources/render.md) |

Both render [`examples/dashboards/mock-service.yml`](examples/dashboards/mock-service.yml).

## Usage

Provider functions need Terraform 1.8 or later.

```hcl
terraform {
  required_providers {
    dashboard = {
      source = "litebitus/dashboard"
    }
  }
}

provider "dashboard" {
  default_target_schemas = {
    grafana = "grafana-v2-13"
  }
}

data "dashboard_render" "mock_service" {
  spec = file("dashboards/mock-service.yml")
}

resource "aws_s3_object" "mock_service_dashboard" {
  bucket       = "mock-config-bucket"
  key          = "grafana/dashboards/mock-owner/mock-service.json"
  content_type = "application/json"
  content      = data.dashboard_render.mock_service.json
}
```

## Published schemas

Every release attaches the schemas as assets, for a modeline or `schema_search_paths` to name:

| Schema | Asset | In the latest release |
|---|---|---|
| the DSL, at a minor | `dashboard-dsl-v<major>.<minor>.schema.json` | [`dashboard-dsl-v1.0.schema.json`](https://github.com/litebitus/terraform-provider-dashboard/releases/latest/download/dashboard-dsl-v1.0.schema.json) |
| the DSL, a major as its newest minor | `dashboard-dsl-v<major>.schema.json` | [`dashboard-dsl-v1.schema.json`](https://github.com/litebitus/terraform-provider-dashboard/releases/latest/download/dashboard-dsl-v1.schema.json) |
| a target schema the provider carries, at a minor | `<kind>-<major>.<minor>.schema.json` | [`grafana-v2-13.3.schema.json`](https://github.com/litebitus/terraform-provider-dashboard/releases/latest/download/grafana-v2-13.3.schema.json) |
| a target schema, a major as its newest carried minor | `<kind>-<major>.schema.json` | [`grafana-v2-13.schema.json`](https://github.com/litebitus/terraform-provider-dashboard/releases/latest/download/grafana-v2-13.schema.json) |

A dashboard document names the DSL's newest 1.x, so its modeline never needs editing:

```yaml
# yaml-language-server: $schema=https://github.com/litebitus/terraform-provider-dashboard/releases/latest/download/dashboard-dsl-v1.schema.json
```

## Development

Go, Terraform and Docker build and test the repo; GoReleaser builds the snapshot archives.

| Target | Does |
|---|---|
| `make build` | runs the tests, then builds the provider into `bin/` |
| `make test` | `go vet`, a `gofmt` check, the Go tests including plans through Terraform, the Grafana integration test, and `tfplugindocs validate` |
| `make docs` | generates the Registry pages in `docs/` from the definitions, `templates/` and `examples/` |
| `make target-schemas` | generates each carried target schema's JSON Schema from the UI's source at its release |
| `make install` | builds the provider and prints the `dev_overrides` block that points local plans at it |
| `make snapshot` | runs the tests, then builds every platform's archive, unsigned, into `dist/` |

The Grafana integration test runs the Grafana release of each carried target schema and loads every panel kind into it through its file provider, as the monitoring UI loads dashboards.

The test pulls the Grafana images from the repository's copies in the GitHub Container Registry, `ghcr.io/litebitus/grafana/grafana`, so Docker Hub's rate limit cannot fail a run. The `Mirror images` workflow runs before every CI test run and copies each carried release not yet there. When a copy cannot be pulled, such as a release not yet copied, the test runs Docker Hub's image.

### Releases

The `Release` workflow, dispatched by hand, runs the tests, computes the next version, stamps `CHANGELOG.md`, tags it, and publishes the signed archives the Terraform Registry reads, with the DSL's schemas and the target schemas'. It signs with the GPG key in the `GPG_PRIVATE_KEY` and `PASSPHRASE` secrets, whose public key is registered with the Registry.

The `Snapshot` workflow, dispatched by hand, publishes a signed pre-release such as `v0.0.1-snapshot.20261105175512`. A configuration installs it by pinning that exact version:

```hcl
dashboard = {
  source  = "litebitus/dashboard"
  version = "0.0.1-snapshot.20261105175512"
}
```
