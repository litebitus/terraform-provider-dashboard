// Package grafanamirror names the repository's copies, in the GitHub Container Registry, of the Grafana images the integration test runs.
package grafanamirror

import "github.com/litebitus/terraform-provider-dashboard/grafanarenderer"

// Registry prefixes every Grafana image the repository copies.
const Registry = "ghcr.io/litebitus/"

// Image is the repository's copy of the Grafana image of the target schema's release.
func Image(target grafanarenderer.TargetSchema) string { return Registry + target.GrafanaImage() }
