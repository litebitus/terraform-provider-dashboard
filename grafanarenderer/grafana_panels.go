package grafanarenderer

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	yamldslspec "github.com/litebitus/yaml-dsl-spec/go"

	"github.com/litebitus/terraform-provider-dashboard/dashboarddsl"
)

// panelField is a panel field that only some of Grafana's panels draw.
type panelField string

const (
	axisField          panelField = "axis"
	calculationField   panelField = "calculation"
	decimalsField      panelField = "decimals"
	legendField        panelField = "legend"
	maxField           panelField = "max"
	minField           panelField = "min"
	stackedField       panelField = "stacked"
	thresholdsField    panelField = "thresholds"
	valueMappingsField panelField = "value_mappings"
)

// grafanaPanelKind is the Grafana panel a kind renders as: its type, the fields it draws, and the form its queries answer in.
type grafanaPanelKind struct {
	panelType    string
	draws        []panelField
	targetFormat string
	instant      bool
}

var (
	seriesFields = []panelField{
		axisField, decimalsField, legendField, maxField, minField, stackedField, thresholdsField, valueMappingsField,
	}
	reducedFields = []panelField{calculationField, decimalsField, maxField, minField, thresholdsField, valueMappingsField}
	tableFields   = []panelField{decimalsField, thresholdsField, valueMappingsField}
	stateFields   = []panelField{decimalsField, legendField, thresholdsField, valueMappingsField}
)

var grafanaPanelKinds = map[string]grafanaPanelKind{
	dashboarddsl.AreaKind:          {panelType: "timeseries", draws: seriesFields, targetFormat: "time_series"},
	dashboarddsl.BarKind:           {panelType: "bargauge", draws: reducedFields, targetFormat: "time_series"},
	dashboarddsl.GaugeKind:         {panelType: "gauge", draws: reducedFields, targetFormat: "time_series"},
	dashboarddsl.HeatmapKind:       {panelType: "heatmap", draws: []panelField{decimalsField, legendField}, targetFormat: "heatmap"},
	dashboarddsl.HistogramKind:     {panelType: "histogram", draws: []panelField{axisField, decimalsField, legendField}, targetFormat: "time_series"},
	dashboarddsl.MapKind:           {panelType: "geomap", draws: []panelField{decimalsField, thresholdsField}, targetFormat: "table", instant: true},
	dashboarddsl.PieKind:           {panelType: "piechart", draws: []panelField{calculationField, decimalsField, legendField, valueMappingsField}, targetFormat: "time_series"},
	dashboarddsl.RecordsKind:       {panelType: "table", draws: tableFields, targetFormat: "table", instant: true},
	dashboarddsl.StatKind:          {panelType: "stat", draws: reducedFields, targetFormat: "time_series"},
	dashboarddsl.StateTimelineKind: {panelType: "state-timeline", draws: stateFields, targetFormat: "time_series"},
	dashboarddsl.StatusHistoryKind: {panelType: "status-history", draws: stateFields, targetFormat: "time_series"},
	dashboarddsl.TableKind:         {panelType: "table", draws: tableFields, targetFormat: "table", instant: true},
	dashboarddsl.TextKind:          {panelType: "text"},
	dashboarddsl.TimeSeriesKind:    {panelType: "timeseries", draws: seriesFields, targetFormat: "time_series"},
}

var units = map[string]string{
	"bytes":      "bytes",
	"count":      "short",
	"percent":    "percent",
	"per_second": "suffix:/s",
	"seconds":    "s",
}

var colors = map[string]string{
	"blue":   "blue",
	"green":  "green",
	"grey":   "#808080",
	"orange": "orange",
	"purple": "purple",
	"red":    "red",
	"yellow": "yellow",
}

// The fill an area draws under its line; a line draws none.
const (
	areaFillOpacity = 25
	lineFillOpacity = 0
)

// baseColor is what a value below the lowest threshold is drawn in where no threshold states it: the theme's text color.
const baseColor = "text"

// timeFieldName is the field Grafana's Prometheus frames hold their evaluation time in.
const timeFieldName = "Time"

// millisecondDigits are the digits of a second a time in milliseconds counts in, the unit Grafana's date units read.
const millisecondDigits = 3

