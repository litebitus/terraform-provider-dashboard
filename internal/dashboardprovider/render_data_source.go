package dashboardprovider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*renderDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*renderDataSource)(nil)
)

// renderDataSource renders a document as the function does, with the provider's default target schemas at the lowest priority.
type renderDataSource struct {
	defaults map[string]string
}

func newRenderDataSource() datasource.DataSource { return &renderDataSource{} }

type renderDataSourceModel struct {
	Spec         types.String `tfsdk:"spec"`
	TargetSchema types.String `tfsdk:"target_schema"`
	JSON         types.String `tfsdk:"json"`
}

func (*renderDataSource) Metadata(_ context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_render"
}

func (*renderDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{
		MarkdownDescription: "Renders a dashboard DSL document into the dashboard JSON of the target schema it resolves to. " +
			"The target schema is `target_schema`, else the one the document's `target_schemas` leave, " +
			"else the provider's `default_target_schemas`. The data source writes nothing: the caller puts `json` where the UI loads it.",
		Attributes: map[string]schema.Attribute{
			specInput: schema.StringAttribute{Required: true, MarkdownDescription: "The dashboard document's text."},
			targetSchemaInput: schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "The target schema to render for: a whole identifier, such as `grafana-v2-13`, " +
					"or the leading words of one, such as `grafana`.",
			},
			"json": schema.StringAttribute{Computed: true, MarkdownDescription: "The rendered dashboard JSON."},
		},
	}
}

func (source *renderDataSource) Configure(_ context.Context, request datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if defaults, configured := request.ProviderData.(map[string]string); configured {
		source.defaults = defaults
	}
}

func (source *renderDataSource) Read(ctx context.Context, request datasource.ReadRequest, response *datasource.ReadResponse) {
	var model renderDataSourceModel
	if response.Diagnostics.Append(request.Config.Get(ctx, &model)...); response.Diagnostics.HasError() {
		return
	}
	rendered, err := renderDocument(renderRequest{
		spec:         model.Spec.ValueString(),
		targetSchema: model.TargetSchema.ValueString(),
		defaults:     source.defaults,
	})
	if err != nil {
		var problem renderProblem
		if errors.As(err, &problem) {
			response.Diagnostics.AddAttributeError(path.Root(problem.input), "The dashboard does not render", problem.Error())
			return
		}
		response.Diagnostics.AddError("The dashboard does not render", err.Error())
		return
	}
	model.JSON = types.StringValue(rendered)
	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
}
