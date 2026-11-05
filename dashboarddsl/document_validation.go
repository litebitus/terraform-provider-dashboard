package dashboarddsl

import (
	"errors"
	"maps"
	"slices"

	"github.com/litebitus/dql-compiler/dqlsyntax"

	"github.com/litebitus/terraform-provider-dashboard/targetschema"
)

// documentCheck holds a document the schema admits to the rules a schema cannot state, collecting every problem so one plan names them all.
type documentCheck struct {
	dashboard Dashboard
	problems  []error
}

func (check *documentCheck) add(at place, format string, arguments ...any) {
	check.problems = append(check.problems, at.problem(format, arguments...))
}

func validateDashboard(dashboard Dashboard) error {
	check := &documentCheck{dashboard: dashboard}
	var at place
	check.targetSchemas(at, dashboard.TargetSchemas)
	check.source(at.with("source"), dashboard.Source)
	if dashboard.Filter != "" {
		check.filter(at, dashboard.Filter)
	}
	check.links(at, dashboard.Links)
	for position, annotation := range dashboard.Annotations {
		annotationAt := at.item("annotations", position)
		check.query(annotationAt, annotation.Query)
		check.source(annotationAt.with("source"), annotation.Source)
	}
	for _, name := range slices.Sorted(maps.Keys(dashboard.Variables)) {
		check.variable(at.entry("variables", name), dashboard.Variables[name])
	}
	for position, row := range dashboard.Rows {
		check.row(at.item("rows", position), row)
	}
	return errors.Join(check.problems...)
}

// targetSchemas holds the document to at most one target schema per kind.
func (check *documentCheck) targetSchemas(at place, declared []string) {
	kinds := map[string]string{}
	for _, text := range declared {
		id, err := targetschema.ParseID(text)
		if err != nil {
			check.add(at, "target_schemas: %v", err)
			continue
		}
		if earlier, held := kinds[id.Kind]; held {
			check.add(at, "target_schemas declares %s and %s, two of kind %s; a document renders for one version of a kind", earlier, text, id.Kind)
		}
		kinds[id.Kind] = text
	}
}

func (check *documentCheck) source(at place, source *Source) {
	if source != nil && source.Kind == MetricsSource && (source.IndexPattern != "" || source.TimeField != "") {
		check.add(at, "a metrics source takes no index_pattern or time_field")
	}
}

func (check *documentCheck) filter(at place, filter string) {
	parsed, err := dqlsyntax.ParseQuery(filter, QueryGrammarVersion())
	if err != nil {
		check.add(at, "filter: %v", err)
		return
	}
	if operands := parsed.Expression.Operands; len(operands) != 1 || operands[0].Query == nil || len(operands[0].Query.Stages) > 0 {
		check.add(at, "filter is a DQL filter, with no stages or arithmetic")
	}
	check.references(at, "filter", filter)
}

func (check *documentCheck) query(at place, query string) {
	if _, err := dqlsyntax.ParseQuery(query, QueryGrammarVersion()); err != nil {
		check.add(at, "query: %v", err)
	}
	check.references(at, "query", query)
}

// references holds every ${name} a text writes to a variable the dashboard declares.
func (check *documentCheck) references(at place, field, text string) {
	for _, reference := range dqlsyntax.VariableReferencePattern.FindAllString(text, -1) {
		if name := reference[2 : len(reference)-1]; !check.declares(name) {
			check.add(at, "%s names variable %q, which the dashboard does not declare", field, name)
		}
	}
}

func (check *documentCheck) declares(name string) bool {
	_, declared := check.dashboard.Variables[name]
	return declared
}

func (check *documentCheck) repeatFor(at place, variable string) {
	if variable != "" && !check.declares(variable) {
		check.add(at, "repeat_for names variable %q, which the dashboard does not declare", variable)
	}
}

func (check *documentCheck) links(at place, links []Link) {
	for position, link := range links {
		check.references(at.item("links", position), "url", link.URL)
	}
}

func (check *documentCheck) variable(at place, variable Variable) {
	choices, err := variable.ChoiceCall()
	if err != nil {
		check.add(at, "choices: %v", err)
		return
	}
	if choices.Query != "" {
		check.query(at, choices.Query)
	}
}

func (check *documentCheck) row(at place, row Row) {
	check.repeatFor(at, row.RepeatFor)
	stated, unstated := 0, 0
	for position, panel := range row.Panels {
		check.panel(at.item("panels", position), panel)
		if panel.Width == nil {
			unstated++
		} else {
			stated += *panel.Width
		}
	}
	switch {
	case stated > 100:
		check.add(at, "the panels' widths add up to %d%%, more than the row", stated)
	case stated == 100 && unstated > 0:
		check.add(at, "the panels with a width fill the row, leaving none for the %d without one", unstated)
	}
}
