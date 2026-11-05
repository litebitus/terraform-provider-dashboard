// Package dashboardprovider is a Terraform provider whose function and data source render dashboard DSL documents into each UI's format.
package dashboardprovider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/litebitus/terraform-provider-dashboard/targetschema"
)

var _ provider.ProviderWithFunctions = (*dashboardProvider)(nil)

type dashboardProvider struct {
	version string
}

// New answers the provider constructor for one release version.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &dashboardProvider{version: version}
	}
}

type dashboardProviderModel struct {
	DefaultTargetSchemas types.Map `tfsdk:"default_target_schemas"`
}

func (dashboard *dashboardProvider) Metadata(_ context.Context, _ provider.MetadataRequest, response *provider.MetadataResponse) {
	response.TypeName = "dashboard"
	response.Version = dashboard.version
}

func (dashboard *dashboardProvider) Schema(_ context.Context, _ provider.SchemaRequest, response *provider.SchemaResponse) {
	response.Schema = schema.Schema{
		MarkdownDescription: "Renders dashboard DSL documents into each UI's dashboard format.",
		Attributes: map[string]schema.Attribute{
			"default_target_schemas": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				MarkdownDescription: "The target schema the `dashboard_render` data source picks when the document and the call leave several or none: " +
					"the leading words of an identifier, such as `grafana`, to the whole identifier it picks, such as `grafana-v2-13`.",
			},
		},
	}
}

func (dashboard *dashboardProvider) Configure(ctx context.Context, request provider.ConfigureRequest, response *provider.ConfigureResponse) {
	var model dashboardProviderModel
	if response.Diagnostics.Append(request.Config.Get(ctx, &model)...); response.Diagnostics.HasError() {
		return
	}
	defaults := map[string]string{}
	if response.Diagnostics.Append(model.DefaultTargetSchemas.ElementsAs(ctx, &defaults, false)...); response.Diagnostics.HasError() {
		return
	}
	if err := targetschema.CheckDefaults(defaults); err != nil {
		response.Diagnostics.AddAttributeError(path.Root("default_target_schemas"), "Invalid default target schemas", err.Error())
		return
	}
	response.DataSourceData = defaults
}

func (dashboard *dashboardProvider) Resources(context.Context) []func() resource.Resource { return nil }

func (dashboard *dashboardProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		newRenderDataSource,
	}
}

func (dashboard *dashboardProvider) Functions(context.Context) []func() function.Function {
	return []func() function.Function{
		newRenderFunction,
	}
}
