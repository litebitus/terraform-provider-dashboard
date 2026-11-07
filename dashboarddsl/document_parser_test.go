package dashboarddsl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const mockDashboard = `schema_version: "1"
title: Mock Service
time_range: 6h
refresh_interval: 1m
timezone: utc
source:
  kind: metrics
variables:
  env:
    choices: field_values(env)
    default: mock
  job:
    multiple: true
    choices: field_values(job, "__name__:mock_requests_total and env:${env}")
rows:
  - title: Traffic
    panels:
      - title: Requests by job
        kind: time_series
        height: 300
        unit: per_second
        queries:
          - query: __name__:mock_requests_total and env:${env} and job:${job} | rate(5m) | sum() by job
            legend_format: "{{job}}"
`

// changed is the mock dashboard with one piece of its text replaced.
func changed(t *testing.T, old, replacement string) string {
	t.Helper()
	if !strings.Contains(mockDashboard, old) {
		t.Fatalf("the mock dashboard holds no %q", old)
	}
	return strings.Replace(mockDashboard, old, replacement, 1)
}

func TestADocumentParsesIntoItsDashboard(t *testing.T) {
	dashboard, err := ParseDashboard(mockDashboard)
	if err != nil {
		t.Fatal(err)
	}
	panel := dashboard.Rows[0].Panels[0]
	if dashboard.Title != "Mock Service" || dashboard.Source.Kind != MetricsSource || panel.Kind != TimeSeriesKind ||
		panel.Height != 300 || panel.Queries[0].LegendFormat != "{{job}}" || !dashboard.Variables["job"].Multiple {
		t.Errorf("dashboard = %+v", dashboard)
	}
}

func TestADocumentStatesItsMajorOrAMinor(t *testing.T) {
	for _, stated := range []string{`"1"`, `"1.0"`, `"1.1"`} {
		if _, err := ParseDashboard(changed(t, `schema_version: "1"`, "schema_version: "+stated)); err != nil {
			t.Errorf("schema_version %s: %v", stated, err)
		}
	}
}

// A value mapping is its text and the color stated for it; the deprecated form, the text alone, reads as a mapping with no color.
func TestAValueMappingReadsInEitherForm(t *testing.T) {
	spec := changed(t, "        height: 300\n", "        height: 300\n        value_mappings:\n"+
		"          \"0\": { text: mock-down, color: red }\n          \"1\": mock-up\n")
	dashboard, err := ParseDashboard(spec)
	if err != nil {
		t.Fatal(err)
	}
	mappings := dashboard.Rows[0].Panels[0].ValueMappings
	if mappings["0"] != (ValueMapping{Text: "mock-down", Color: "red"}) || mappings["1"] != (ValueMapping{Text: "mock-up"}) {
		t.Errorf("value mappings = %+v", mappings)
	}
	for name, mapping := range map[string]string{
		"no text":       `{ color: red }`,
		"unknown color": `{ text: mock-down, color: mock-color }`,
	} {
		refused := changed(t, "        height: 300\n", "        height: 300\n        value_mappings:\n          \"0\": "+mapping+"\n")
		if _, err := ParseDashboard(refused); err == nil {
			t.Errorf("%s: the mapping %s parsed", name, mapping)
		}
	}
}

func TestEveryExampleDashboardParses(t *testing.T) {
	examples, err := filepath.Glob("../examples/dashboards/*.yml")
	if err != nil || len(examples) == 0 {
		t.Fatalf("examples/dashboards holds no dashboard: %v", err)
	}
	for _, example := range examples {
		spec, err := os.ReadFile(example)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ParseDashboard(string(spec)); err != nil {
			t.Errorf("%s: %v", example, err)
		}
	}
}

func TestAVariablesChoicesReadAsACallToAChoiceFunction(t *testing.T) {
	for choices, want := range map[string]ChoiceCall{
		`field_values(env)`:               {Function: FieldValuesChoices, Field: "env"},
		`field_values(job, "env:${env}")`: {Function: FieldValuesChoices, Field: "job", Query: "env:${env}"},
		`list("mock-a", "mock-b")`:        {Function: ListChoices, Values: []string{"mock-a", "mock-b"}},
		`free_text()`:                     {Function: FreeTextChoices},
		`range(latency_ms)`:               {Function: RangeChoices, Field: "latency_ms"},
	} {
		got, err := Variable{Choices: choices}.ChoiceCall()
		if err != nil || got.Function != want.Function || got.Field != want.Field || got.Query != want.Query ||
			strings.Join(got.Values, ",") != strings.Join(want.Values, ",") {
			t.Errorf("%s: %+v (%v), want %+v", choices, got, err, want)
		}
	}
}

