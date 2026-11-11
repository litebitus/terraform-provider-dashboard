package grafanarenderer

import (
	"testing"

	"github.com/litebitus/terraform-provider-dashboard/dashboarddsl"
)

// An ungrouped count answers 0 while the source of the query reports: its own source's reporting filter, or else the dashboard's.
func TestACountAnswersZeroWhileItsSourceReports(t *testing.T) {
	reporting := &dashboarddsl.Source{Kind: dashboarddsl.MetricsSource, ReportingFilter: "__name__:up and job:mock-exporter"}
	silent := &dashboarddsl.Source{Kind: dashboarddsl.MetricsSource}
	for name, testCase := range map[string]struct {
		dashboardSource, querySource *dashboarddsl.Source
		want                         string
	}{
		"the dashboard's source": {reporting, nil, `count(mock_waiting) or on() (count(up{job="mock-exporter"}) * 0)`},
		"the query's own source": {silent, reporting, `count(mock_waiting) or on() (count(up{job="mock-exporter"}) * 0)`},
		"a source without one":   {reporting, silent, `count(mock_waiting)`},
	} {
		render := &dashboardRender{dashboard: dashboarddsl.Dashboard{Source: testCase.dashboardSource}}
		got, err := render.queryCompiler().promQL("__name__:mock_waiting | count()", testCase.querySource)
		if err != nil || got != testCase.want {
			t.Errorf("%s: got %s (%v), want %s", name, got, err, testCase.want)
		}
	}
}
