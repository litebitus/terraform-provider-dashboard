package dashboarddsl

import (
	"strings"
	"testing"
)

// A schema problem is reported as a compiler reports one: the message, the document's title and position, the lines around it, and the rule it breaks.
func TestASchemaProblemIsReportedAsACompilerReportsOne(t *testing.T) {
	_, err := ParseDashboard(changed(t, "        height: 300\n", "        height: 300\n        width: 0\n"))
	want := "error: `width` must be >= `1`\n" +
		"  --> dashboard:\"Mock Service\":21:16\n" +
		"   |\n" +
		"15 | rows:\n" +
		"16 |   - title: Traffic\n" +
		"17 |     panels:\n" +
		"18 |       - title: Requests by job\n" +
		"19 |         kind: time_series\n" +
		"20 |         height: 300\n" +
		"21 |         width: 0\n" +
		"   |                ^ breaks `minimum`\n" +
		"22 |         unit: per_second\n" +
		"23 |         queries:\n" +
		"   |\n" +
		"   = rule: minimum, at dashboard-dsl-v1.2.schema.json#/$defs/panel/properties/width/minimum\n" +
		"\n" +
		"1 error"
	if err == nil || err.Error() != want {
		t.Errorf("err = %v, want\n%s", err, want)
	}
}

// A schema failure is worded by the YAML DSL standard and located at the value it is about, a missing field at its map.
func TestASchemaFailureIsWordedByTheStandard(t *testing.T) {
	for name, testCase := range map[string]struct {
		old, replacement string
		message          string
		location         string
	}{
		"maximum": {
			"        height: 300\n", "        height: 300\n        width: 101\n", "`width` must be <= `100`", "21:16",
		},
		"minimum": {"height: 300", "height: 0", "`height` must be >= `1`", "20:17"},
		"missing field": {
			"        height: 300\n",
			"        height: 300\n        thresholds:\n          - {value: 1, color: green}\n          - {color: red}\n",
			"`value` is required", "23:13",
		},
		"missing field at the root": {"timezone: utc\n", "", "`timezone` is required", "1:1"},
		"unknown field": {
			"        height: 300\n", "        height: 300\n        mock_field: mock\n",
			"`mock_field` is not a field of `panels[0]`", "21:9",
		},
		"invalid key": {"  job:\n", "  mock-key:\n", "`mock-key` is not a valid key of `variables`", "12:3"},
		"enum": {
			"unit: per_second", "unit: mock",
			"`unit` must be one of `\"bytes\"`, `\"count\"`, `\"percent\"`, `\"per_second\"`, `\"seconds\"`", "21:15",
		},
		"type": {
			"        height: 300\n", "        height: 300\n        stacked: mock\n", "`stacked` must be `boolean`", "21:18",
		},
		"pattern": {
			"time_range: 6h", "time_range: mock", "`time_range` must match pattern `^([0-9]+(ms|s|m|h|d|w|y))+$`", "3:13",
		},
		"uniqueItems": {
			"title: Mock Service\n", "target_schemas: [mock-v1-1, mock-v2-1, mock-v1-1, mock-v2-1]\ntitle: Mock Service\n",
			"`target_schemas` must NOT have duplicate items (items ## 3 and 1 are identical)", "2:17",
		},
		"oneOf, no form holds": {
			"        height: 300\n", "        height: 300\n        value_mappings:\n          \"0\": 5\n",
			"`0` matches none of its allowed forms", "22:16",
		},
	} {
		_, err := ParseDashboard(changed(t, testCase.old, testCase.replacement))
		if !reportsProblem(err, testCase.message, mockSource+":"+testCase.location) {
			t.Errorf("%s: err = %v, want %q at %s", name, err, testCase.message, testCase.location)
		}
	}
}

// A failed not carries the description of the schema it rules out as its schema note.
func TestAFailedNotCarriesTheDescriptionOfWhatItRulesOut(t *testing.T) {
	_, err := ParseDashboard(changed(t, "        height: 300\n", "        height: 300\n        value_mappings:\n"+
		"          \"0\": { text: mock, color: red }\n        thresholds:\n          - {color: red}\n"))
	note := "   = [schema] In a panel with `thresholds`, a value mapping takes no `color`: the thresholds color the panel's values.\n"
	if !reportsProblem(err, "`0` matches a form that is not allowed", mockSource+":22:16") || !strings.Contains(err.Error(), note) {
		t.Errorf("err = %v, want the schema note %q", err, note)
	}
}
