package grafanarenderer

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/litebitus/terraform-provider-dashboard/dashboarddsl"
	"github.com/litebitus/terraform-provider-dashboard/targetschema"
)

var updateGoldens = flag.Bool("update", false, "write each render to its golden file")

// newestCarried is the last target schema the manifest carries, which every golden render is made for.
func newestCarried(t *testing.T) targetschema.ID {
	t.Helper()
	carried, err := CarriedIDs()
	if err != nil || len(carried) == 0 {
		t.Fatalf("the renderer carries no target schema: %v", err)
	}
	return carried[len(carried)-1]
}

// Every target schema the manifest lists has its JSON Schema beside it, and it compiles.
func TestEveryCarriedTargetSchemaHasItsJSONSchema(t *testing.T) {
	carried, err := TargetSchemas()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range carried {
		if _, err := compiledTargetSchema(target); err != nil {
			t.Errorf("%s: %v", target.ID, err)
		}
	}
}

// A release publishes every carried minor's schema, and each major's as the minor it resolves to.
func TestAReleasePublishesEachTargetSchemaMajorAsItsNewestMinor(t *testing.T) {
	files, err := TargetSchemaFiles()
	if err != nil {
		t.Fatal(err)
	}
	carried, err := CarriedIDs()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range carried {
		if _, published := files[id.String()+".schema.json"]; !published {
			t.Errorf("%s is not published", id)
		}
		major := targetschema.ID{Kind: id.Kind, Version: strings.SplitN(id.Version, ".", 2)[0]}
		newest, err := targetschema.Resolve(major, carried)
		if err != nil {
			t.Fatal(err)
		}
		if string(files[major.String()+".schema.json"]) != string(files[newest.String()+".schema.json"]) {
			t.Errorf("%s.schema.json is not %s's schema", major, newest)
		}
	}
}

// A render that breaks its target schema fails as the provider's defect, naming where it breaks.
func TestARenderThatBreaksItsTargetSchemaFails(t *testing.T) {
	target, err := targetSchemaFor(newestCarried(t))
	if err != nil {
		t.Fatal(err)
	}
	broken := dashboardSpec{Title: "Mock", CursorSync: "mock-sync"}
	err = conforms(target, broken)
	if err == nil || !strings.Contains(err.Error(), "a defect of this provider") || !strings.Contains(err.Error(), "/cursorSync") {
		t.Errorf("err = %v", err)
	}
}

// A render for a target schema the renderer does not carry is refused.
func TestARenderForATargetNotCarriedIsRefused(t *testing.T) {
	dashboard, err := dashboarddsl.ParseDashboard(mockSpec)
	if err != nil {
		t.Fatal(err)
	}
	_, err = RenderDashboard(dashboard, targetschema.ID{Kind: "grafana-v2", Version: "1.0"})
	if err == nil || !strings.Contains(err.Error(), "not one this renderer carries") {
		t.Errorf("err = %v", err)
	}
}

func rendered(t *testing.T, spec string) ([]byte, error) {
	t.Helper()
	dashboard, err := dashboarddsl.ParseDashboard(spec)
	if err != nil {
		t.Fatalf("the spec does not parse: %v", err)
	}
	return RenderDashboard(dashboard, newestCarried(t))
}

