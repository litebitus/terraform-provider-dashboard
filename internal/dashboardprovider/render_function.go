package dashboardprovider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/function"
)

var _ function.Function = renderFunction{}

type renderFunction struct{}

func newRenderFunction() function.Function { return renderFunction{} }

func (renderFunction) Metadata(_ context.Context, _ function.MetadataRequest, response *function.MetadataResponse) {
	response.Name = "render"
}

func (renderFunction) Definition(_ context.Context, _ function.DefinitionRequest, response *function.DefinitionResponse) {
	response.Definition = function.Definition{
		Summary: "Renders a dashboard DSL document into a UI's dashboard JSON, as a string.",
		MarkdownDescription: "Renders a dashboard DSL document into the dashboard JSON of the target schema it resolves to, " +
			"and returns it as a string. The function writes nothing: the caller puts the string where the UI loads it, " +
			"such as the `content` of an `aws_s3_object` in a bucket the UI's file provider reads. " +
			"The target schema is the call's `target_schema`, else the one the document's `target_schemas` leave; " +
			"the provider's `default_target_schemas` apply only to the `dashboard_render` data source. " +
			"The dashboard's name in the UI derives from `owner` and `name`, so the same pair replaces the same dashboard.",
		Parameters: []function.Parameter{
			function.StringParameter{Name: specInput, MarkdownDescription: "The dashboard document's text."},
			function.StringParameter{Name: ownerInput, MarkdownDescription: "Who declares the dashboard."},
			function.StringParameter{Name: nameInput, MarkdownDescription: "The dashboard's name among its owner's."},
		},
		VariadicParameter: function.StringParameter{
			Name: targetSchemaInput,
			MarkdownDescription: "The target schema to render for: a whole identifier, such as `grafana-v2-13`, " +
				"or the leading words of one, such as `grafana`; at most one.",
		},
		Return: function.StringReturn{},
	}
}

// renderArguments are the arguments a problem is blamed on, by position.
var renderArguments = map[string]int64{specInput: 0, ownerInput: 1, nameInput: 2, targetSchemaInput: 3}

func (renderFunction) Run(ctx context.Context, request function.RunRequest, response *function.RunResponse) {
	var spec, owner, name string
	var targetSchemas []string
	if response.Error = request.Arguments.Get(ctx, &spec, &owner, &name, &targetSchemas); response.Error != nil {
		return
	}
	if len(targetSchemas) > 1 {
		response.Error = function.NewArgumentFuncError(renderArguments[targetSchemaInput]+1,
			fmt.Sprintf("a render is for one target schema, not %d", len(targetSchemas)))
		return
	}
	render := renderRequest{spec: spec, owner: owner, name: name}
	if len(targetSchemas) == 1 {
		render.targetSchema = targetSchemas[0]
	}
	rendered, err := renderDocument(render)
	if err != nil {
		var problem renderProblem
		if errors.As(err, &problem) {
			response.Error = function.NewArgumentFuncError(renderArguments[problem.input], problem.Error())
			return
		}
		response.Error = function.NewFuncError(err.Error())
		return
	}
	response.Error = response.Result.Set(ctx, rendered)
}
