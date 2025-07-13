package monitoring

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GrafanaDashboardGenerator generates Grafana dashboard JSON templates
type GrafanaDashboardGenerator struct {
	config *Config
}

// NewGrafanaDashboardGenerator creates a new dashboard generator
func NewGrafanaDashboardGenerator(config *Config) *GrafanaDashboardGenerator {
	return &GrafanaDashboardGenerator{
		config: config,
	}
}

// GrafanaDashboard represents a Grafana dashboard
type GrafanaDashboard struct {
	Dashboard DashboardJSON `json:"dashboard"`
	FolderID  int          `json:"folderId"`
	Overwrite bool         `json:"overwrite"`
}

// DashboardJSON represents the dashboard structure
type DashboardJSON struct {
	ID            *int         `json:"id"`
	UID           string       `json:"uid,omitempty"`
	Title         string       `json:"title"`
	Tags          []string     `json:"tags"`
	Timezone      string       `json:"timezone"`
	SchemaVersion int          `json:"schemaVersion"`
	Version       int          `json:"version"`
	Panels        []Panel      `json:"panels"`
	Time          TimeRange    `json:"time"`
	Timepicker    TimePicker   `json:"timepicker"`
	Templating    Templating   `json:"templating"`
	Annotations   Annotations  `json:"annotations"`
	Refresh       string       `json:"refresh"`
	Variables     []Variable   `json:"variables,omitempty"`
}

// Panel represents a Grafana panel
type Panel struct {
	ID          int                    `json:"id"`
	GridPos     GridPos                `json:"gridPos"`
	Type        string                 `json:"type"`
	Title       string                 `json:"title"`
	Datasource  Datasource             `json:"datasource"`
	Targets     []Target               `json:"targets"`
	Options     map[string]interface{} `json:"options,omitempty"`
	FieldConfig FieldConfig            `json:"fieldConfig,omitempty"`
	Transparent bool                   `json:"transparent,omitempty"`
}

// GridPos represents panel position
type GridPos struct {
	H int `json:"h"`
	W int `json:"w"`
	X int `json:"x"`
	Y int `json:"y"`
}

// Datasource represents a data source
type Datasource struct {
	Type string `json:"type"`
	UID  string `json:"uid"`
}

// Target represents a query target
type Target struct {
	RefID         string `json:"refId"`
	Expr          string `json:"expr"`
	LegendFormat  string `json:"legendFormat,omitempty"`
	Interval      string `json:"interval,omitempty"`
	IntervalMs    int    `json:"intervalMs,omitempty"`
	MaxDataPoints int    `json:"maxDataPoints,omitempty"`
}

// FieldConfig represents field configuration
type FieldConfig struct {
	Defaults  FieldDefaults   `json:"defaults"`
	Overrides []FieldOverride `json:"overrides,omitempty"`
}

// FieldDefaults represents field defaults
type FieldDefaults struct {
	Color       ColorConfig            `json:"color,omitempty"`
	Mappings    []interface{}          `json:"mappings,omitempty"`
	Thresholds  ThresholdConfig        `json:"thresholds,omitempty"`
	Unit        string                 `json:"unit,omitempty"`
	Custom      map[string]interface{} `json:"custom,omitempty"`
	Min         *float64               `json:"min,omitempty"`
	Max         *float64               `json:"max,omitempty"`
	Decimals    *int                   `json:"decimals,omitempty"`
}

// ColorConfig represents color configuration
type ColorConfig struct {
	Mode       string `json:"mode"`
	FixedColor string `json:"fixedColor,omitempty"`
}

// ThresholdConfig represents threshold configuration
type ThresholdConfig struct {
	Mode  string      `json:"mode"`
	Steps []Threshold `json:"steps"`
}

// Threshold represents a single threshold
type Threshold struct {
	Color string   `json:"color"`
	Value *float64 `json:"value"`
}

// FieldOverride represents field overrides
type FieldOverride struct {
	Matcher    Matcher                `json:"matcher"`
	Properties []OverrideProperty     `json:"properties"`
}

// Matcher represents a field matcher
type Matcher struct {
	ID      string                 `json:"id"`
	Options map[string]interface{} `json:"options"`
}

// OverrideProperty represents an override property
type OverrideProperty struct {
	ID    string      `json:"id"`
	Value interface{} `json:"value"`
}

// TimeRange represents time range
type TimeRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// TimePicker represents time picker settings
type TimePicker struct {
	RefreshIntervals []string `json:"refresh_intervals"`
	TimeOptions      []string `json:"time_options"`
}

