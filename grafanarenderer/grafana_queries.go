package grafanarenderer

import (
	"errors"

	"github.com/litebitus/dql-compiler/dqlsyntax"
	"github.com/litebitus/dql-compiler/promqlbackend"

	"github.com/litebitus/terraform-provider-dashboard/dashboarddsl"
)

// queryCompiler compiles a document's queries into PromQL, each applying the dashboard's filter.
type queryCompiler struct {
	dashboardFilter dqlsyntax.FilterNode
}

func (render *dashboardRender) queryCompiler() queryCompiler {
	if render.dashboard.Filter == "" {
		return queryCompiler{}
	}
	parsed, err := dqlsyntax.ParseQuery(render.dashboard.Filter, dashboarddsl.QueryGrammarVersion())
	if err != nil {
		render.refuse("", "filter: "+err.Error())
		return queryCompiler{}
	}
	return queryCompiler{dashboardFilter: parsed.Expression.Operands[0].Query.Filter}
}

// promQL is a query in PromQL, under the dashboard's filter.
func (compiler queryCompiler) promQL(query string) (string, error) {
	parsed, err := compiler.filtered(query)
	if err != nil {
		return "", err
	}
	return promqlbackend.CompileQuery(parsed)
}

// seriesSelector is a variable's choices query as the series selector Grafana reads label values from, under the dashboard's filter.
func (compiler queryCompiler) seriesSelector(query string) (string, error) {
	parsed, err := compiler.filtered(query)
	if err != nil {
		return "", err
	}
	if operands := parsed.Expression.Operands; len(operands) != 1 || operands[0].Query == nil || len(operands[0].Query.Stages) > 0 {
		return "", errors.New("Grafana reads a variable's values from a series selector, so its choices query is a filter, with no stages or arithmetic")
	}
	return promqlbackend.CompileQuery(parsed)
}

// filtered is a query parsed, its every query operand also applying the dashboard's filter.
func (compiler queryCompiler) filtered(query string) (dqlsyntax.ParsedQuery, error) {
	parsed, err := dqlsyntax.ParseQuery(query, dashboarddsl.QueryGrammarVersion())
	if err != nil {
		return dqlsyntax.ParsedQuery{}, err
	}
	if compiler.dashboardFilter != nil {
		for _, operand := range parsed.Expression.Operands {
			if operand.Query == nil {
				continue
			}
			if operand.Query.Filter == nil {
				operand.Query.Filter = compiler.dashboardFilter
			} else {
				operand.Query.Filter = dqlsyntax.AllOf{Clauses: []dqlsyntax.FilterNode{compiler.dashboardFilter, operand.Query.Filter}}
			}
		}
	}
	return parsed, nil
}
