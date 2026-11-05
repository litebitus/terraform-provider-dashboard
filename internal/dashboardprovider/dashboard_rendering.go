package dashboardprovider

import (
	"fmt"

	"github.com/litebitus/terraform-provider-dashboard/dashboarddsl"
	"github.com/litebitus/terraform-provider-dashboard/grafanarenderer"
	"github.com/litebitus/terraform-provider-dashboard/targetschema"
)

// renderRequest is one render, as the function and the data source state it.
type renderRequest struct {
	spec         string
	targetSchema string
	defaults     map[string]string
}

// renderProblem is why a render failed, and which of the request's inputs it lies with.
type renderProblem struct {
	input string
	err   error
}

func (problem renderProblem) Error() string { return problem.err.Error() }

// The inputs a problem lies with.
const (
	specInput         = "spec"
	targetSchemaInput = "target_schema"
)

// dashboardRenderer renders a document for the target schemas it carries.
type dashboardRenderer struct {
	carried func() ([]targetschema.ID, error)
	render  func(dashboard dashboarddsl.Dashboard, target targetschema.ID) ([]byte, error)
}

// renderers are the renderers the provider carries.
var renderers = []dashboardRenderer{
	{carried: grafanarenderer.CarriedIDs, render: grafanarenderer.RenderDashboard},
}

// renderDocument renders a document for the one target schema the request leaves, resolved among those the provider carries.
func renderDocument(request renderRequest) (string, error) {
	dashboard, err := dashboarddsl.ParseDashboard(request.spec)
	if err != nil {
		return "", renderProblem{specInput, err}
	}
	selected, err := targetschema.Select(targetschema.Request{
		TargetSchema: request.targetSchema, Declared: dashboard.TargetSchemas, Defaults: request.defaults,
	})
	if err != nil {
		return "", renderProblem{targetSchemaInput, err}
	}
	var carried []targetschema.ID
	byKind := map[string]dashboardRenderer{}
	for _, renderer := range renderers {
		ids, err := renderer.carried()
		if err != nil {
			return "", err
		}
		for _, id := range ids {
			byKind[id.Kind] = renderer
		}
		carried = append(carried, ids...)
	}
	resolved, err := targetschema.Resolve(selected, carried)
	if err != nil {
		return "", renderProblem{targetSchemaInput, err}
	}
	rendered, err := byKind[resolved.Kind].render(dashboard, resolved)
	if err != nil {
		return "", renderProblem{specInput, fmt.Errorf("target schema %s: %w", resolved, err)}
	}
	return string(rendered), nil
}
