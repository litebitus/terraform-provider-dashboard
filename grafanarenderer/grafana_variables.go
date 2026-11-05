package grafanarenderer

import (
	"strings"

	"github.com/litebitus/terraform-provider-dashboard/dashboarddsl"
)

type variableOption struct {
	Selected bool `json:"selected"`
	Text     any  `json:"text"`
	Value    any  `json:"value"`
}

type queryVariableSpec struct {
	Name             string           `json:"name"`
	Label            string           `json:"label"`
	Current          variableOption   `json:"current"`
	Hide             string           `json:"hide"`
	Refresh          string           `json:"refresh"`
	SkipURLSync      bool             `json:"skipUrlSync"`
	Query            dataQuery        `json:"query"`
	Regex            string           `json:"regex"`
	Sort             string           `json:"sort"`
	Definition       string           `json:"definition"`
	Options          []variableOption `json:"options"`
	Multi            bool             `json:"multi"`
	IncludeAll       bool             `json:"includeAll"`
	AllowCustomValue bool             `json:"allowCustomValue"`
}

type customVariableSpec struct {
	Name             string           `json:"name"`
	Label            string           `json:"label"`
	Query            string           `json:"query"`
	Current          variableOption   `json:"current"`
	Options          []variableOption `json:"options"`
	Multi            bool             `json:"multi"`
	IncludeAll       bool             `json:"includeAll"`
	Hide             string           `json:"hide"`
	SkipURLSync      bool             `json:"skipUrlSync"`
	AllowCustomValue bool             `json:"allowCustomValue"`
}

type textVariableSpec struct {
	Name        string         `json:"name"`
	Label       string         `json:"label"`
	Current     variableOption `json:"current"`
	Query       string         `json:"query"`
	Hide        string         `json:"hide"`
	SkipURLSync bool           `json:"skipUrlSync"`
}

// variable renders one variable as the Grafana variable its choices call for: a field's values read on every dashboard load and offered in alphabetical order, a list as written, or free text.
func (render *dashboardRender) variable(name string, variable dashboarddsl.Variable) kindSpec {
	current := currentChoiceOf(variable)
	choices, err := variable.ChoiceCall()
	if err != nil {
		render.refuse(variablePlace(name), err.Error())
		return kindSpec{Kind: "TextVariable", Spec: textVariableSpec{Name: name, Label: name, Current: current, Hide: "dontHide"}}
	}
	switch choices.Function {
	case dashboarddsl.FieldValuesChoices:
		definition := "label_values(" + choices.Field + ")"
		if choices.Query != "" {
			selector, err := render.queries.seriesSelector(choices.Query)
			if err != nil {
				render.refuse(variablePlace(name), err.Error())
			}
			definition = "label_values(" + selector + ", " + choices.Field + ")"
		}
		return kindSpec{Kind: "QueryVariable", Spec: queryVariableSpec{
			Name: name, Label: name, Current: current, Hide: "dontHide", Refresh: "onDashboardLoad",
			Query:      prometheusQuery(map[string]any{"query": definition, "refId": "PrometheusVariableQueryEditor-VariableQuery"}),
			Sort:       "alphabeticalAsc",
			Definition: definition,
			Options:    []variableOption{},
			Multi:      variable.Multiple,
			IncludeAll: variable.IncludeAll,
		}}
	case dashboarddsl.ListChoices:
		escaped := make([]string, len(choices.Values))
		options := make([]variableOption, len(choices.Values))
		for index, value := range choices.Values {
			escaped[index] = strings.ReplaceAll(value, ",", `\,`)
			options[index] = variableOption{Selected: value == variable.Default, Text: value, Value: value}
		}
		return kindSpec{Kind: "CustomVariable", Spec: customVariableSpec{
			Name: name, Label: name, Query: strings.Join(escaped, ","), Current: current, Options: options,
			Multi: variable.Multiple, IncludeAll: variable.IncludeAll, Hide: "dontHide",
		}}
	case dashboarddsl.RangeChoices:
		render.refuse(variablePlace(name), "Grafana has no range variable")
	}
	return kindSpec{Kind: "TextVariable", Spec: textVariableSpec{
		Name: name, Label: name, Current: current, Query: variable.Default, Hide: "dontHide",
	}}
}

// currentChoiceOf is the value a variable opens with, as Grafana holds it: a list when several may be chosen, and empty with no default.
func currentChoiceOf(variable dashboarddsl.Variable) variableOption {
	if variable.Multiple && variable.Default != "" {
		return variableOption{Selected: true, Text: []string{variable.Default}, Value: []string{variable.Default}}
	}
	return variableOption{Selected: variable.Default != "", Text: variable.Default, Value: variable.Default}
}
