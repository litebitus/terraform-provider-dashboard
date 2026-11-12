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

const mockSource = `dashboard:"Mock Service"`

func reportsProblem(err error, message, location string) bool {
	if err == nil {
		return false
	}
	for _, renderedProblem := range strings.Split(err.Error(), "\n\n") {
		headline, otherLines, _ := strings.Cut(renderedProblem, "\n")
		locationLine, _, _ := strings.Cut(otherLines, "\n")
		if headline == "error: "+message && strings.TrimSpace(locationLine) == "--> "+location {
			return true
		}
	}
	return false
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

func TestAMetricsSourceReadsItsReportingFilter(t *testing.T) {
	dashboard, err := ParseDashboard(changed(t, "  kind: metrics\n", "  kind: metrics\n  reporting_filter: job:mock-exporter\n"))
	if err != nil {
		t.Fatal(err)
	}
	if dashboard.Source.ReportingFilter != "job:mock-exporter" {
		t.Errorf("source = %+v", dashboard.Source)
	}
}

func TestADocumentStatesItsMajorOrAMinor(t *testing.T) {
	for _, stated := range []string{`"1"`, `"1.0"`, `"1.1"`, `"1.2"`, `"1.3"`} {
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

// The first threshold may leave out its value, its color then starting from the lowest; no later one may.
func TestOnlyTheFirstThresholdLeavesOutItsValue(t *testing.T) {
	spec := changed(t, "        height: 300\n",
		"        height: 300\n        thresholds:\n          - {color: red}\n          - {value: 1, color: green}\n")
	dashboard, err := ParseDashboard(spec)
	if err != nil {
		t.Fatal(err)
	}
	thresholds := dashboard.Rows[0].Panels[0].Thresholds
	if thresholds[0].Value != nil || thresholds[1].Value == nil || *thresholds[1].Value != 1 {
		t.Errorf("thresholds = %+v", thresholds)
	}
	refused := changed(t, "        height: 300\n",
		"        height: 300\n        thresholds:\n          - {value: 1, color: green}\n          - {color: red}\n")
	if _, err := ParseDashboard(refused); !reportsProblem(err, "`value` is required", mockSource+":23:13") {
		t.Errorf("a later threshold without a value: err = %v, want it refused at the second threshold", err)
	}
}

// A panel colors its values from one source: its thresholds, or its value mappings' colors.
func TestAPanelColorsItsValuesFromOneSource(t *testing.T) {
	thresholds := "        thresholds:\n          - {color: red}\n          - {value: 1, color: green}\n"
	for name, testCase := range map[string]struct {
		written string
		refused bool
	}{
		"thresholds and a colored mapping": {
			"        value_mappings:\n          \"0\": { text: mock-down, color: red }\n" + thresholds, true,
		},
		"colored mappings alone": {
			"        value_mappings:\n          \"0\": { text: mock-down, color: red }\n", false,
		},
		"thresholds and a mapping without color": {
			"        value_mappings:\n          \"0\": { text: mock-down }\n" + thresholds, false,
		},
	} {
		_, err := ParseDashboard(changed(t, "        height: 300\n", "        height: 300\n"+testCase.written))
		refusal := "`0` matches a form that is not allowed"
		if testCase.refused && !reportsProblem(err, refusal, mockSource+":22:16") {
			t.Errorf("%s: err = %v, want %q", name, err, refusal)
		}
		if !testCase.refused && err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestEveryExampleDashboardParses(t *testing.T) {
	examples, err := filepath.Glob("../examples/dashboards/*.yml")
	if err != nil || len(examples) == 0 {
		t.Fatalf("examples/dashboards holds no dashboard: %v", err)
	}
	for _, example := range examples {
		if strings.HasSuffix(example, ".invalid.yml") {
			continue
		}
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

// Every problem is reported at its position in the document: a value's at the value, a field not allowed at its key, a block's at the block.
func TestAProblemIsReportedAtItsPosition(t *testing.T) {
	for name, testCase := range map[string]struct {
		spec     string
		message  string
		location string
	}{
		"empty document": {"", "the document is empty", "dashboard:{title}:1:1"},
		"unknown version": {
			changed(t, `schema_version: "1"`, `schema_version: "2"`),
			"`schema_version` \"2\" is not a version this provider reads; it reads [\"1\" \"1.0\" \"1.1\" \"1.2\" \"1.3\"]",
			mockSource + ":1:17",
		},
		"unquoted version": {
			changed(t, `schema_version: "1"`, `schema_version: 1`),
			"`schema_version` is required, quoted, one of `\"1\"`, `\"1.0\"`, `\"1.1\"`, `\"1.2\"` or `\"1.3\"`",
			mockSource + ":1:17",
		},
		"two target schemas of one kind": {
			changed(t, "title: Mock Service\n", "target_schemas: [grafana-v2-13, grafana-v2-14]\ntitle: Mock Service\n"),
			"`target_schemas` declares `grafana-v2-13` and `grafana-v2-14`, two of kind `grafana-v2`; " +
				"a document renders for one version of a kind",
			mockSource + ":2:17",
		},
		"target schema without a version": {
			changed(t, "title: Mock Service\n", "target_schemas: [grafana-v2]\ntitle: Mock Service\n"),
			"`target_schemas[0]` must match pattern `^[a-z][a-z0-9]*(-[a-z][a-z0-9]*)*-[0-9]+(\\.[0-9]+)?$`",
			mockSource + ":2:18",
		},
		"unknown field": {
			changed(t, "        height: 300\n", "        height: 300\n        colour: red\n"),
			"`colour` is not defined in the schema", mockSource + ":21:9",
		},
		"missing field": {changed(t, "timezone: utc\n", ""), "`timezone` is required", mockSource + ":1:1"},
		"value outside its set": {
			changed(t, "unit: per_second", "unit: furlongs"),
			"`unit` must be one of `\"bytes\"`, `\"count\"`, `\"percent\"`, `\"per_second\"`, `\"seconds\"`",
			mockSource + ":21:15",
		},
		"a kind's required field": {
			changed(t, "kind: time_series", "kind: gauge"), "`calculation` is required", mockSource + ":18:9",
		},
		"undeclared variable": {
			changed(t, "job:${job} |", "job:${mock} |"),
			"`query` names variable `mock`, which the dashboard does not declare", mockSource + ":23:20",
		},
		"query that does not parse": {
			changed(t, "| rate(5m)", "| rate(5m"), "`query`: a parenthesis is not closed", mockSource + ":23:20",
		},
		"unknown choice function": {
			changed(t, "field_values(env)", "values(env)"),
			"`choices`: unknown choice function `values`; choices call `field_values`, `free_text`, `list` or `range`",
			mockSource + ":10:14",
		},
		"undeclared variable in choices": {
			changed(t, "and env:${env}\")", "and env:${mock}\")"),
			"`query` names variable `mock`, which the dashboard does not declare", mockSource + ":14:14",
		},
		"field for another kind": {
			changed(t, "unit: per_second", "unit: per_second\n        orientation: vertical"),
			"`orientation` is for a `bar` panel", mockSource + ":22:9",
		},
		"value column header for another kind": {
			changed(t, "legend_format: \"{{job}}\"\n", "legend_format: \"{{job}}\"\n            value_column_header: mock\n"),
			"`value_column_header` is for a `table` panel", mockSource + ":25:13",
		},
		"value format for another kind": {
			changed(t, "legend_format: \"{{job}}\"\n", "legend_format: \"{{job}}\"\n            value_format: \"%{%Y}C\"\n"),
			"`value_format` is for a `table` panel", mockSource + ":25:13",
		},
		"value format the format function does not read": {
			strings.Replace(
				changed(t, "legend_format: \"{{job}}\"\n", "legend_format: \"{{job}}\"\n            value_format: \"%k\"\n"),
				"kind: time_series", "kind: table", 1),
			"`value_format`: `%k` is not a verb `format` writes", mockSource + ":25:27",
		},
		"value format with a verb per value but one value": {
			strings.Replace(
				changed(t, "legend_format: \"{{job}}\"\n", "legend_format: \"{{job}}\"\n            value_format: \"%{%Y}C %{%m}C\"\n"),
				"kind: time_series", "kind: table", 1),
			"`value_format` holds one verb, for the query's value, not 2", mockSource + ":25:27",
		},
		"metrics source with an index pattern": {
			changed(t, "  kind: metrics\n", "  kind: metrics\n  index_pattern: mock-*\n"),
			"`source`: a `metrics` source takes no `index_pattern` or `time_field`", mockSource + ":8:3",
		},
		"logs source with a reporting filter": {
			changed(t, "  kind: metrics\n", "  kind: logs\n  index_pattern: mock-*\n  time_field: mock_time\n"+
				"  reporting_filter: job:mock-exporter\n"),
			"`source`: a `logs` source takes no `reporting_filter`", mockSource + ":10:3",
		},
		"reporting filter with a stage": {
			changed(t, "  kind: metrics\n", "  kind: metrics\n  reporting_filter: job:mock-exporter | count()\n"),
			"`reporting_filter` is a DQL filter, with no stages or arithmetic", mockSource + ":8:21",
		},
		"undeclared variable in a reporting filter": {
			changed(t, "  kind: metrics\n", "  kind: metrics\n  reporting_filter: job:${mock}\n"),
			"`reporting_filter` names variable `mock`, which the dashboard does not declare", mockSource + ":8:21",
		},
		"undeclared repeat": {
			changed(t, "  - title: Traffic\n", "  - title: Traffic\n    repeat_for: mock\n"),
			"`repeat_for` names variable `mock`, which the dashboard does not declare", mockSource + ":17:17",
		},
		"a width past the row": {
			changed(t, "        height: 300\n", "        height: 300\n        width: 101\n"),
			"`width` must be <= `100`", mockSource + ":21:16",
		},
		"min not below max": {
			changed(t, "        height: 300\n", "        height: 300\n        min: 5\n        max: 1\n"),
			"`min` `5` is not below `max` `1`", mockSource + ":21:14",
		},
		"widths past the row": {
			strings.Replace(changed(t, "        height: 300\n", "        height: 300\n        width: 60\n"),
				"    panels:\n", "    panels:\n      - {title: Mock, kind: text, height: 100, width: 60, content: mock}\n", 1),
			"the panels' widths add up to 120%, more than the row", mockSource + ":16:5",
		},
		"thresholds out of order": {
			changed(t, "        height: 300\n",
				"        height: 300\n        thresholds:\n          - {value: 5, color: red}\n          - {value: 1, color: green}\n"),
			"`value` `1` is not above the threshold before it", mockSource + ":23:21",
		},
		"text panel with values": {
			changed(t, "kind: time_series", "kind: text\n        content: mock"),
			"a `text` panel shows its `content` and takes no `queries`", mockSource + ":23:9",
		},
	} {
		_, err := ParseDashboard(testCase.spec)
		if !reportsProblem(err, testCase.message, testCase.location) {
			t.Errorf("%s: err = %v, want %q at %s", name, err, testCase.message, testCase.location)
		}
	}
}

// A problem the provider finds at a value is underlined there, with the lines around it.
func TestAProblemAtAValueIsUnderlinedAtTheValue(t *testing.T) {
	_, err := ParseDashboard(changed(t, "  - title: Traffic\n", "  - title: Traffic\n    repeat_for: mock\n"))
	want := "error: `repeat_for` names variable `mock`, which the dashboard does not declare\n" +
		"  --> dashboard:\"Mock Service\":17:17\n" +
		"   |\n" +
		"15 | rows:\n" +
		"16 |   - title: Traffic\n" +
		"17 |     repeat_for: mock\n" +
		"   |                 ^^^^\n" +
		"18 |     panels:\n" +
		"19 |       - title: Requests by job\n" +
		"   |\n" +
		"\n" +
		"1 error"
	if err == nil || err.Error() != want {
		t.Errorf("err = %v, want\n%s", err, want)
	}
}

// A field a panel does not take is underlined at its key.
func TestAFieldNotAllowedIsUnderlinedAtItsKey(t *testing.T) {
	_, err := ParseDashboard(changed(t, "unit: per_second", "unit: per_second\n        orientation: vertical"))
	want := "error: `orientation` is for a `bar` panel\n" +
		"  --> dashboard:\"Mock Service\":22:9\n" +
		"   |\n" +
		"15 | rows:\n" +
		"16 |   - title: Traffic\n" +
		"17 |     panels:\n" +
		"18 |       - title: Requests by job\n" +
		"...\n" +
		"20 |         height: 300\n" +
		"21 |         unit: per_second\n" +
		"22 |         orientation: vertical\n" +
		"   |         ^^^^^^^^^^^\n" +
		"23 |         queries:\n" +
		"24 |           - query: __name__:mock_requests_total and env:${env} and job:${job} | rate(5m) | sum() by job\n" +
		"   |\n" +
		"\n" +
		"1 error"
	if err == nil || err.Error() != want {
		t.Errorf("err = %v, want\n%s", err, want)
	}
}

// A problem with a whole block is underlined at the block.
func TestAProblemWithABlockIsUnderlinedAtTheBlock(t *testing.T) {
	spec := changed(t, "title: Mock Service\n", "title: Mock Service\ntarget_schemas: [grafana-v2-13, grafana-v2-14]\n")
	_, err := ParseDashboard(spec)
	want := "error: `target_schemas` declares `grafana-v2-13` and `grafana-v2-14`, two of kind `grafana-v2`; " +
		"a document renders for one version of a kind\n" +
		" --> dashboard:\"Mock Service\":3:17\n" +
		"  |\n" +
		"1 | schema_version: \"1\"\n" +
		"2 | title: Mock Service\n" +
		"3 | target_schemas: [grafana-v2-13, grafana-v2-14]\n" +
		"  |                 ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^\n" +
		"4 | time_range: 6h\n" +
		"5 | refresh_interval: 1m\n" +
		"  |\n" +
		"\n" +
		"1 error"
	if err == nil || err.Error() != want {
		t.Errorf("err = %v, want\n%s", err, want)
	}
}

// A document that is not YAML is reported where the YAML reader stops, under the name of a document with no title.
func TestAYAMLProblemIsReportedWhereTheReaderStops(t *testing.T) {
	_, err := ParseDashboard(changed(t, "title: Mock Service", "title: [Mock"))
	want := "error: while parsing a flow sequence, did not find expected ',' or ']'\n" +
		" --> dashboard:{title}:3:11\n" +
		"  |\n" +
		"1 | schema_version: \"1\"\n" +
		"2 | title: [Mock\n" +
		"3 | time_range: 6h\n" +
		"  |           ^\n" +
		"4 | refresh_interval: 1m\n" +
		"5 | timezone: utc\n" +
		"  |\n" +
		"\n" +
		"1 error"
	if err == nil || err.Error() != want {
		t.Errorf("err = %v, want\n%s", err, want)
	}
}

// A report names the document by its title written as a JSON string, and a document with no title as the dashboard alone.
func TestAReportNamesTheDocumentByItsTitle(t *testing.T) {
	widthPastTheRow := "        height: 300\n        width: 101\n"
	quotedTitle := strings.Replace(changed(t, "        height: 300\n", widthPastTheRow),
		"title: Mock Service", `title: 'Mock "Service"'`, 1)
	_, err := ParseDashboard(quotedTitle)
	if !reportsProblem(err, "`width` must be <= `100`", `dashboard:"Mock \"Service\"":21:16`) {
		t.Errorf("a title holding a quote: err = %v", err)
	}
	_, err = ParseDashboard(changed(t, "title: Mock Service\n", ""))
	if !reportsProblem(err, "`title` is required", "dashboard:{title}:1:1") {
		t.Errorf("no title: err = %v", err)
	}
}

// Problems are reported in document order, each apart from the next by a blank line, then counted.
func TestProblemsAreReportedInDocumentOrderThenCounted(t *testing.T) {
	_, err := ParseDashboard(changed(t, "kind: time_series", "kind: text\n        content: mock"))
	if err == nil {
		t.Fatal("a text panel with values parsed")
	}
	renderedProblems := strings.Split(err.Error(), "\n\n")
	if len(renderedProblems) != 3 || renderedProblems[2] != "2 errors" ||
		!strings.HasPrefix(renderedProblems[0], "error: a `text` panel shows its `content` and takes no `unit`\n") ||
		!strings.HasPrefix(renderedProblems[1], "error: a `text` panel shows its `content` and takes no `queries`\n") {
		t.Errorf("err = %v, want the `unit` problem, the `queries` problem, then `2 errors`", err)
	}
	if _, err := ParseDashboard(""); err == nil || !strings.HasSuffix(err.Error(), "\n\n1 error") {
		t.Errorf("one problem: err = %v, want it counted as `1 error`", err)
	}
}
