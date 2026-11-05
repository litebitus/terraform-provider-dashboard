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

func run(t *testing.T, arguments []string, targetSchemas ...string) (string, *function.FuncError) {
	t.Helper()
	values := make([]attr.Value, 0, len(arguments)+1)
	for _, argument := range arguments {
		values = append(values, types.StringValue(argument))
	}
	elementTypes := make([]attr.Type, len(targetSchemas))
	elements := make([]attr.Value, len(targetSchemas))
	for index, targetSchema := range targetSchemas {
		elementTypes[index], elements[index] = types.StringType, types.StringValue(targetSchema)
	}
	values = append(values, types.TupleValueMust(elementTypes, elements))
	response := &function.RunResponse{Result: function.NewResultData(types.StringUnknown())}
	renderFunction{}.Run(context.Background(), function.RunRequest{Arguments: function.NewArgumentsData(values)}, response)
	if response.Error != nil {
		return "", response.Error
	}
	return response.Result.Value().(types.String).ValueString(), nil
}

func TestRenderAnswersTheDashboardOfTheTargetSchemaTheDocumentDeclares(t *testing.T) {
	got, err := run(t, []string{withTargetSchemas("grafana-v2-13"), "mock-owner", "mock-name"})
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

func TestAProblemBlamesTheArgumentItLiesWith(t *testing.T) {
	for name, testCase := range map[string]struct {
		arguments     []string
		targetSchemas []string
		argument      int64
		says          string
	}{
		"no owner": {[]string{withTargetSchemas("grafana-v2-13"), "", "mock-name"}, nil, 1, "owner is required"},
		"no name":  {[]string{withTargetSchemas("grafana-v2-13"), "mock-owner", ""}, nil, 2, "name is required"},
		"invalid spec": {
			[]string{strings.Replace(mockSpec, "timezone: utc\n", "", 1), "mock-owner", "mock-name"}, nil, 0,
			"missing property 'timezone'",
		},
		"refused by Grafana": {
			[]string{withTargetSchemas("grafana-v2-13"), "mock-owner", "mock-name"}, nil, 0, "",
		},
		"no target schema anywhere": {
			[]string{mockSpec, "mock-owner", "mock-name"}, nil, 3,
			"the document declares no target_schemas; name a whole identifier with target_schema",
		},
		"a target schema not carried": {
			[]string{mockSpec, "mock-owner", "mock-name"}, []string{"grafana-v2-1"}, 3,
			"target schema grafana-v2-1 is not one this provider renders; it renders grafana-v2-13.3",
		},
		"two target schemas named": {
			[]string{mockSpec, "mock-owner", "mock-name"}, []string{"grafana-v2-13", "grafana-v2-13.3"}, 4,
			"a render is for one target schema, not 2",
		},
	} {
		if name == "refused by Grafana" {
			testCase.arguments[0] = strings.Replace(testCase.arguments[0], "kind: time_series", "kind: tag_cloud", 1)
			testCase.says = "Grafana has no tag_cloud panel"
		}
		_, err := run(t, testCase.arguments, testCase.targetSchemas...)
		if err == nil || !strings.Contains(err.Text, testCase.says) || err.FunctionArgument == nil || *err.FunctionArgument != testCase.argument {
			t.Errorf("%s: err = %+v, want %q on argument %d", name, err, testCase.says, testCase.argument)
		}
	}
}

func TestAWholeIdentifierRendersADocumentThatDeclaresNone(t *testing.T) {
	if _, err := run(t, []string{mockSpec, "mock-owner", "mock-name"}, "grafana-v2-13.3"); err != nil {
		t.Error(err)
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
  value = jsondecode(provider::dashboard::render(` + specHeredoc(withTargetSchemas("grafana-v2-13")) + `  , "mock-owner", "mock-name")).spec.title
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
  owner         = "mock-owner"
  name          = "mock-name"
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
  value = provider::dashboard::render(` + specHeredoc(mockSpec) + `  , "mock-owner", "mock-name", "grafana")
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
  spec  = ` + specHeredoc(withTargetSchemas("grafana-v2-13")) + `
  owner = "mock-owner"
  name  = "mock-name"
}
`,
			ExpectError: regexp.MustCompile(`default_target_schemas\[perses\] picks grafana-v2-13, which perses does not\s+match`),
		}},
	})
}