// Each spec in testdata renders to its golden JSON.
func TestASpecRendersToItsGoldenJSON(t *testing.T) {
	specs, err := filepath.Glob("testdata/*.yml")
	if err != nil || len(specs) == 0 {
		t.Fatalf("testdata holds no spec: %v", err)
	}
	for _, specFile := range specs {
		spec, err := os.ReadFile(specFile)
		if err != nil {
			t.Fatal(err)
		}
		got, err := rendered(t, string(spec))
		if err != nil {
			t.Fatalf("%s: %v", specFile, err)
		}
		goldenFile := strings.TrimSuffix(specFile, ".yml") + ".json"
		if *updateGoldens {
			if err := os.WriteFile(goldenFile, append(got, '\n'), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		want, err := os.ReadFile(goldenFile)
		if err != nil {
			t.Fatal(err)
		}
		if string(got)+"\n" != string(want) {
			t.Errorf("%s renders differently from %s; run the tests with -update and read the diff", specFile, goldenFile)
		}
	}
}

// A render is the same on every run, with or without an id, so a plan shows no change until the document does.
func TestARenderIsTheSameEveryTime(t *testing.T) {
	first, err := rendered(t, mockSpec)
	if err != nil {
		t.Fatal(err)
	}
	for range 5 {
		again, err := rendered(t, mockSpec)
		if err != nil || string(again) != string(first) {
			t.Fatalf("a second render differs (%v):\n%s\nfrom\n%s", err, again, first)
		}
	}
}

// A document's id is the dashboard's name in Grafana; without one, the render names none and Grafana assigns it.
func TestADocumentsIDIsTheDashboardsName(t *testing.T) {
	for id, want := range map[string]string{"mock-dashboard": "mock-dashboard", "": ""} {
		spec := mockSpec
		if id != "" {
			spec = strings.Replace(mockSpec, "title: Mock\n", "id: "+id+"\ntitle: Mock\n", 1)
		}
		rendered, err := rendered(t, spec)
		if err != nil {
			t.Fatal(err)
		}
		var resource struct {
			Metadata map[string]string `json:"metadata"`
		}
		if err := json.Unmarshal(rendered, &resource); err != nil || resource.Metadata["name"] != want {
			t.Errorf("id %q: metadata %v (%v), want name %q", id, resource.Metadata, err, want)
		}
	}
}

const mockSpec = `schema_version: "1"
title: Mock
time_range: 1h
refresh_interval: 1m
timezone: utc
source:
  kind: metrics
variables:
  mock:
    choices: field_values(job)
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

const mockQueryOfAPanel = "        kind: time_series\n        height: 300\n        unit: count\n        queries:\n" +
	"          - query: __name__:mock_up\n"

// mockTableQuery is the mock panel as a table whose query's value is written by the value_format layout.
func mockTableQuery(valueFormat string) string {
	return "        kind: table\n        height: 300\n        unit: count\n        queries:\n" +
		"          - query: __name__:mock_up\n            value_format: " + valueFormat + "\n"
}

// What Grafana cannot draw is refused, naming the dashboard, where in it, and why.
func TestWhatGrafanaCannotDrawIsRefused(t *testing.T) {
	for name, testCase := range map[string]struct {
		old, replacement string
		says             string
	}{
		"logs source": {
			"  kind: metrics", "  kind: logs\n  index_pattern: mock-*\n  time_field: mock_time",
			`dashboard "Mock": Grafana reads no OpenSearch data source; a logs dashboard renders for OpenSearch Dashboards`,
		},
		"range variable": {
			"field_values(job)", "range(mock_ms)",
			`dashboard "Mock", variable mock: Grafana has no range variable`,
		},
		"id Grafana does not take": {
			"title: Mock\n", "id: Mock_Dashboard\ntitle: Mock\n",
			`dashboard "Mock": id "Mock_Dashboard" is not a name Grafana takes`,
		},
		"tag cloud": {
			"kind: time_series", "kind: tag_cloud",
			`dashboard "Mock", row 1 "Mock row", panel 1 "Mock panel": Grafana has no tag_cloud panel`,
		},
		"field its panel does not draw": {
			"kind: time_series", "kind: table\n        stacked: true",
			`dashboard "Mock", row 1 "Mock row", panel 1 "Mock panel": Grafana's table panel draws no stacked`,
		},
		"query the backend cannot compile": {
			"query: __name__:mock_up", "query: mock",
			`dashboard "Mock", row 1 "Mock row", panel 1 "Mock panel", query 1: a term with no field does not compile to PromQL`,
		},
		"choices query with stages": {
			"field_values(job)", `field_values(job, "__name__:mock_up | count()")`,
			`dashboard "Mock", variable mock: Grafana reads a variable's values from a series selector`,
		},
		"value format with a verb other than a date": {
			mockQueryOfAPanel, mockTableQuery(`"%d"`),
			"dashboard \"Mock\", row 1 \"Mock row\", panel 1 \"Mock panel\", query 1: " +
				"Grafana writes a `value_format` only with `%C`, as a date",
		},
		"value format with a padded date": {
			mockQueryOfAPanel, mockTableQuery(`"%12{%Y}C"`),
			"dashboard \"Mock\", row 1 \"Mock row\", panel 1 \"Mock panel\", query 1: Grafana writes `%C` with no width",
		},
		"value format with a time offset": {
			mockQueryOfAPanel, mockTableQuery(`"%{%Y|+08:00}C"`),
			"dashboard \"Mock\", row 1 \"Mock row\", panel 1 \"Mock panel\", query 1: " +
				"Grafana writes `%C` in the dashboard's time zone, with no time offset",
		},
		"value format with a date verb Grafana does not write": {
			mockQueryOfAPanel, mockTableQuery(`"%{%Y %3m}C"`),
			"dashboard \"Mock\", row 1 \"Mock row\", panel 1 \"Mock panel\", query 1: " +
				"Grafana's date format writes no `%3m`",
		},
		"value format writing the offset": {
			mockQueryOfAPanel, mockTableQuery(`"%{%02H:%02M %z}C"`),
			"dashboard \"Mock\", row 1 \"Mock row\", panel 1 \"Mock panel\", query 1: " +
				"Grafana's date format writes no `%z`",
		},
		"value format whose text holds a closing bracket": {
			mockQueryOfAPanel, mockTableQuery(`"%{[%Y]}C"`),
			"dashboard \"Mock\", row 1 \"Mock row\", panel 1 \"Mock panel\", query 1: " +
				"Grafana's date format holds no `]` in its text",
		},
	} {
		if !strings.Contains(mockSpec, testCase.old) {
			t.Fatalf("%s: the mock spec holds no %q", name, testCase.old)
		}
		_, err := rendered(t, strings.Replace(mockSpec, testCase.old, testCase.replacement, 1))
		var refusal Refusal
		if !errors.As(err, &refusal) || !strings.Contains(err.Error(), testCase.says) {
			t.Errorf("%s: err = %v, want a refusal saying %q", name, err, testCase.says)
		}
	}
}

