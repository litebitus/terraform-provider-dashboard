// Package grafanarenderer renders a dashboard DSL document into a Grafana dashboard resource of a target schema it carries, refusing what Grafana cannot draw.
package grafanarenderer

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"

	"github.com/litebitus/terraform-provider-dashboard/dashboarddsl"
	"github.com/litebitus/terraform-provider-dashboard/targetschema"
)

// RenderDashboard answers a parsed document as a Grafana dashboard resource of the target schema given, a carried minor, or every refusal it meets.
func RenderDashboard(dashboard dashboarddsl.Dashboard, target targetschema.ID) ([]byte, error) {
	carried, err := targetSchemaFor(target)
	if err != nil {
		return nil, err
	}
	render := &dashboardRender{dashboard: dashboard, target: carried}
	resource := render.resource()
	if len(render.refusals) > 0 {
		return nil, errors.Join(render.refusals...)
	}
	if err := conforms(carried, resource.Spec); err != nil {
		return nil, err
	}
	return json.MarshalIndent(resource, "", "  ")
}

// dashboardRender is one render in progress: the document, its target, the filter its queries apply, and the refusals met.
type dashboardRender struct {
	dashboard dashboarddsl.Dashboard
	target    TargetSchema
	queries   queryCompiler
	refusals  []error
}

func (render *dashboardRender) refuse(at, reason string) {
	render.refusals = append(render.refusals, Refusal{Dashboard: render.dashboard.Title, Place: at, Reason: reason})
}

type dashboardResource struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   resourceMetadata `json:"metadata"`
	Spec       dashboardSpec    `json:"spec"`
}

// resourceMetadata is the resource's metadata: its name, the document's id, or none for Grafana to assign one.
type resourceMetadata struct {
	Name string `json:"name,omitempty"`
}

