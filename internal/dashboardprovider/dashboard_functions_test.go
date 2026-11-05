package dashboardprovider

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// mockSpec declares no target schema; withTargetSchemas declares the ones given.
const mockSpec = `schema_version: "1"
title: Mock
time_range: 1h
refresh_interval: 1m
timezone: utc
source:
  kind: metrics
rows:
  - title: Mock row
    panels:
      - title: Mock panel
        kind: time_series
        height: 300
        unit: count
        queries:
          - query: __name__:mock_up
`

func withTargetSchemas(declared string) string {
	return strings.Replace(mockSpec, "title: Mock\n", "target_schemas: ["+declared+"]\ntitle: Mock\n", 1)
}

// run calls the function with the spec and the option maps given, as Terraform passes them.
func run(t *testing.T, spec string, options ...map[string]string) (string, *function.FuncError) {
	t.Helper()
	elementTypes := make([]attr.Type, len(options))
	elements := make([]attr.Value, len(options))
	for index, option := range options {
		values := map[string]attr.Value{}
		for key, value := range option {
			values[key] = types.StringValue(value)
		}
		elementTypes[index], elements[index] = types.MapType{ElemType: types.StringType}, types.MapValueMust(types.StringType, values)
	}
	arguments := []attr.Value{types.StringValue(spec), types.TupleValueMust(elementTypes, elements)}
	response := &function.RunResponse{Result: function.NewResultData(types.StringUnknown())}
	renderFunction{}.Run(context.Background(), function.RunRequest{Arguments: function.NewArgumentsData(arguments)}, response)
	if response.Error != nil {
		return "", response.Error
	}
	return response.Result.Value().(types.String).ValueString(), nil
}

func TestRenderAnswersTheDashboardOfTheTargetSchemaTheDocumentDeclares(t *testing.T) {
	got, err := run(t, withTargetSchemas("grafana-v2-13"))
	if err != nil {
		t.Fatal(err)
	}
	var dashboard struct {
		APIVersion string `json:"apiVersion"`
		Spec       struct {
			Title string `json:"title"`
		} `json:"spec"`
	}
	if err := json.Unmarshal([]byte(got), &dashboard); err != nil || dashboard.APIVersion != "dashboard.grafana.app/v2" || dashboard.Spec.Title != "Mock" {
		t.Errorf("dashboard = %+v (%v)", dashboard, err)
	}
}

func TestTheOptionsNameTheTargetSchema(t *testing.T) {
	if _, err := run(t, mockSpec, map[string]string{"target_schema": "grafana-v2-13.3"}); err != nil {
		t.Error(err)
	}
}

func TestAProblemBlamesTheArgumentItLiesWith(t *testing.T) {
	for name, testCase := range map[string]struct {
		spec     string
		options  []map[string]string
		argument int64
		says     string
	}{
		"invalid spec": {
			strings.Replace(mockSpec, "timezone: utc\n", "", 1), nil, specArgument, "missing property 'timezone'",
		},
		"refused by Grafana": {
			strings.Replace(withTargetSchemas("grafana-v2-13"), "kind: time_series", "kind: tag_cloud", 1), nil,
			specArgument, "Grafana has no tag_cloud panel",
		},
		"no target schema anywhere": {
			mockSpec, nil, optionsArgument,
			"the document declares no target_schemas; name a whole identifier with target_schema",
		},
		"a target schema not carried": {
			mockSpec, []map[string]string{{"target_schema": "grafana-v2-1"}}, optionsArgument,
			"target schema grafana-v2-1 is not one this provider renders; it renders grafana-v2-13.3",
		},
		"an unknown option": {
			mockSpec, []map[string]string{{"target_schemas": "grafana-v2-13"}}, optionsArgument,
			`"target_schemas" is not an option; the options are target_schema`,
		},
		"two option maps": {
			mockSpec, []map[string]string{{}, {}}, optionsArgument + 1, "the options are one map, not 2",
		},
	} {
		_, err := run(t, testCase.spec, testCase.options...)
		if err == nil || !strings.Contains(err.Text, testCase.says) || err.FunctionArgument == nil || *err.FunctionArgument != testCase.argument {
			t.Errorf("%s: err = %+v, want %q on argument %d", name, err, testCase.says, testCase.argument)
		}
	}
}

var protocolProviders = map[string]func() (tfprotov6.ProviderServer, error){
	"dashboard": providerserver.NewProtocol6WithError(New("test")()),
}

// specHeredoc is a spec as an HCL heredoc.
func specHeredoc(spec string) string { return "<<-SPEC\n" + spec + "SPEC\n" }

func TestTheFunctionAnswersInAPlan(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protocolProviders,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_8_0)},
		Steps: []resource.TestStep{{
			Config: `
output "title" {
  value = jsondecode(provider::dashboard::render(` + specHeredoc(mockSpec) + `  , { target_schema = "grafana-v2-13" })).spec.title
}
`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownOutputValue("title", knownvalue.StringExact("Mock")),
			},
		}},
	})
}

const mockDefaultsProvider = `
provider "dashboard" {
  default_target_schemas = { grafana = "grafana-v2-13" }
}
`

// The data source picks by the provider's defaults.
func TestTheDataSourceAppliesTheProviderDefaults(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protocolProviders,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_8_0)},
		Steps: []resource.TestStep{{
			Config: mockDefaultsProvider + `
data "dashboard_render" "mock" {
  spec          = ` + specHeredoc(mockSpec) + `
  target_schema = "grafana"
}

output "api_version" {
  value = jsondecode(data.dashboard_render.mock.json).apiVersion
}
`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownOutputValue("api_version", knownvalue.StringExact("dashboard.grafana.app/v2")),
			},
		}},
	})
}

// The function cannot see the provider's defaults, so where the data source picks by them, the function refuses.
func TestTheFunctionDoesNotSeeTheProviderDefaults(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protocolProviders,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_8_0)},
		Steps: []resource.TestStep{{
			Config: mockDefaultsProvider + `
output "rendered" {
  value = provider::dashboard::render(` + specHeredoc(mockSpec) + `  , { target_schema = "grafana" })
}
`,
			ExpectError: regexp.MustCompile(`target_schema grafana is not a whole identifier`),
		}},
	})
}

func TestInvalidDefaultsFailTheProvider(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protocolProviders,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_8_0)},
		Steps: []resource.TestStep{{
			Config: `
provider "dashboard" {
  default_target_schemas = { perses = "grafana-v2-13" }
}

data "dashboard_render" "mock" {
  spec = ` + specHeredoc(withTargetSchemas("grafana-v2-13")) + `
}
`,
			ExpectError: regexp.MustCompile(`default_target_schemas\[perses\] picks grafana-v2-13, which perses does not\s+match`),
		}},
	})
}