// A variable's choices are scoped by its own query alone, so a variable the dashboard's filter names chains on the
// variables its query names and never on itself; the panels' queries still apply the filter whole.
func TestAVariablesChoicesTakeOnlyItsOwnQuery(t *testing.T) {
	spec := strings.Replace(mockSpec, "variables:\n  mock:\n    choices: field_values(job)\n",
		"filter: mock_env:${mock_env} and mock_region:${mock_region}\n"+
			"variables:\n  mock_env:\n    choices: field_values(mock_env)\n"+
			"  mock_region:\n    choices: field_values(mock_region, \"mock_env:${mock_env}\")\n", 1)
	output, err := rendered(t, spec)
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)
	wantChoices := `label_values({mock_env=~\"${mock_env}\"}, mock_region)`
	wantPanel := `mock_up{mock_env=~\"${mock_env}\",mock_region=~\"${mock_region}\"}`
	if !strings.Contains(text, wantChoices) || !strings.Contains(text, wantPanel) {
		t.Errorf("render lacks the choices query %s or the panel query %s:\n%s", wantChoices, wantPanel, text)
	}
}

// A value_format's date verbs render as their Moment.js tokens, unpadded and zero-padded alike.
func TestADateFormatRendersItsMomentTokens(t *testing.T) {
	render := &dashboardRender{}
	unit := render.dateUnit("mock", "at %{%Y-%m-%02d %H:%02M:%S.%03L %02y}C")
	want := "time:[at ]YYYY[-]M[-]DD[ ]H[:]mm[:]s[.]SSS[ ]YY"
	if unit != want || len(render.refusals) > 0 {
		t.Errorf("unit = %q, refusals = %v, want %q", unit, render.refusals, want)
	}
}

// A value_format's query is scaled from the digits of a second its `%C` precision states to milliseconds.
func TestAValueFormatScalesItsQueryToMilliseconds(t *testing.T) {
	for valueFormat, want := range map[string]string{
		"%{%Y}C":   "(mock_up) * 1000",
		"%{%Y}.1C": "(mock_up) * 100",
		"%{%Y}.3C": "mock_up",
		"%{%Y}.9C": "(mock_up) / 1000000",
	} {
		if got := millisecondsExpression("mock_up", valueFormat); got != want {
			t.Errorf("%s: expression = %q, want %q", valueFormat, got, want)
		}
	}
}

