package dashboardprovider

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ function.Function = renderFunction{}

type renderFunction struct{}

func newRenderFunction() function.Function { return renderFunction{} }

func (renderFunction) Metadata(_ context.Context, _ function.MetadataRequest, response *function.MetadataResponse) {
	response.Name = "render"
}

// renderOptions are the keys the function's options map takes.
var renderOptions = []string{targetSchemaInput}

func (renderFunction) Definition(_ context.Context, _ function.DefinitionRequest, response *function.DefinitionResponse) {
	response.Definition = function.Definition{
		Summary: "Renders a dashboard DSL document into a UI's dashboard JSON, as a string.",
		MarkdownDescription: "Renders a dashboard DSL document into the dashboard JSON of the target schema it resolves to, " +
			"and returns it as a string. The function writes nothing: the caller puts the string where the UI loads it, " +
			"such as the `content` of an `aws_s3_object` in a bucket the UI's file provider reads. " +
			"The target schema is the options' `target_schema`, else the one the document's `target_schemas` leave; " +
			"the provider's `default_target_schemas` apply only to the `dashboard_render` data source.",
		Parameters: []function.Parameter{
			function.StringParameter{Name: specInput, MarkdownDescription: "The dashboard document's text."},
		},
		VariadicParameter: function.MapParameter{
			Name:        "options",
			ElementType: types.StringType,
			MarkdownDescription: "At most one map of options: `target_schema`, the target schema to render for, " +
				"a whole identifier such as `grafana-v2-13` or the leading words of one such as `grafana`.",
		},
		Return: function.StringReturn{},
	}
}

const (
	specArgument    = 0
	optionsArgument = 1
)

func (renderFunction) Run(ctx context.Context, request function.RunRequest, response *function.RunResponse) {
	var spec string
	var options []map[string]string
	if response.Error = request.Arguments.Get(ctx, &spec, &options); response.Error != nil {
		return
	}
	if len(options) > 1 {
		response.Error = function.NewArgumentFuncError(optionsArgument+1,
			fmt.Sprintf("the options are one map, not %d", len(options)))
		return
	}
	render := renderRequest{spec: spec}
	if len(options) == 1 {
		for _, key := range slices.Sorted(maps.Keys(options[0])) {
			if !slices.Contains(renderOptions, key) {
				response.Error = function.NewArgumentFuncError(optionsArgument,
					fmt.Sprintf("%q is not an option; the options are %s", key, strings.Join(renderOptions, ", ")))
				return
			}
		}
		render.targetSchema = options[0][targetSchemaInput]
	}
	rendered, err := renderDocument(render)
	if err != nil {
		var problem renderProblem
		if errors.As(err, &problem) {
			argument := int64(specArgument)
			if problem.input == targetSchemaInput {
				argument = optionsArgument
			}
			response.Error = function.NewArgumentFuncError(argument, problem.Error())
			return
		}
		response.Error = function.NewFuncError(err.Error())
		return
	}
	response.Error = response.Result.Set(ctx, rendered)
}