// Templating represents dashboard templating
type Templating struct {
	List []Variable `json:"list"`
}

// Variable represents a dashboard variable
type Variable struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Label      string                 `json:"label,omitempty"`
	Query      string                 `json:"query,omitempty"`
	Datasource Datasource             `json:"datasource,omitempty"`
	Current    map[string]interface{} `json:"current,omitempty"`
	Options    []interface{}          `json:"options,omitempty"`
	Multi      bool                   `json:"multi,omitempty"`
	IncludeAll bool                   `json:"includeAll,omitempty"`
}

// Annotations represents dashboard annotations
type Annotations struct {
	List []Annotation `json:"list"`
}

// Annotation represents a single annotation
type Annotation struct {
	Name       string     `json:"name"`
	Datasource Datasource `json:"datasource"`
	Enable     bool       `json:"enable"`
	Hide       bool       `json:"hide"`
	Query      string     `json:"query,omitempty"`
}

// GenerateOverviewDashboard generates the main overview dashboard
func (g *GrafanaDashboardGenerator) GenerateOverviewDashboard() (*GrafanaDashboard, error) {
	dashboard := &DashboardJSON{
		Title:         "Event Validation Overview",
		Tags:          []string{"events", "validation", "overview"},
		Timezone:      "browser",
		SchemaVersion: 26,
		Version:       1,
		Time: TimeRange{
			From: "now-6h",
			To:   "now",
		},
		Timepicker: TimePicker{
			RefreshIntervals: []string{"5s", "10s", "30s", "1m", "5m", "15m", "30m", "1h", "2h", "1d"},
			TimeOptions:      []string{"5m", "15m", "1h", "6h", "12h", "24h", "2d", "7d", "30d"},
		},
		Refresh: "30s",
		Panels:  []Panel{},
	}
	
	// Row 1: Key Metrics
	dashboard.Panels = append(dashboard.Panels,
		g.createStatPanel(1, "Total Events", "sum(increase(ag_ui_events_validation_total[5m]))", 0, 0, 6, 4),
		g.createStatPanel(2, "Error Rate", "sum(rate(ag_ui_events_validation_errors_total[5m])) / sum(rate(ag_ui_events_validation_total[5m])) * 100", 6, 0, 6, 4),
		g.createStatPanel(3, "Throughput", "sum(rate(ag_ui_events_validation_total[5m]))", 12, 0, 6, 4),
		g.createStatPanel(4, "SLA Compliance", "avg(ag_ui_events_sla_compliance_percent)", 18, 0, 6, 4),
	)
	
	// Row 2: Time Series
	dashboard.Panels = append(dashboard.Panels,
		g.createTimeSeriesPanel(5, "Event Processing Rate", []Target{
			{RefID: "A", Expr: "sum(rate(ag_ui_events_validation_total[5m])) by (status)", LegendFormat: "{{status}}"},
		}, 0, 4, 12, 8),
		g.createTimeSeriesPanel(6, "Validation Latency", []Target{
			{RefID: "A", Expr: "histogram_quantile(0.5, sum(rate(ag_ui_events_validation_duration_seconds_bucket[5m])) by (le))", LegendFormat: "p50"},
			{RefID: "B", Expr: "histogram_quantile(0.95, sum(rate(ag_ui_events_validation_duration_seconds_bucket[5m])) by (le))", LegendFormat: "p95"},
			{RefID: "C", Expr: "histogram_quantile(0.99, sum(rate(ag_ui_events_validation_duration_seconds_bucket[5m])) by (le))", LegendFormat: "p99"},
		}, 12, 4, 12, 8),
	)
	
	// Row 3: Rule Performance
	dashboard.Panels = append(dashboard.Panels,
		g.createTablePanel(7, "Top Slow Rules", "topk(10, avg_over_time(ag_ui_rule_avg_duration[5m])) by (rule_id)", 0, 12, 12, 8),
		g.createHeatmapPanel(8, "Rule Execution Heatmap", "sum(increase(ag_ui_events_rule_execution_duration_seconds_bucket[5m])) by (le, rule_id)", 12, 12, 12, 8),
	)
	
	// Row 4: System Health
	dashboard.Panels = append(dashboard.Panels,
		g.createTimeSeriesPanel(9, "Memory Usage", []Target{
			{RefID: "A", Expr: "ag_ui_memory_allocated_bytes", LegendFormat: "Allocated"},
			{RefID: "B", Expr: "ag_ui_memory_heap_objects", LegendFormat: "Heap Objects"},
		}, 0, 20, 12, 8),
		g.createGaugePanel(10, "Memory Pressure", "ag_ui_memory_allocated_bytes / 1024 / 1024 / 1024", 12, 20, 6, 8),
		g.createStatPanel(11, "GC Cycles", "increase(ag_ui_gc_cycles_total[5m])", 18, 20, 6, 8),
	)
	
	return &GrafanaDashboard{
		Dashboard: *dashboard,
		FolderID:  0,
		Overwrite: true,
	}, nil
}