// element is a panel among the dashboard's elements, which the layout places by name.
type element struct {
	Kind string    `json:"kind"`
	Spec panelSpec `json:"spec"`
}

type panelSpec struct {
	ID          int        `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Links       []dataLink `json:"links"`
	Data        kindSpec   `json:"data"`
	VizConfig   vizConfig  `json:"vizConfig"`
}

type dataLink struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	TargetBlank bool   `json:"targetBlank"`
}

type queryGroupSpec struct {
	Queries         []kindSpec     `json:"queries"`
	Transformations []any          `json:"transformations"`
	QueryOptions    map[string]any `json:"queryOptions"`
}

type panelQuerySpec struct {
	Query  dataQuery `json:"query"`
	RefID  string    `json:"refId"`
	Hidden bool      `json:"hidden"`
}

type vizConfig struct {
	Kind    string        `json:"kind"`
	Group   string        `json:"group"`
	Version string        `json:"version"`
	Spec    vizConfigSpec `json:"spec"`
}

type vizConfigSpec struct {
	Options     map[string]any `json:"options"`
	FieldConfig fieldConfig    `json:"fieldConfig"`
}

type fieldConfig struct {
	Defaults  map[string]any `json:"defaults"`
	Overrides []any          `json:"overrides"`
}

// panel renders one panel as an element, refusing a kind Grafana has no panel for and a field its panel does not draw.
func (render *dashboardRender) panel(at string, id int, panel dashboarddsl.Panel) element {
	spec := panelSpec{ID: id, Title: panel.Title, Description: panel.Description, Links: []dataLink{}}
	for _, link := range panel.Links {
		spec.Links = append(spec.Links, dataLink{Title: link.Title, URL: link.URL, TargetBlank: true})
	}
	queries := queryGroupSpec{Queries: []kindSpec{}, Transformations: []any{}, QueryOptions: map[string]any{}}
	spec.Data = kindSpec{Kind: "QueryGroup", Spec: &queries}
	spec.VizConfig = vizConfig{Kind: "VizConfig", Version: render.target.PluginVersion, Spec: vizConfigSpec{
		Options: map[string]any{}, FieldConfig: fieldConfig{Defaults: map[string]any{}, Overrides: []any{}},
	}}
	kind, drawn := grafanaPanelKinds[panel.Kind]
	if !drawn {
		render.refuse(at, fmt.Sprintf("Grafana has no %s panel", panel.Kind))
		return element{Kind: "Panel", Spec: spec}
	}
	spec.VizConfig.Group = kind.panelType
	for _, field := range writtenFields(panel) {
		if !slices.Contains(kind.draws, field) {
			render.refuse(at, fmt.Sprintf("Grafana's %s panel draws no %s", kind.panelType, field))
		}
	}
	if panel.Kind == dashboarddsl.TextKind {
		spec.VizConfig.Spec.Options = map[string]any{"mode": "markdown", "content": panel.Content}
		return element{Kind: "Panel", Spec: spec}
	}
	queries.Queries = render.panelQueries(at, panel.Queries, kind)
	spec.VizConfig.Spec.FieldConfig.Defaults = fieldDefaults(panel)
	spec.VizConfig.Spec.Options = panelOptions(panel, kind)
	if panel.Kind == dashboarddsl.TableKind {
		queries.Transformations = tableTransformations(panel.Queries)
		spec.VizConfig.Spec.FieldConfig.Overrides = render.valueFormatOverrides(at, panel.Queries)
	}
	return element{Kind: "Panel", Spec: spec}
}

// tableTransformations merge a table's queries into one row per value of the fields they share, and show each
// query's values under its header, with no evaluation time.
func tableTransformations(queries []dashboarddsl.Query) []any {
	transformations := []any{}
	if len(queries) > 1 {
		transformations = append(transformations, grafanaTransformation("merge", map[string]any{}))
	}
	headers := map[string]any{}
	for position, query := range queries {
		if query.ValueColumnHeader != "" {
			headers[valueFieldName(position, len(queries))] = query.ValueColumnHeader
		}
	}
	organized := map[string]any{
		"excludeByName": map[string]any{timeFieldName: true},
		"indexByName":   map[string]any{},
		"renameByName":  headers,
	}
	return append(transformations, grafanaTransformation("organize", organized))
}

func grafanaTransformation(transformationID string, options map[string]any) map[string]any {
	return map[string]any{"kind": "Transformation", "group": transformationID, "spec": map[string]any{"options": options}}
}

// valueFieldName is the field a table query's values fill in Grafana's Prometheus frames: `Value` for a panel's one
// query, else `Value #` and its refId.
func valueFieldName(position, queryCount int) string {
	if queryCount == 1 {
		return "Value"
	}
	return "Value #" + referenceID(position)
}

// valueFormatOverrides write each query's value_format as the unit of the column its values fill.
func (render *dashboardRender) valueFormatOverrides(at string, queries []dashboarddsl.Query) []any {
	overrides := []any{}
	for position, query := range queries {
		if query.ValueFormat == "" {
			continue
		}
		queryAt := fmt.Sprintf("%s, query %d", at, position+1)
		overrides = append(overrides, map[string]any{
			"matcher":    map[string]any{"id": "byName", "options": valueFieldName(position, len(queries))},
			"properties": []any{map[string]any{"id": "unit", "value": render.dateUnit(queryAt, query.ValueFormat)}},
		})
	}
	return overrides
}

// dateUnit is a value_format layout as Grafana's custom date unit: its text kept literal, and `%C` as its date format.
func (render *dashboardRender) dateUnit(at, layout string) string {
	layoutParts, _ := yamldslspec.ParseLayout(layout)
	var dateFormat strings.Builder
	for _, layoutPart := range layoutParts {
		verb := layoutPart.Verb
		switch {
		case verb == nil:
			dateFormat.WriteString(render.literalDateText(at, layoutPart.Text))
		case verb.Letter != 'C':
			render.refuse(at, "Grafana writes a `value_format` only with `%C`, as a date")
		case verb.LeftAligned || verb.Width != nil:
			render.refuse(at, "Grafana writes `%C` with no width")
		case verb.DateFormat.Offset != nil:
			render.refuse(at, "Grafana writes `%C` in the dashboard's time zone, with no time offset")
		default:
			for _, dateFormatPart := range verb.DateFormat.Parts {
				if dateFormatPart.DateVerb == 0 {
					dateFormat.WriteString(render.literalDateText(at, dateFormatPart.Text))
				} else {
					dateFormat.WriteString(render.momentToken(at, dateFormatPart))
				}
			}
		}
	}
	return "time:" + dateFormat.String()
}

// momentTokens are the date verbs Grafana's date unit writes, as written, keyed to their Moment.js tokens.
var momentTokens = map[string]string{
	"%Y":   "YYYY",
	"%04Y": "YYYY",
	"%02y": "YY",
	"%m":   "M",
	"%02m": "MM",
	"%d":   "D",
	"%02d": "DD",
	"%H":   "H",
	"%02H": "HH",
	"%M":   "m",
	"%02M": "mm",
	"%S":   "s",
	"%02S": "ss",
	"%03L": "SSS",
}

// momentToken is a date verb as the Moment.js token Grafana's date unit writes it with.
func (render *dashboardRender) momentToken(at string, dateVerb yamldslspec.DateFormatPart) string {
	writtenDateVerb := "%"
	if dateVerb.LeftAligned {
		writtenDateVerb += "-"
	}
	if dateVerb.ZeroPadded {
		writtenDateVerb += "0"
	}
	if dateVerb.Width != nil {
		writtenDateVerb += strconv.Itoa(*dateVerb.Width)
	}
	writtenDateVerb += string(dateVerb.DateVerb)
	token, isWritten := momentTokens[writtenDateVerb]
	if !isWritten {
		render.refuse(at, "Grafana's date format writes no "+yamldslspec.CodeSpan(writtenDateVerb))
	}
	return token
}

// millisecondsExpression scales a query's value, a time in the digits of a second its `%C` precision states, to the
// milliseconds Grafana's date units read.
func millisecondsExpression(expression, valueFormat string) string {
	layoutParts, _ := yamldslspec.ParseLayout(valueFormat)
	timeDigits := 0
	for _, layoutPart := range layoutParts {
		if layoutPart.Verb != nil && layoutPart.Verb.Precision != nil {
			timeDigits = *layoutPart.Verb.Precision
		}
	}
	digitsApart := timeDigits - millisecondDigits
	switch {
	case digitsApart < 0:
		return "(" + expression + ") * 1" + strings.Repeat("0", -digitsApart)
	case digitsApart > 0:
		return "(" + expression + ") / 1" + strings.Repeat("0", digitsApart)
	}
	return expression
}

// literalDateText is text a date format writes as it is, which Moment.js reads between brackets.
func (render *dashboardRender) literalDateText(at, text string) string {
	if strings.Contains(text, "]") {
		render.refuse(at, "Grafana's date format holds no `]` in its text")
	}
	return "[" + text + "]"
}

// writtenFields are the fields a panel states that only some of Grafana's panels draw.
func writtenFields(panel dashboarddsl.Panel) []panelField {
	var written []panelField
	for field, isWritten := range map[panelField]bool{
		axisField:          panel.Axis != nil,
		calculationField:   panel.Calculation != "",
		decimalsField:      panel.Decimals != nil,
		legendField:        panel.Legend != nil,
		maxField:           panel.Max != nil,
		minField:           panel.Min != nil,
		stackedField:       panel.Stacked,
		thresholdsField:    len(panel.Thresholds) > 0,
		valueMappingsField: len(panel.ValueMappings) > 0,
	} {
		if isWritten {
			written = append(written, field)
		}
	}
	slices.Sort(written)
	return written
}

func (render *dashboardRender) panelQueries(at string, queries []dashboarddsl.Query, kind grafanaPanelKind) []kindSpec {
	panelQueries := make([]kindSpec, 0, len(queries))
	for position, query := range queries {
		queryAt := fmt.Sprintf("%s, query %d", at, position+1)
		if query.Source != nil && query.Source.Kind != dashboarddsl.MetricsSource {
			render.refuse(queryAt, "Grafana reads no OpenSearch data source")
		}
		expression, err := render.queries.promQL(query.Query, query.Source)
		if err != nil {
			render.refuse(queryAt, err.Error())
		}
		if query.ValueFormat != "" {
			expression = millisecondsExpression(expression, query.ValueFormat)
		}
		querySpec := map[string]any{
			"expr":    expression,
			"range":   !kind.instant,
			"instant": kind.instant,
			"format":  kind.targetFormat,
		}
		if query.LegendFormat != "" {
			querySpec["legendFormat"] = query.LegendFormat
		}
		if query.MinInterval != "" {
			querySpec["interval"] = query.MinInterval
		}
		panelQueries = append(panelQueries, kindSpec{Kind: "PanelQuery", Spec: panelQuerySpec{
			Query: prometheusQuery(querySpec), RefID: referenceID(position),
		}})
	}
	return panelQueries
}

// referenceID is a query's Grafana refId: A to Z, then AA, AB and on.
func referenceID(position int) string {
	if position < 26 {
		return string(rune('A' + position))
	}
	return referenceID(position/26-1) + string(rune('A'+position%26))
}

func fieldDefaults(panel dashboarddsl.Panel) map[string]any {
	steps := []map[string]any{{"color": baseColor, "value": nil}}
	for _, threshold := range panel.Thresholds {
		if threshold.Value == nil {
			steps[0]["color"] = colors[threshold.Color]
			continue
		}
		steps = append(steps, map[string]any{"color": colors[threshold.Color], "value": *threshold.Value})
	}
	mappings := []any{}
	if len(panel.ValueMappings) > 0 {
		options := map[string]any{}
		for index, value := range slices.Sorted(maps.Keys(panel.ValueMappings)) {
			mapping := panel.ValueMappings[value]
			option := map[string]any{"text": mapping.Text, "index": index}
			if mapping.Color != "" {
				option["color"] = colors[mapping.Color]
			}
			options[value] = option
		}
		mappings = append(mappings, map[string]any{"type": "value", "options": options})
	}
	defaults := map[string]any{
		"unit":       units[panel.Unit],
		"thresholds": map[string]any{"mode": "absolute", "steps": steps},
		"mappings":   mappings,
		"custom":     customDefaults(panel),
	}
	if panel.Decimals != nil {
		defaults["decimals"] = *panel.Decimals
	}
	if panel.Min != nil {
		defaults["min"] = *panel.Min
	}
	if panel.Max != nil {
		defaults["max"] = *panel.Max
	}
	return defaults
}

// customDefaults are the series settings a time series and a histogram draw with.
func customDefaults(panel dashboarddsl.Panel) map[string]any {
	if panel.Kind != dashboarddsl.TimeSeriesKind && panel.Kind != dashboarddsl.AreaKind && panel.Kind != dashboarddsl.HistogramKind {
		return map[string]any{}
	}
	scale := map[string]any{"type": "linear"}
	label := ""
	if panel.Axis != nil {
		label = panel.Axis.Label
		if panel.Axis.Scale == "log" {
			scale = map[string]any{"type": "log", "log": 10}
		}
	}
	custom := map[string]any{"axisLabel": label, "scaleDistribution": scale}
	if panel.Kind == dashboarddsl.HistogramKind {
		return custom
	}
	custom["fillOpacity"] = lineFillOpacity
	if panel.Kind == dashboarddsl.AreaKind {
		custom["fillOpacity"] = areaFillOpacity
	}
	stacking := "none"
	if panel.Stacked {
		stacking = "normal"
	}
	custom["stacking"] = map[string]any{"mode": stacking, "group": "A"}
	return custom
}

func panelOptions(panel dashboarddsl.Panel, kind grafanaPanelKind) map[string]any {
	reduced := map[string]any{"calcs": []string{panel.Calculation}, "fields": "", "values": false}
	switch kind.panelType {
	case "timeseries":
		return map[string]any{"legend": legendOptions(panel.Legend), "tooltip": map[string]any{"mode": "multi", "sort": "none"}}
	case "bargauge":
		return map[string]any{
			"reduceOptions": reduced, "orientation": panel.Orientation, "displayMode": "basic", "showUnfilled": true,
		}
	case "gauge":
		return map[string]any{"reduceOptions": reduced, "showThresholdMarkers": true, "showThresholdLabels": false}
	case "stat":
		return map[string]any{
			"reduceOptions": reduced, "colorMode": "value", "graphMode": "none", "textMode": "auto", "justifyMode": "auto",
		}
	case "piechart":
		return map[string]any{"reduceOptions": reduced, "pieType": "pie", "legend": legendOptions(panel.Legend), "displayLabels": []string{}}
	case "heatmap":
		shown := panel.Legend == nil || panel.Legend.Show
		return map[string]any{"calculate": false, "legend": map[string]any{"show": shown}}
	case "histogram":
		return map[string]any{"legend": legendOptions(panel.Legend), "combine": false}
	case "geomap":
		return map[string]any{
			"view":     map[string]any{"id": "zero", "lat": 0, "lon": 0, "zoom": 1},
			"controls": map[string]any{"showZoom": true, "mouseWheelZoom": true},
			"basemap":  map[string]any{"type": "default", "name": "Basemap"},
			"layers": []map[string]any{{
				"type": "markers",
				"name": panel.Title,
				"location": map[string]any{
					"mode": "lookup", "lookup": panel.LocationField, "gazetteer": "public/gazetteer/countries.json",
				},
			}},
		}
	case "table":
		return map[string]any{"showHeader": true}
	case "state-timeline":
		return map[string]any{"legend": legendOptions(panel.Legend), "mergeValues": true, "showValue": "auto"}
	case "status-history":
		return map[string]any{"legend": legendOptions(panel.Legend), "showValue": "auto"}
	}
	return map[string]any{}
}

// legendOptions is a legend as Grafana holds it; a panel that states none shows its legend as a list at the bottom.
func legendOptions(legend *dashboarddsl.Legend) map[string]any {
	if legend == nil {
		return map[string]any{"showLegend": true, "displayMode": "list", "placement": "bottom", "calcs": []string{}}
	}
	placement := legend.Position
	if placement == "" {
		placement = "bottom"
	}
	return map[string]any{
		"showLegend": legend.Show, "displayMode": "list", "placement": placement, "calcs": []string{},
	}
}
