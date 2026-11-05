package grafanarenderer

import (
	"fmt"

	"github.com/litebitus/terraform-provider-dashboard/dashboarddsl"
)

// Refusal is a part of a document Grafana cannot draw as the document says: the dashboard, where in it, and why.
type Refusal struct {
	Dashboard string
	Place     string
	Reason    string
}

func (refusal Refusal) Error() string {
	if refusal.Place == "" {
		return fmt.Sprintf("dashboard %q: %s", refusal.Dashboard, refusal.Reason)
	}
	return fmt.Sprintf("dashboard %q, %s: %s", refusal.Dashboard, refusal.Place, refusal.Reason)
}

func panelPlace(rowPosition int, row dashboarddsl.Row, panelPosition int, panel dashboarddsl.Panel) string {
	return fmt.Sprintf("row %d %q, panel %d %q", rowPosition+1, row.Title, panelPosition+1, panel.Title)
}

func annotationPlace(position int, annotation dashboarddsl.Annotation) string {
	return fmt.Sprintf("annotation %d %q", position+1, annotation.Title)
}

func variablePlace(name string) string { return fmt.Sprintf("variable %s", name) }
