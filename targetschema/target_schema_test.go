package targetschema

import (
	"strings"
	"testing"
)

func TestAnIdentifierIsItsKindThenItsVersion(t *testing.T) {
	for text, want := range map[string]ID{
		"grafana-v2-13":   {Kind: "grafana-v2", Version: "13"},
		"grafana-v2-13.3": {Kind: "grafana-v2", Version: "13.3"},
		"mock-ui-1":       {Kind: "mock-ui", Version: "1"},
	} {
		if got, err := ParseID(text); err != nil || got != want || got.String() != text {
			t.Errorf("%s: %+v (%v), want %+v", text, got, err, want)
		}
	}
	for _, text := range []string{"grafana", "grafana-v2", "Grafana-v2-13", "grafana-v2-13.3.1", "-13"} {
		if _, err := ParseID(text); err == nil {
			t.Errorf("%s reads as an identifier", text)
		}
	}
}

// A filter matches by whole leading words, never part of a word, and a major it states matches that major's minors.
func TestAFilterMatchesByWholeLeadingWords(t *testing.T) {
	id := ID{Kind: "grafana-v2", Version: "13.3"}
	for filter, want := range map[string]bool{
		"grafana":         true,
		"grafana-v2":      true,
		"grafana-v2-13":   true,
		"grafana-v2-13.3": true,
		"graf":            false,
		"grafana-v1":      false,
		"grafana-v2-14":   false,
		"grafana-v2-13.2": false,
		"perses":          false,
	} {
		parsed, err := ParseFilter(filter)
		if err != nil {
			t.Fatalf("%s: %v", filter, err)
		}
		if got := parsed.Matches(id); got != want {
			t.Errorf("%s matches %s: %v, want %v", filter, id, got, want)
		}
	}
}

func TestAMajorResolvesToItsNewestCarriedMinor(t *testing.T) {
	carried := []ID{{"grafana-v2", "13.2"}, {"grafana-v2", "13.10"}, {"grafana-v2", "14.0"}}
	for stated, want := range map[ID]string{
		{Kind: "grafana-v2", Version: "13"}:   "grafana-v2-13.10",
		{Kind: "grafana-v2", Version: "13.2"}: "grafana-v2-13.2",
		{Kind: "grafana-v2", Version: "14"}:   "grafana-v2-14.0",
	} {
		if got, err := Resolve(stated, carried); err != nil || got.String() != want {
			t.Errorf("%s resolves to %s (%v), want %s", stated, got, err, want)
		}
	}
	_, err := Resolve(ID{Kind: "grafana-v2", Version: "12"}, carried)
	if err == nil || !strings.Contains(err.Error(), "it renders grafana-v2-13.10, grafana-v2-13.2, grafana-v2-14.0") {
		t.Errorf("err = %v", err)
	}
}

var grafanaDefault = map[string]string{"grafana": "grafana-v2-14"}

// The call's target_schema outranks the document's target_schemas, which outrank the defaults; the defaults decide only among several candidates, or for a document that declares none.
func TestSelectionFollowsItsPriority(t *testing.T) {
	for name, testCase := range map[string]struct {
		request Request
		want    string
	}{
		"one declared, no call":               {Request{Declared: []string{"grafana-v2-13"}, Defaults: grafanaDefault}, "grafana-v2-13"},
		"two declared, default picks":         {Request{Declared: []string{"grafana-v2-13", "grafana-v2-14"}, Defaults: grafanaDefault}, "grafana-v2-14"},
		"call outranks default":               {Request{TargetSchema: "grafana-v2-13", Declared: []string{"grafana-v2-13", "grafana-v2-14"}, Defaults: grafanaDefault}, "grafana-v2-13"},
		"call filters, default picks":         {Request{TargetSchema: "grafana", Declared: []string{"grafana-v2-13", "grafana-v2-14", "perses-mock-1"}, Defaults: grafanaDefault}, "grafana-v2-14"},
		"call picks the other UI":             {Request{TargetSchema: "perses", Declared: []string{"grafana-v2-13", "perses-mock-1"}}, "perses-mock-1"},
		"none declared, whole call":           {Request{TargetSchema: "grafana-v2-13", Defaults: grafanaDefault}, "grafana-v2-13"},
		"none declared, filter picks default": {Request{TargetSchema: "grafana", Defaults: grafanaDefault}, "grafana-v2-14"},
		"none declared, the only default":     {Request{Defaults: grafanaDefault}, "grafana-v2-14"},
		"most specific default":               {Request{Declared: []string{"grafana-v2-13", "grafana-v2-14"}, Defaults: map[string]string{"grafana": "grafana-v2-13", "grafana-v2": "grafana-v2-14"}}, "grafana-v2-14"},
	} {
		got, err := Select(testCase.request)
		if err != nil || got.String() != testCase.want {
			t.Errorf("%s: %s (%v), want %s", name, got, err, testCase.want)
		}
	}
}

func TestASelectionThatLeavesNoneOrSeveralIsRefused(t *testing.T) {
	for name, testCase := range map[string]struct {
		request Request
		says    string
	}{
		"two declared, no defaults": {
			Request{Declared: []string{"grafana-v2-13", "grafana-v2-14"}},
			"the document's target schemas leave grafana-v2-13, grafana-v2-14; name one with target_schema",
		},
		"two UIs, a default for one": {
			Request{Declared: []string{"grafana-v2-13", "grafana-v2-14", "perses-mock-1"}, Defaults: grafanaDefault},
			"leave grafana-v2-14, perses-mock-1",
		},
		"call names one the document does not declare": {
			Request{TargetSchema: "grafana-v2-13", Declared: []string{"grafana-v2-14"}, Defaults: grafanaDefault},
			"target_schema grafana-v2-13 matches none of the document's target schemas, grafana-v2-14",
		},
		"part of a word": {
			Request{TargetSchema: "graf", Declared: []string{"grafana-v2-13"}},
			"target_schema graf matches none",
		},
		"none declared, a filter and no default": {
			Request{TargetSchema: "grafana"},
			"target_schema grafana is not a whole identifier and leaves no single default",
		},
		"none declared, nothing named": {
			Request{},
			"the document declares no target_schemas; name a whole identifier with target_schema",
		},
		"default picking outside its key": {
			Request{Declared: []string{"grafana-v2-13"}, Defaults: map[string]string{"perses": "grafana-v2-14"}},
			"default_target_schemas[perses] picks grafana-v2-14, which perses does not match",
		},
	} {
		_, err := Select(testCase.request)
		if err == nil || !strings.Contains(err.Error(), testCase.says) {
			t.Errorf("%s: err = %v, want it to say %q", name, err, testCase.says)
		}
	}
}