// GenerateRulePerformanceDashboard generates the rule performance dashboard
func (g *GrafanaDashboardGenerator) GenerateRulePerformanceDashboard() (*GrafanaDashboard, error) {
	dashboard := &DashboardJSON{
		Title:         "Rule Performance Analysis",
		Tags:          []string{"events", "rules", "performance"},
		Timezone:      "browser",
		SchemaVersion: 26,
		Version:       1,
		Time: TimeRange{
			From: "now-3h",
			To:   "now",
		},
		Refresh: "1m",
		Panels:  []Panel{},
		Variables: []Variable{
			{
				Name:  "rule_id",
				Type:  "query",
				Label: "Rule ID",
				Query: "label_values(ag_ui_rule_execution_count, rule_id)",
				Datasource: Datasource{
					Type: "prometheus",
					UID:  "prometheus",
				},
				Multi:      true,
				IncludeAll: true,
			},
		},
	}
	
	// Rule-specific panels
	dashboard.Panels = append(dashboard.Panels,
		g.createTimeSeriesPanel(1, "Rule Execution Duration", []Target{
			{RefID: "A", Expr: "ag_ui_rule_avg_duration{rule_id=~\"$rule_id\"}", LegendFormat: "{{rule_id}}"},
		}, 0, 0, 24, 8),
		g.createStatPanel(2, "Total Executions", "sum(ag_ui_rule_execution_count{rule_id=~\"$rule_id\"})", 0, 8, 8, 4),
		g.createStatPanel(3, "Average Duration", "avg(ag_ui_rule_avg_duration{rule_id=~\"$rule_id\"})", 8, 8, 8, 4),
		g.createStatPanel(4, "Error Rate", "avg(ag_ui_rule_error_rate{rule_id=~\"$rule_id\"})", 16, 8, 8, 4),
	)
	
	return &GrafanaDashboard{
		Dashboard: *dashboard,
		FolderID:  0,
		Overwrite: true,
	}, nil
}

// GenerateSLAComplianceDashboard generates the SLA compliance dashboard
func (g *GrafanaDashboardGenerator) GenerateSLAComplianceDashboard() (*GrafanaDashboard, error) {
	dashboard := &DashboardJSON{
		Title:         "SLA Compliance Monitoring",
		Tags:          []string{"sla", "compliance", "monitoring"},
		Timezone:      "browser",
		SchemaVersion: 26,
		Version:       1,
		Time: TimeRange{
			From: "now-24h",
			To:   "now",
		},
		Refresh: "5m",
		Panels:  []Panel{},
	}
	
	// SLA panels
	dashboard.Panels = append(dashboard.Panels,
		g.createTablePanel(1, "SLA Status", "ag_ui_sla_compliance", 0, 0, 24, 8),
		g.createTimeSeriesPanel(2, "SLA Trends", []Target{
			{RefID: "A", Expr: "ag_ui_sla_current_value", LegendFormat: "{{sla_name}}"},
		}, 0, 8, 24, 10),
		g.createStatPanel(3, "Overall Compliance", "avg(ag_ui_sla_compliance)", 0, 18, 8, 6),
		g.createStatPanel(4, "Active Violations", "count(ag_ui_sla_compliance == 0)", 8, 18, 8, 6),
		g.createStatPanel(5, "At Risk SLAs", "count(ag_ui_sla_current_value > ag_ui_sla_target_value * 0.9)", 16, 18, 8, 6),
	)
	
	return &GrafanaDashboard{
		Dashboard: *dashboard,
		FolderID:  0,
		Overwrite: true,
	}, nil
}

