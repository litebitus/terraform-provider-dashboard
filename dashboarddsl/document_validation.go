package dashboarddsl

import (
	"fmt"
	"maps"
	"slices"

	"github.com/litebitus/dql-compiler/dqlsyntax"
	yamldslspec "github.com/litebitus/yaml-dsl-spec/go"

	"github.com/litebitus/terraform-provider-dashboard/targetschema"
)

// documentCheck holds a document the schema admits to the rules a schema cannot state, collecting every problem so one plan names them all.
type documentCheck struct {
	dashboard    Dashboard
	documentRoot yamldslspec.Node
	problems     []yamldslspec.Diagnostic
}

func (check *documentCheck) addAtValue(path documentPath, format string, arguments ...any) {
	valuePosition, _ := path.nodePositions(check.documentRoot)
	check.problems = append(check.problems, yamldslspec.DiagnosticAt(valuePosition, fmt.Sprintf(format, arguments...)))
}

func (check *documentCheck) addAtKey(path documentPath, format string, arguments ...any) {
	_, keyPosition := path.nodePositions(check.documentRoot)
	check.problems = append(check.problems, yamldslspec.DiagnosticAt(keyPosition, fmt.Sprintf(format, arguments...)))
}

func validateDashboard(dashboard Dashboard, documentRoot yamldslspec.Node) []yamldslspec.Diagnostic {
	check := &documentCheck{dashboard: dashboard, documentRoot: documentRoot}
	var at documentPath
	check.targetSchemas(at.field("target_schemas"), dashboard.TargetSchemas)
	check.source(at.field("source"), dashboard.Source)
	if dashboard.Filter != "" {
		check.filter(at.field("filter"), "filter", dashboard.Filter)
	}
	check.links(at.field("links"), dashboard.Links)
	for position, annotation := range dashboard.Annotations {
		annotationAt := at.field("annotations").item(position)
		check.query(annotationAt.field("query"), annotation.Query)
		check.source(annotationAt.field("source"), annotation.Source)
	}
	for _, name := range slices.Sorted(maps.Keys(dashboard.Variables)) {
		check.variable(at.field("variables").field(name), dashboard.Variables[name])
	}
	for position, row := range dashboard.Rows {
		check.row(at.field("rows").item(position), row)
	}
	return check.problems
}

// targetSchemas holds the document to at most one target schema per kind.
func (check *documentCheck) targetSchemas(at documentPath, declared []string) {
	kinds := map[string]string{}
	for position, text := range declared {
		id, err := targetschema.ParseID(text)
		if err != nil {
			check.addAtValue(at.item(position), "`target_schemas`: %v", err)
			continue
		}
		if earlier, held := kinds[id.Kind]; held {
			check.addAtValue(at, "`target_schemas` declares %s and %s, two of kind %s; a document renders for one version of a kind",
				yamldslspec.CodeSpan(earlier), yamldslspec.CodeSpan(text), yamldslspec.CodeSpan(id.Kind))
		}
		kinds[id.Kind] = text
	}
}

func (check *documentCheck) source(at documentPath, source *Source) {
	if source != nil && source.Kind == MetricsSource && (source.IndexPattern != "" || source.TimeField != "") {
		writtenField := "index_pattern"
		if source.IndexPattern == "" {
			writtenField = "time_field"
		}
		check.addAtKey(at.field(writtenField), "`source`: a `metrics` source takes no `index_pattern` or `time_field`")
	}
	if source == nil || source.ReportingFilter == "" {
		return
	}
	if source.Kind == LogsSource {
		check.addAtKey(at.field("reporting_filter"), "`source`: a `logs` source takes no `reporting_filter`")
		return
	}
	check.filter(at.field("reporting_filter"), "reporting_filter", source.ReportingFilter)
}

func (check *documentCheck) filter(at documentPath, field, filter string) {
	parsed, err := dqlsyntax.ParseQuery(filter, QueryGrammarVersion())
	if err != nil {
		check.addAtValue(at, "%s: %v", yamldslspec.CodeSpan(field), err)
		return
	}
	if operands := parsed.Expression.Operands; len(operands) != 1 || operands[0].Query == nil || len(operands[0].Query.Stages) > 0 {
		check.addAtValue(at, "%s is a DQL filter, with no stages or arithmetic", yamldslspec.CodeSpan(field))
	}
	check.references(at, field, filter)
}

func (check *documentCheck) query(at documentPath, query string) {
	if _, err := dqlsyntax.ParseQuery(query, QueryGrammarVersion()); err != nil {
		check.addAtValue(at, "`query`: %v", err)
	}
	check.references(at, "query", query)
}

// references holds every ${name} a text writes to a variable the dashboard declares.
func (check *documentCheck) references(at documentPath, field, text string) {
	for _, reference := range dqlsyntax.VariableReferencePattern.FindAllString(text, -1) {
		if name := reference[2 : len(reference)-1]; !check.declares(name) {
			check.addAtValue(at, "%s names variable %s, which the dashboard does not declare",
				yamldslspec.CodeSpan(field), yamldslspec.CodeSpan(name))
		}
	}
}

func (check *documentCheck) declares(name string) bool {
	_, declared := check.dashboard.Variables[name]
	return declared
}

func (check *documentCheck) repeatFor(at documentPath, variable string) {
	if variable != "" && !check.declares(variable) {
		check.addAtValue(at, "`repeat_for` names variable %s, which the dashboard does not declare", yamldslspec.CodeSpan(variable))
	}
}

func (check *documentCheck) links(at documentPath, links []Link) {
	for position, link := range links {
		check.references(at.item(position).field("url"), "url", link.URL)
	}
}

func (check *documentCheck) variable(at documentPath, variable Variable) {
	choices, err := variable.ChoiceCall()
	if err != nil {
		check.addAtValue(at.field("choices"), "`choices`: %v", err)
		return
	}
	if choices.Query != "" {
		check.query(at.field("choices"), choices.Query)
	}
}

func (check *documentCheck) row(at documentPath, row Row) {
	check.repeatFor(at.field("repeat_for"), row.RepeatFor)
	stated, unstated := 0, 0
	for position, panel := range row.Panels {
		check.panel(at.field("panels").item(position), panel)
		if panel.Width == nil {
			unstated++
		} else {
			stated += *panel.Width
		}
	}
	switch {
	case stated > 100:
		check.addAtValue(at, "the panels' widths add up to %d%%, more than the row", stated)
	case stated == 100 && unstated > 0:
		check.addAtValue(at, "the panels with a width fill the row, leaving none for the %d without one", unstated)
	}
}
