// Package dashboarddsl reads a dashboard DSL document: what a dashboard shows, in words no UI owns.
package dashboarddsl

// Dashboard is a dashboard document: a dashboard holds rows, a row holds panels, and a panel holds queries.
type Dashboard struct {
	SchemaVersion   string              `yaml:"schema_version"`
	TargetSchemas   []string            `yaml:"target_schemas"`
	ID              string              `yaml:"id"`
	Title           string              `yaml:"title"`
	Description     string              `yaml:"description"`
	Tags            []string            `yaml:"tags"`
	TimeRange       string              `yaml:"time_range"`
	RefreshInterval string              `yaml:"refresh_interval"`
	Timezone        string              `yaml:"timezone"`
	Source          *Source             `yaml:"source"`
	Filter          string              `yaml:"filter"`
	Links           []Link              `yaml:"links"`
	Annotations     []Annotation        `yaml:"annotations"`
	Variables       map[string]Variable `yaml:"variables"`
	Rows            []Row               `yaml:"rows"`
}

// Source is where queries read from.
type Source struct {
	Kind         string `yaml:"kind"`
	IndexPattern string `yaml:"index_pattern"`
	TimeField    string `yaml:"time_field"`
}

// Link is a link shown with a dashboard or a panel.
type Link struct {
	Title string `yaml:"title"`
	URL   string `yaml:"url"`
}

// Annotation is a mark on the time axis, one per item its query answers.
type Annotation struct {
	Title  string  `yaml:"title"`
	Query  string  `yaml:"query"`
	Source *Source `yaml:"source"`
}

// Variable is a value the viewer chooses, referenced in queries as ${<name>}.
type Variable struct {
	Choices    string `yaml:"choices"`
	Default    string `yaml:"default"`
	Multiple   bool   `yaml:"multiple"`
	IncludeAll bool   `yaml:"include_all"`
}

// Row is a row of panels.
type Row struct {
	Title     string  `yaml:"title"`
	Collapsed bool    `yaml:"collapsed"`
	RepeatFor string  `yaml:"repeat_for"`
	Panels    []Panel `yaml:"panels"`
}

// Panel is one thing a dashboard draws.
type Panel struct {
	Title         string            `yaml:"title"`
	Description   string            `yaml:"description"`
	Kind          string            `yaml:"kind"`
	Width         *int              `yaml:"width"`
	Height        int               `yaml:"height"`
	RepeatFor     string            `yaml:"repeat_for"`
	Unit          string            `yaml:"unit"`
	Decimals      *int              `yaml:"decimals"`
	Min           *float64          `yaml:"min"`
	Max           *float64          `yaml:"max"`
	Calculation   string            `yaml:"calculation"`
	Thresholds    []Threshold       `yaml:"thresholds"`
	ValueMappings map[string]string `yaml:"value_mappings"`
	Legend        *Legend           `yaml:"legend"`
	Axis          *Axis             `yaml:"axis"`
	Stacked       bool              `yaml:"stacked"`
	Orientation   string            `yaml:"orientation"`
	LocationField string            `yaml:"location_field"`
	Content       string            `yaml:"content"`
	Links         []Link            `yaml:"links"`
	Queries       []Query           `yaml:"queries"`
}

// Threshold is the color a value takes from Value up.
type Threshold struct {
	Value float64 `yaml:"value"`
	Color string  `yaml:"color"`
}

// Legend is whether a panel shows its legend, and where.
type Legend struct {
	Show     bool   `yaml:"show"`
	Position string `yaml:"position"`
}

// Axis is the scale values are drawn on, and its label.
type Axis struct {
	Scale string `yaml:"scale"`
	Label string `yaml:"label"`
}

// Query is one query of a panel, in DQL.
type Query struct {
	Query        string  `yaml:"query"`
	Source       *Source `yaml:"source"`
	LegendFormat string  `yaml:"legend_format"`
	MinInterval  string  `yaml:"min_interval"`
}

// The panel kinds.
const (
	AreaKind          = "area"
	BarKind           = "bar"
	GaugeKind         = "gauge"
	HeatmapKind       = "heatmap"
	HistogramKind     = "histogram"
	MapKind           = "map"
	PieKind           = "pie"
	RecordsKind       = "records"
	StatKind          = "stat"
	StateTimelineKind = "state_timeline"
	StatusHistoryKind = "status_history"
	TableKind         = "table"
	TagCloudKind      = "tag_cloud"
	TextKind          = "text"
	TimeSeriesKind    = "time_series"
)

// The source kinds.
const (
	MetricsSource = "metrics"
	LogsSource    = "logs"
)