// GenerateSystemHealthDashboard generates the system health dashboard
func (g *GrafanaDashboardGenerator) GenerateSystemHealthDashboard() (*GrafanaDashboard, error) {
	dashboard := &DashboardJSON{
		Title:         "System Health Monitor",
		Tags:          []string{"system", "health", "resources"},
		Timezone:      "browser",
		SchemaVersion: 26,
		Version:       1,
		Time: TimeRange{
			From: "now-12h",
			To:   "now",
		},
		Refresh: "1m",
		Panels:  []Panel{},
	}
	
	// System health panels
	dashboard.Panels = append(dashboard.Panels,
		g.createTimeSeriesPanel(1, "Memory Usage Trend", []Target{
			{RefID: "A", Expr: "ag_ui_memory_allocated_bytes", LegendFormat: "Allocated"},
			{RefID: "B", Expr: "ag_ui_memory_heap_objects * 32", LegendFormat: "Est. Heap Size"},
		}, 0, 0, 12, 8),
		g.createTimeSeriesPanel(2, "GC Activity", []Target{
			{RefID: "A", Expr: "rate(ag_ui_gc_cycles_total[5m])", LegendFormat: "GC Rate"},
			{RefID: "B", Expr: "rate(ag_ui_gc_pause_total_seconds[5m])", LegendFormat: "GC Pause Rate"},
		}, 12, 0, 12, 8),
		g.createAlertListPanel(3, "Active Alerts", 0, 8, 24, 6),
		g.createLogPanel(4, "Recent Warnings", "{component=\"event-validation\",level=\"warning\"}", 0, 14, 24, 10),
	)
	
	return &GrafanaDashboard{
		Dashboard: *dashboard,
		FolderID:  0,
		Overwrite: true,
	}, nil
}

// Helper methods for creating panels

func (g *GrafanaDashboardGenerator) createStatPanel(id int, title, expr string, x, y, w, h int) Panel {
	return Panel{
		ID:      id,
		Type:    "stat",
		Title:   title,
		GridPos: GridPos{X: x, Y: y, W: w, H: h},
		Datasource: Datasource{
			Type: "prometheus",
			UID:  "prometheus",
		},
		Targets: []Target{
			{RefID: "A", Expr: expr},
		},
		FieldConfig: FieldConfig{
			Defaults: FieldDefaults{
				Mappings: []interface{}{},
				Thresholds: ThresholdConfig{
					Mode: "absolute",
					Steps: []Threshold{
						{Color: "green", Value: nil},
						{Color: "red", Value: floatPtr(80)},
					},
				},
			},
		},
	}
}

func (g *GrafanaDashboardGenerator) createTimeSeriesPanel(id int, title string, targets []Target, x, y, w, h int) Panel {
	return Panel{
		ID:      id,
		Type:    "timeseries",
		Title:   title,
		GridPos: GridPos{X: x, Y: y, W: w, H: h},
		Datasource: Datasource{
			Type: "prometheus",
			UID:  "prometheus",
		},
		Targets: targets,
		FieldConfig: FieldConfig{
			Defaults: FieldDefaults{
				Color: ColorConfig{
					Mode: "palette-classic",
				},
				Custom: map[string]interface{}{
					"axisLabel":     "",
					"axisPlacement": "auto",
					"barAlignment":  0,
					"drawStyle":     "line",
					"fillOpacity":   10,
					"gradientMode":  "none",
					"hideFrom": map[string]bool{
						"tooltip":  false,
						"viz":      false,
						"legend":   false,
					},
					"lineInterpolation": "linear",
					"lineWidth":         1,
					"pointSize":         5,
					"scaleDistribution": map[string]string{
						"type": "linear",
					},
					"showPoints":        "never",
					"spanNulls":         true,
					"stacking":          map[string]interface{}{"group": "A", "mode": false},
					"thresholdsStyle":   map[string]string{"mode": "off"},
				},
				Mappings: []interface{}{},
				Thresholds: ThresholdConfig{
					Mode:  "absolute",
					Steps: []Threshold{{Color: "green", Value: nil}},
				},
			},
		},
	}
}

func (g *GrafanaDashboardGenerator) createTablePanel(id int, title, expr string, x, y, w, h int) Panel {
	return Panel{
		ID:      id,
		Type:    "table",
		Title:   title,
		GridPos: GridPos{X: x, Y: y, W: w, H: h},
		Datasource: Datasource{
			Type: "prometheus",
			UID:  "prometheus",
		},
		Targets: []Target{
			{RefID: "A", Expr: expr, Interval: "5m"},
		},
	}
}