// dashboardNamePattern is what Grafana takes as a dashboard's name: lowercase letters, digits and hyphens, beginning and ending with a letter or digit, at most 40.
var dashboardNamePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,38}[a-z0-9])?$`)

type dashboardSpec struct {
	Annotations  []annotationQuery  `json:"annotations"`
	CursorSync   string             `json:"cursorSync"`
	Description  string             `json:"description,omitempty"`
	Editable     bool               `json:"editable"`
	Elements     map[string]element `json:"elements"`
	Layout       kindSpec           `json:"layout"`
	Links        []dashboardLink    `json:"links"`
	Preload      bool               `json:"preload"`
	Tags         []string           `json:"tags"`
	TimeSettings timeSettings       `json:"timeSettings"`
	Title        string             `json:"title"`
	Variables    []kindSpec         `json:"variables"`
}

// kindSpec is the shape every V2 part takes: what kind it is, and its spec.
type kindSpec struct {
	Kind string `json:"kind"`
	Spec any    `json:"spec"`
}

type dashboardLink struct {
	Title       string   `json:"title"`
	Type        string   `json:"type"`
	Icon        string   `json:"icon"`
	Tooltip     string   `json:"tooltip"`
	URL         string   `json:"url"`
	Tags        []string `json:"tags"`
	AsDropdown  bool     `json:"asDropdown"`
	TargetBlank bool     `json:"targetBlank"`
	IncludeVars bool     `json:"includeVars"`
	KeepTime    bool     `json:"keepTime"`
}

type timeSettings struct {
	Timezone             string   `json:"timezone"`
	From                 string   `json:"from"`
	To                   string   `json:"to"`
	AutoRefresh          string   `json:"autoRefresh"`
	AutoRefreshIntervals []string `json:"autoRefreshIntervals"`
	HideTimepicker       bool     `json:"hideTimepicker"`
	FiscalYearStartMonth int      `json:"fiscalYearStartMonth"`
}

type annotationQuery struct {
	Kind string              `json:"kind"`
	Spec annotationQuerySpec `json:"spec"`
}

type annotationQuerySpec struct {
	Query     dataQuery `json:"query"`
	Enable    bool      `json:"enable"`
	Hide      bool      `json:"hide"`
	IconColor string    `json:"iconColor"`
	Name      string    `json:"name"`
}

// dataQuery is a query of Grafana's default Prometheus data source.
type dataQuery struct {
	Kind    string         `json:"kind"`
	Group   string         `json:"group"`
	Version string         `json:"version"`
	Spec    map[string]any `json:"spec"`
}

func prometheusQuery(spec map[string]any) dataQuery {
	return dataQuery{Kind: "DataQuery", Group: "prometheus", Version: "v0", Spec: spec}
}

// autoRefreshIntervals are the refresh picker's choices, the dashboard's own refresh interval among them.
var autoRefreshIntervals = []string{"5s", "10s", "30s", "1m", "5m", "15m", "30m", "1h", "2h", "1d"}

func (render *dashboardRender) resource() dashboardResource {
	dashboard := render.dashboard
	if dashboard.Source.Kind != dashboarddsl.MetricsSource {
		render.refuse("", "Grafana reads no OpenSearch data source; a logs dashboard renders for OpenSearch Dashboards")
	}
	if dashboard.ID != "" && !dashboardNamePattern.MatchString(dashboard.ID) {
		render.refuse("", fmt.Sprintf("id %q is not a name Grafana takes: lowercase letters, digits and hyphens, "+
			"beginning and ending with a letter or digit, at most 40", dashboard.ID))
	}
	render.queries = render.queryCompiler()
	intervals := slices.Clone(autoRefreshIntervals)
	if !slices.Contains(intervals, dashboard.RefreshInterval) {
		intervals = append(intervals, dashboard.RefreshInterval)
	}
	spec := dashboardSpec{
		Annotations: []annotationQuery{},
		CursorSync:  "Off",
		Description: dashboard.Description,
		Links:       []dashboardLink{},
		Tags:        nonNil(dashboard.Tags),
		TimeSettings: timeSettings{
			Timezone:             dashboard.Timezone,
			From:                 "now-" + dashboard.TimeRange,
			To:                   "now",
			AutoRefresh:          dashboard.RefreshInterval,
			AutoRefreshIntervals: intervals,
		},
		Title:     dashboard.Title,
		Variables: []kindSpec{},
	}
	for _, link := range dashboard.Links {
		spec.Links = append(spec.Links, dashboardLink{
			Title: link.Title, Type: "link", Icon: "external link", Tooltip: link.Title, URL: link.URL,
			Tags: []string{}, TargetBlank: true,
		})
	}
	for position, annotation := range dashboard.Annotations {
		spec.Annotations = append(spec.Annotations, render.annotation(position, annotation))
	}
	for _, name := range slices.Sorted(maps.Keys(dashboard.Variables)) {
		spec.Variables = append(spec.Variables, render.variable(name, dashboard.Variables[name]))
	}
	spec.Elements, spec.Layout = render.layOutRows()
	return dashboardResource{
		APIVersion: render.target.APIVersion,
		Kind:       "Dashboard",
		Metadata:   resourceMetadata{Name: dashboard.ID},
		Spec:       spec,
	}
}

func (render *dashboardRender) annotation(position int, annotation dashboarddsl.Annotation) annotationQuery {
	at := annotationPlace(position, annotation)
	if annotation.Source != nil && annotation.Source.Kind != dashboarddsl.MetricsSource {
		render.refuse(at, "Grafana reads no OpenSearch data source")
	}
	expression, err := render.queries.promQL(annotation.Query, annotation.Source)
	if err != nil {
		render.refuse(at, err.Error())
	}
	return annotationQuery{Kind: "AnnotationQuery", Spec: annotationQuerySpec{
		Query:     prometheusQuery(map[string]any{"expr": expression, "refId": "Anno"}),
		Enable:    true,
		IconColor: "blue",
		Name:      annotation.Title,
	}}
}

func nonNil[T any](values []T) []T {
	if values == nil {
		return []T{}
	}
	return values
}