// Every problem is named at its place, in the words a reader of the document uses.
func TestAProblemIsNamedAtItsPlace(t *testing.T) {
	for name, testCase := range map[string]struct {
		spec string
		says string
	}{
		"empty document": {"", "the document is empty"},
		"unknown version": {
			changed(t, `schema_version: "1"`, `schema_version: "2"`),
			`schema_version "2" is not a version this provider reads`,
		},
		"unquoted version": {changed(t, `schema_version: "1"`, `schema_version: 1`), "schema_version is required, quoted"},
		"two target schemas of one kind": {
			changed(t, "title: Mock Service\n", "target_schemas: [grafana-v2-13, grafana-v2-14]\ntitle: Mock Service\n"),
			"target_schemas declares grafana-v2-13 and grafana-v2-14, two of kind grafana-v2",
		},
		"target schema without a version": {
			changed(t, "title: Mock Service\n", "target_schemas: [grafana-v2]\ntitle: Mock Service\n"),
			"target schema 1: 'grafana-v2' does not match pattern",
		},
		"unknown field": {
			changed(t, "        height: 300\n", "        height: 300\n        colour: red\n"),
			"row 1, panel 1: additional properties 'colour' not allowed",
		},
		"missing field": {changed(t, "timezone: utc\n", ""), "missing property 'timezone'"},
		"value outside its set": {
			changed(t, "unit: per_second", "unit: furlongs"),
			"row 1, panel 1, unit: value must be one of",
		},
		"a kind's required field": {
			changed(t, "kind: time_series", "kind: gauge"),
			"row 1, panel 1: missing property 'calculation'",
		},
		"undeclared variable": {
			changed(t, "job:${job} |", "job:${mock} |"),
			`row 1, panel 1, query 1: query names variable "mock", which the dashboard does not declare`,
		},
		"query that does not parse": {
			changed(t, "| rate(5m)", "| rate(5m"),
			"row 1, panel 1, query 1: query: a parenthesis is not closed",
		},
		"unknown choice function": {
			changed(t, "field_values(env)", "values(env)"),
			`variable env: choices: unknown choice function "values"; choices call "field_values", "free_text", "list" or "range"`,
		},
		"field for another kind": {
			changed(t, "unit: per_second", "unit: per_second\n        orientation: vertical"),
			"row 1, panel 1: orientation is for a bar panel",
		},
		"undeclared repeat": {
			changed(t, "  - title: Traffic\n", "  - title: Traffic\n    repeat_for: mock\n"),
			`row 1: repeat_for names variable "mock", which the dashboard does not declare`,
		},
		"a width past the row": {
			changed(t, "        height: 300\n", "        height: 300\n        width: 101\n"),
			"row 1, panel 1, width: maximum: got 101, want 100",
		},
		"widths past the row": {
			strings.Replace(changed(t, "        height: 300\n", "        height: 300\n        width: 60\n"),
				"    panels:\n", "    panels:\n      - {title: Mock, kind: text, height: 100, width: 60, content: mock}\n", 1),
			"row 1: the panels' widths add up to 120%, more than the row",
		},
		"thresholds out of order": {
			changed(t, "        height: 300\n",
				"        height: 300\n        thresholds:\n          - {value: 5, color: red}\n          - {value: 1, color: green}\n"),
			"row 1, panel 1, threshold 2: value 1 is not above the threshold before it",
		},
		"text panel with values": {
			changed(t, "kind: time_series", "kind: text\n        content: mock"),
			"row 1, panel 1: a text panel shows its content and takes no queries",
		},
	} {
		_, err := ParseDashboard(testCase.spec)
		if err == nil || !strings.Contains(err.Error(), testCase.says) {
			t.Errorf("%s: err = %v, want it to say %q", name, err, testCase.says)
		}
	}
}
