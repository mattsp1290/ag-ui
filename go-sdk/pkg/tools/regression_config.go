package tools

import (
	"time"
)

// RegressionConfig configures regression testing parameters
type RegressionConfig struct {
	// Baseline configuration
	BaselineStrategy       RegressionBaselineStrategy
	BaselineStorage        string
	BaselineRetentionDays  int
	BaselineWindow         time.Duration
	
	// Detection configuration
	DetectionAlgorithms    []RegressionDetectionAlgorithm
	DetectionThresholds    *RegressionDetectionThresholds
	StatisticalConfidence  float64
	MinimumSampleSize      int
	
	// Analysis configuration
	AnalysisDepth          RegressionAnalysisDepth
	TrendAnalysisWindow    time.Duration
	SeasonalityDetection   bool
	OutlierDetection       bool
	
	// Reporting configuration
	ReportDetailLevel      RegressionReportDetailLevel
	ReportFormats          []string
	ReportOutputDir        string
	
	// Alert configuration
	AlertThresholds        *RegressionAlertThresholds
	AlertChannels          []RegressionAlertChannel
	AlertsEnabled          bool
	
	// Test configuration
	TestEnvironment        string
	TestLabels             map[string]string
	MetricsToTrack         []string
	CustomMetrics          map[string]MetricConfig
	
	// Quality gates
	QualityGates           []RegressionQualityGate
	FailOnRegression       bool
	FailOnDegradation      bool
	
	// Advanced configuration
	AnomalyDetection       bool
	PredictiveAnalysis     bool
	ModelUpdateInterval    time.Duration
	HistoricalDataLimit    int
}

// RegressionBaselineStrategy defines how baselines are managed
type RegressionBaselineStrategy string

const (
	RegressionBaselineStrategyFixed      RegressionBaselineStrategy = "fixed"
	RegressionBaselineStrategyRolling    RegressionBaselineStrategy = "rolling"
	RegressionBaselineStrategyAdaptive   RegressionBaselineStrategy = "adaptive"
	RegressionBaselineStrategyStatistical RegressionBaselineStrategy = "statistical"
)

// RegressionDetectionAlgorithm defines different regression detection algorithms
type RegressionDetectionAlgorithm string

const (
	RegressionAlgorithmThreshold      RegressionDetectionAlgorithm = "threshold"
	RegressionAlgorithmStatistical    RegressionDetectionAlgorithm = "statistical"
	RegressionAlgorithmTrend          RegressionDetectionAlgorithm = "trend"
	RegressionAlgorithmChangePoint    RegressionDetectionAlgorithm = "changepoint"
	RegressionAlgorithmAnomaly        RegressionDetectionAlgorithm = "anomaly"
	RegressionAlgorithmMachineLearning RegressionDetectionAlgorithm = "ml"
)

// RegressionAnalysisDepth defines the depth of regression analysis
type RegressionAnalysisDepth string

const (
	RegressionAnalysisDepthBasic       RegressionAnalysisDepth = "basic"
	RegressionAnalysisDepthStandard    RegressionAnalysisDepth = "standard"
	RegressionAnalysisDepthDetailed    RegressionAnalysisDepth = "detailed"
	RegressionAnalysisDepthComprehensive RegressionAnalysisDepth = "comprehensive"
)

// RegressionReportDetailLevel defines the detail level of regression reports
type RegressionReportDetailLevel string

const (
	RegressionReportDetailLevelSummary  RegressionReportDetailLevel = "summary"
	RegressionReportDetailLevelStandard RegressionReportDetailLevel = "standard"
	RegressionReportDetailLevelDetailed RegressionReportDetailLevel = "detailed"
	RegressionReportDetailLevelVerbose  RegressionReportDetailLevel = "verbose"
)

// RegressionDetectionThresholds defines thresholds for regression detection
type RegressionDetectionThresholds struct {
	// Percentage thresholds
	PerformanceDegradation float64
	ThroughputDecrease     float64
	ResponseTimeIncrease   float64
	ErrorRateIncrease      float64
	MemoryUsageIncrease    float64
	
	// Statistical thresholds
	StatisticalSignificance float64
	ConfidenceLevel         float64
	MinimumEffectSize       float64
	
	// Trend thresholds
	TrendSignificance       float64
	TrendDuration          time.Duration
	TrendConsistency       float64
	
	// Anomaly thresholds
	AnomalyScore           float64
	AnomalyDeviation       float64
	AnomalyFrequency       float64
}