// A bar panel's value_order sorts the column its calculation is written in, descending where stated.
func TestABarPanelsValueOrderSortsItsCalculationColumn(t *testing.T) {
	for _, testCase := range []struct {
		calculation, valueOrder, column string
		descending                      bool
	}{
		{"last", dashboarddsl.AscendingOrder, "Last", false},
		{"sum", dashboarddsl.DescendingOrder, "Total", true},
	} {
		transformations := valueOrderTransformations(dashboarddsl.Panel{
			Calculation: testCase.calculation, ValueOrder: testCase.valueOrder,
		})
		sortOptions := transformations[1].(map[string]any)["spec"].(map[string]any)["options"].(map[string]any)
		sortedBy := sortOptions["sort"].([]any)[0].(map[string]any)
		if sortedBy["field"] != testCase.column || sortedBy["desc"] != testCase.descending {
			t.Errorf("%s %s: sorted by %v, want %s descending %v",
				testCase.calculation, testCase.valueOrder, sortedBy, testCase.column, testCase.descending)
		}
	}
}

// A value mapping renders its text, and its color where one is stated, which is what puts it in Grafana's legend.
func TestAValueMappingRendersItsTextAndColor(t *testing.T) {
	defaults := fieldDefaults(dashboarddsl.Panel{ValueMappings: map[string]dashboarddsl.ValueMapping{
		"0": {Text: "mock-down", Color: "red"},
		"1": {Text: "mock-up"},
	}})
	mappings := defaults["mappings"].([]any)
	options := mappings[0].(map[string]any)["options"].(map[string]any)
	down, up := options["0"].(map[string]any), options["1"].(map[string]any)
	if down["text"] != "mock-down" || down["color"] != "red" || down["index"] != 0 {
		t.Errorf("mapping 0 = %v", down)
	}
	if _, colored := up["color"]; up["text"] != "mock-up" || colored || up["index"] != 1 {
		t.Errorf("mapping 1 = %v, want its text and no color", up)
	}
}

// A first threshold without a value colors the base step, below the lowest; without one the base step is the text color.
func TestAThresholdWithoutAValueColorsTheBaseStep(t *testing.T) {
	one := 1.0
	for name, testCase := range map[string]struct {
		thresholds []dashboarddsl.Threshold
		steps      []map[string]any
	}{
		"stated": {
			[]dashboarddsl.Threshold{{Color: "red"}, {Value: &one, Color: "green"}},
			[]map[string]any{{"color": "red", "value": nil}, {"color": "green", "value": 1.0}},
		},
		"left to the renderer": {
			[]dashboarddsl.Threshold{{Value: &one, Color: "green"}},
			[]map[string]any{{"color": baseColor, "value": nil}, {"color": "green", "value": 1.0}},
		},
	} {
		defaults := fieldDefaults(dashboarddsl.Panel{Thresholds: testCase.thresholds})
		steps := defaults["thresholds"].(map[string]any)["steps"]
		if !reflect.DeepEqual(steps, testCase.steps) {
			t.Errorf("%s: steps = %v, want %v", name, steps, testCase.steps)
		}
	}
}

func TestPanelsShareTheGridsColumns(t *testing.T) {
	half, quarter := 50, 25
	for name, testCase := range map[string]struct {
		widths []*int
		want   []int
	}{
		"none stated":         {[]*int{nil, nil, nil}, []int{8, 8, 8}},
		"left over to first":  {[]*int{nil, nil, nil, nil, nil}, []int{5, 5, 5, 5, 4}},
		"stated and unstated": {[]*int{&half, nil, &quarter}, []int{12, 6, 6}},
	} {
		panels := make([]dashboarddsl.Panel, len(testCase.widths))
		for index, width := range testCase.widths {
			panels[index].Width = width
		}
		if got, err := panelWidths(panels); err != nil || !slices.Equal(got, testCase.want) {
			t.Errorf("%s: widths %v (%v), want %v", name, got, err, testCase.want)
		}
	}
}