func (g *GrafanaDashboardGenerator) createHeatmapPanel(id int, title, expr string, x, y, w, h int) Panel {
	return Panel{
		ID:      id,
		Type:    "heatmap",
		Title:   title,
		GridPos: GridPos{X: x, Y: y, W: w, H: h},
		Datasource: Datasource{
			Type: "prometheus",
			UID:  "prometheus",
		},
		Targets: []Target{
			{RefID: "A", Expr: expr},
		},
		Options: map[string]interface{}{
			"calculate": false,
			"cellGap":   2,
			"cellRadius": 2,
			"color": map[string]interface{}{
				"exponent": 0.5,
				"fill":     "dark-orange",
				"mode":     "scheme",
				"scale":    "exponential",
				"scheme":   "Oranges",
				"steps":    64,
			},
			"exemplars": map[string]interface{}{
				"color": "rgba(255,0,255,0.7)",
			},
			"filterValues": map[string]interface{}{
				"le": 1e-9,
			},
			"legend": map[string]interface{}{
				"show": true,
			},
			"rowsFrame": map[string]interface{}{
				"layout": "auto",
			},
			"tooltip": map[string]interface{}{
				"show":       true,
				"yHistogram": false,
			},
			"yAxis": map[string]interface{}{
				"axisPlacement": "left",
				"reverse":       false,
			},
		},
	}
}

func (g *GrafanaDashboardGenerator) createGaugePanel(id int, title, expr string, x, y, w, h int) Panel {
	return Panel{
		ID:      id,
		Type:    "gauge",
		Title:   title,
		GridPos: GridPos{X: x, Y: y, W: w, H: h},
		Datasource: Datasource{
			Type: "prometheus",
			UID:  "prometheus",
		},
		Targets: []Target{
			{RefID: "A", Expr: expr},
		},
		FieldConfig: FieldConfig{
			Defaults: FieldDefaults{
				Color: ColorConfig{
					Mode: "thresholds",
				},
				Mappings: []interface{}{},
				Thresholds: ThresholdConfig{
					Mode: "absolute",
					Steps: []Threshold{
						{Color: "green", Value: nil},
						{Color: "yellow", Value: floatPtr(70)},
						{Color: "red", Value: floatPtr(90)},
					},
				},
				Unit: "GB",
				Min:  floatPtr(0),
				Max:  floatPtr(100),
			},
		},
	}
}

func (g *GrafanaDashboardGenerator) createAlertListPanel(id int, title string, x, y, w, h int) Panel {
	return Panel{
		ID:      id,
		Type:    "alertlist",
		Title:   title,
		GridPos: GridPos{X: x, Y: y, W: w, H: h},
		Options: map[string]interface{}{
			"alertName":   "",
			"dashboardAlerts": false,
			"groupBy":        []string{},
			"groupMode":      "default",
			"maxItems":       20,
			"sortOrder":      1,
			"stateFilter": map[string]bool{
				"alerting":        true,
				"pending":         true,
				"noData":          false,
				"noDataState":     false,
				"executionError":  false,
			},
		},
	}
}

func (g *GrafanaDashboardGenerator) createLogPanel(id int, title, query string, x, y, w, h int) Panel {
	return Panel{
		ID:      id,
		Type:    "logs",
		Title:   title,
		GridPos: GridPos{X: x, Y: y, W: w, H: h},
		Datasource: Datasource{
			Type: "loki",
			UID:  "loki",
		},
		Targets: []Target{
			{RefID: "A", Expr: query},
		},
		Options: map[string]interface{}{
			"dedupStrategy":        "none",
			"enableLogDetails":     true,
			"prettifyLogMessage":   false,
			"showCommonLabels":     false,
			"showLabels":           false,
			"showTime":             true,
			"sortOrder":            "Descending",
			"wrapLogMessage":       false,
		},
	}
}

// UploadDashboard uploads a dashboard to Grafana
func (g *GrafanaDashboardGenerator) UploadDashboard(dashboard *GrafanaDashboard) error {
	if g.config.GrafanaURL == "" || g.config.GrafanaAPIKey == "" {
		return fmt.Errorf("Grafana URL or API key not configured")
	}
	
	data, err := json.Marshal(dashboard)
	if err != nil {
		return fmt.Errorf("failed to marshal dashboard: %w", err)
	}
	
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/dashboards/db", g.config.GrafanaURL), bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", g.config.GrafanaAPIKey))
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload dashboard: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to upload dashboard: status %d", resp.StatusCode)
	}
	
	return nil
}

// Helper function to create float64 pointer
func floatPtr(f float64) *float64 {
	return &f
}