// RegressionAlertThresholds defines when to trigger alerts
type RegressionAlertThresholds struct {
	CriticalRegression     float64
	MajorRegression        float64
	MinorRegression        float64
	WarningRegression      float64
	
	CriticalAnomaly        float64
	MajorAnomaly           float64
	MinorAnomaly           float64
	
	TrendDegradation       float64
	ConsistentDegradation  float64
}

// RegressionAlertChannel defines alert notification channels
type RegressionAlertChannel struct {
	Type     string
	Config   map[string]string
	Enabled  bool
	Filters  []string
}

// MetricConfig defines configuration for custom metrics
type MetricConfig struct {
	Name        string
	Type        string
	Aggregation string
	Thresholds  map[string]float64
	Weight      float64
}

// RegressionQualityGate defines quality gates for regression testing
type RegressionQualityGate struct {
	Name        string
	Metric      string
	Threshold   float64
	Operator    string
	Severity    TestRegressionSeverity
	Enabled     bool
}

// TestRegressionSeverity defines severity levels for regressions in tests
type TestRegressionSeverity string

const (
	TestRegressionSeverityInfo     TestRegressionSeverity = "info"
	TestRegressionSeverityWarning  TestRegressionSeverity = "warning"
	TestRegressionSeverityMajor    TestRegressionSeverity = "major"
	TestRegressionSeverityCritical TestRegressionSeverity = "critical"
)

// DefaultRegressionConfig returns default regression configuration
func DefaultRegressionConfig() *RegressionConfig {
	return &RegressionConfig{
		BaselineStrategy:      RegressionBaselineStrategyRolling,
		BaselineStorage:       "filesystem",
		BaselineRetentionDays: 30,
		BaselineWindow:        7 * 24 * time.Hour,
		DetectionAlgorithms: []RegressionDetectionAlgorithm{
			RegressionAlgorithmThreshold,
			RegressionAlgorithmStatistical,
			RegressionAlgorithmTrend,
		},
		DetectionThresholds: &RegressionDetectionThresholds{
			PerformanceDegradation:  10.0,
			ThroughputDecrease:      5.0,
			ResponseTimeIncrease:    15.0,
			ErrorRateIncrease:       2.0,
			MemoryUsageIncrease:     20.0,
			StatisticalSignificance: 0.05,
			ConfidenceLevel:         0.95,
			MinimumEffectSize:       0.2,
			TrendSignificance:       0.01,
			TrendDuration:          24 * time.Hour,
			TrendConsistency:       0.8,
			AnomalyScore:           0.7,
			AnomalyDeviation:       2.0,
			AnomalyFrequency:       0.1,
		},
		StatisticalConfidence: 0.95,
		MinimumSampleSize:     10,
		AnalysisDepth:         RegressionAnalysisDepthStandard,
		TrendAnalysisWindow:   24 * time.Hour,
		SeasonalityDetection:  true,
		OutlierDetection:      true,
		ReportDetailLevel:     RegressionReportDetailLevelStandard,
		ReportFormats:         []string{"json", "html"},
		ReportOutputDir:       "./regression-reports",
		AlertThresholds: &RegressionAlertThresholds{
			CriticalRegression:    25.0,
			MajorRegression:       15.0,
			MinorRegression:       10.0,
			WarningRegression:     5.0,
			CriticalAnomaly:       0.9,
			MajorAnomaly:          0.7,
			MinorAnomaly:          0.5,
			TrendDegradation:      0.8,
			ConsistentDegradation: 0.6,
		},
		AlertsEnabled:         true,
		TestEnvironment:       "test",
		TestLabels:            make(map[string]string),
		MetricsToTrack: []string{
			"throughput",
			"response_time",
			"error_rate",
			"memory_usage",
			"cpu_usage",
		},
		QualityGates: []RegressionQualityGate{
			{
				Name:     "Performance Degradation",
				Metric:   "performance_degradation",
				Threshold: 10.0,
				Operator: "lt",
				Severity: TestRegressionSeverityMajor,
				Enabled:  true,
			},
			{
				Name:     "Error Rate Increase",
				Metric:   "error_rate_increase",
				Threshold: 2.0,
				Operator: "lt",
				Severity: TestRegressionSeverityCritical,
				Enabled:  true,
			},
		},
		FailOnRegression:     true,
		FailOnDegradation:    true,
		AnomalyDetection:     true,
		PredictiveAnalysis:   false,
		ModelUpdateInterval:  24 * time.Hour,
		HistoricalDataLimit:  1000,
	}
}