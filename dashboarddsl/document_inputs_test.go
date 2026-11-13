package dashboarddsl

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const mockInputsDashboard = `schema_version: "1"
id: mock-${inputs.service}
title: Mock ${inputs.service}
time_range: 6h
refresh_interval: 1m
timezone: utc
source:
  kind: metrics
inputs:
  env:
    type: string
  service:
    type: string
    default: mock-service
  height:
    type: number
    default: 300
  regions:
    type: list
    default: [mock-region-1]
  owner:
    type: map
    default: { team: mock-team }
filter: env:${inputs.env}
variables:
  region:
    choices: list("mock-region-1", "mock-region-2")
    default: ${inputs.env}
rows:
  - title: Traffic of ${inputs.owner.team}
    panels:
      - title: Requests
        kind: time_series
        height: ${inputs.height}
        unit: per_second
        queries:
          - query: __name__:mock_requests_total and env:${inputs.env} and region:${region}
`

// inputsDocument is the mock inputs dashboard with one piece of its text replaced.
func inputsDocument(t *testing.T, oldText, replacementText string) string {
	t.Helper()
	if !strings.Contains(mockInputsDashboard, oldText) {
		t.Fatalf("the mock inputs dashboard holds no %q", oldText)
	}
	return strings.Replace(mockInputsDashboard, oldText, replacementText, 1)
}

// A placeholder that is a whole value takes the input as it is, and one inside
// text its scalar text; a dashboard variable is left for the UI.
func TestAnInputIsSubstitutedWholeOrInsideText(t *testing.T) {
	dashboard, err := ParseDashboardWithInputs(mockInputsDashboard, InputValues{"env": "mock-env"})
	if err != nil {
		t.Fatal(err)
	}
	panel := dashboard.Rows[0].Panels[0]
	checks := map[string]bool{
		"id from a default":             dashboard.ID == "mock-mock-service",
		"title inside text":             dashboard.Title == "Mock mock-service",
		"filter inside text":            dashboard.Filter == "env:mock-env",
		"variable default whole":        dashboard.Variables["region"].Default == "mock-env",
		"number default whole":          panel.Height == 300,
		"a map input's key":             dashboard.Rows[0].Title == "Traffic of mock-team",
		"dashboard variable left as is": strings.HasSuffix(panel.Queries[0].Query, "region:${region}"),
	}
	for check, holds := range checks {
		if !holds {
			t.Errorf("%s does not hold: %+v", check, dashboard)
		}
	}
}

// A passed value wins over the input's default, whatever kind of value it is.
func TestAPassedValueWinsOverTheDefault(t *testing.T) {
	dashboard, err := ParseDashboardWithInputs(mockInputsDashboard, InputValues{
		"env": "mock-env", "service": "mock-other", "height": json.Number("200"),
		"owner": map[string]any{"team": "mock-other-team"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if dashboard.Title != "Mock mock-other" || dashboard.Rows[0].Panels[0].Height != 200 ||
		dashboard.Rows[0].Title != "Traffic of mock-other-team" {
		t.Errorf("the passed values are not read: %+v", dashboard)
	}
}

// An escaped placeholder is written as text, its escape prefix dropped.
func TestAnEscapedPlaceholderIsWrittenAsText(t *testing.T) {
	document := inputsDocument(t, "title: Mock ${inputs.service}", "title: Mock $${inputs.service}")
	dashboard, err := ParseDashboardWithInputs(document, InputValues{"env": "mock-env"})
	if err != nil {
		t.Fatal(err)
	}
	if dashboard.Title != "Mock ${inputs.service}" {
		t.Errorf("title = %q", dashboard.Title)
	}
}

// A value passed for an input the document does not declare, or of another kind
// than declared, is a problem with what was passed.
func TestAPassedValueTheDocumentDoesNotTakeIsAnInputsProblem(t *testing.T) {
	for name, testCase := range map[string]struct {
		passedValues InputValues
		wantMessage  string
	}{
		"undeclared": {InputValues{"env": "mock-env", "mock": "x"}, "the document declares no input `mock`"},
		"other kind": {
			InputValues{"env": json.Number("1")},
			"input `env` is declared `string`, and is passed a number",
		},
	} {
		_, err := ParseDashboardWithInputs(mockInputsDashboard, testCase.passedValues)
		var inputsProblem InputsProblem
		if !errors.As(err, &inputsProblem) || err.Error() != testCase.wantMessage {
			t.Errorf("%s: err = %v, want the inputs problem %q", name, err, testCase.wantMessage)
		}
	}
}

// A problem with how the document reads its inputs is reported at its position.
func TestAnInputProblemIsReportedAtItsPosition(t *testing.T) {
	for name, testCase := range map[string]struct {
		documentText string
		wantMessage  string
		wantLocation string
	}{
		"required and not passed": {
			mockInputsDashboard,
			"input `env` is required, and the render is passed none",
			mockInputsSource + ":10:3",
		},
		"undeclared reference": {
			inputsDocument(t, "filter: env:${inputs.env}", "filter: env:${inputs.mock}"),
			"`${inputs.mock}` reads input `mock`, which `inputs` does not declare",
			mockInputsSource + ":24:9",
		},
		"a list inside text": {
			inputsDocument(t, "filter: env:${inputs.env}", "filter: env:${inputs.regions}"),
			"`${inputs.regions}` is a list inside text, which takes a scalar",
			mockInputsSource + ":24:9",
		},
		"a key the map lacks": {
			inputsDocument(t, "${inputs.owner.team}", "${inputs.owner.mock}"),
			"`${inputs.owner.mock}` reads key `mock`, which the value does not hold",
			mockInputsSource + ":30:12",
		},
		"a key of a scalar": {
			inputsDocument(t, "${inputs.owner.team}", "${inputs.env.mock}"),
			"`${inputs.env.mock}` reads key `mock` of a string, which holds no keys",
			mockInputsSource + ":30:12",
		},
		"a default of another kind": {
			inputsDocument(t, "    default: 300", "    default: mock"),
			"input `height` is declared `number`, and its `default` is a string",
			mockInputsSource + ":17:14",
		},
	} {
		passedValues := InputValues{"env": "mock-env"}
		if name == "required and not passed" {
			passedValues = nil
		}
		_, err := ParseDashboardWithInputs(testCase.documentText, passedValues)
		if !reportsProblem(err, testCase.wantMessage, testCase.wantLocation) {
			t.Errorf("%s: err = %v, want %q at %s", name, err, testCase.wantMessage, testCase.wantLocation)
		}
	}
}

// A value an input brings in is held to the schema like a written one, reported
// where its placeholder stands.
func TestASubstitutedValueIsHeldToTheSchema(t *testing.T) {
	_, err := ParseDashboardWithInputs(mockInputsDashboard, InputValues{
		"env": "mock-env", "height": json.Number("0"),
	})
	if !reportsProblem(err, "`height` must be >= `1`", mockInputsSource+":34:17") {
		t.Errorf("err = %v", err)
	}
}

// A document without inputs reads as it always has.
func TestADocumentWithoutInputsReadsAsWritten(t *testing.T) {
	written, err := ParseDashboard(mockDashboard)
	if err != nil {
		t.Fatal(err)
	}
	passedNone, err := ParseDashboardWithInputs(mockDashboard, nil)
	if err != nil {
		t.Fatal(err)
	}
	if written.Title != passedNone.Title || len(written.Variables) != len(passedNone.Variables) {
		t.Errorf("passing no inputs changes the read: %+v vs %+v", written, passedNone)
	}
}

const mockInputsSource = `dashboard:"Mock ${inputs.service}"`
