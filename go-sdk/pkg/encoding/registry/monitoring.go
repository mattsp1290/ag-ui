package registry

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// MemoryPressureLevel represents different levels of memory pressure
type MemoryPressureLevel int

const (
	MemoryPressureLow MemoryPressureLevel = iota + 1
	MemoryPressureMedium
	MemoryPressureHigh
)

// String returns a string representation of the memory pressure level
func (mpl MemoryPressureLevel) String() string {
	switch mpl {
	case MemoryPressureLow:
		return "low"
	case MemoryPressureMedium:
		return "medium"
	case MemoryPressureHigh:
		return "high"
	default:
		return "unknown"
	}
}

// RegistryHealthReport provides comprehensive health information about the registry
type RegistryHealthReport struct {
	Timestamp              time.Time              `json:"timestamp"`
	OverallHealth          string                 `json:"overall_health"`
	MemoryPressureLevel    MemoryPressureLevel    `json:"memory_pressure_level"`
	MemoryPressurePercent  int                    `json:"memory_pressure_percent"`
	Stats                  map[string]interface{} `json:"stats"`
	Recommendations        []string               `json:"recommendations"`
	AlertConditions        []string               `json:"alert_conditions,omitempty"`
	ConfigurationAnalysis  ConfigAnalysis         `json:"configuration_analysis"`
}

// ConfigAnalysis provides analysis of the registry configuration
type ConfigAnalysis struct {
	IsOptimal                bool     `json:"is_optimal"`
	Issues                   []string `json:"issues,omitempty"`
	Suggestions              []string `json:"suggestions,omitempty"`
	MemoryEfficiencyRating   string   `json:"memory_efficiency_rating"`
	PerformanceRating        string   `json:"performance_rating"`
}

// GenerateHealthReport creates a comprehensive health report for the registry
func (r *CoreRegistry) GenerateHealthReport() *RegistryHealthReport {
	stats := r.GetRegistryStats()
	memoryPressurePercent := stats["memory_pressure_percent"].(int)
	isUnderPressure := stats["is_under_memory_pressure"].(bool)
	
	// Determine memory pressure level
	var pressureLevel MemoryPressureLevel
	if memoryPressurePercent >= 90 || isUnderPressure {
		pressureLevel = MemoryPressureHigh
	} else if memoryPressurePercent >= 80 {
		pressureLevel = MemoryPressureMedium
	} else {
		pressureLevel = MemoryPressureLow
	}
	
	// Determine overall health
	overallHealth := "healthy"
	if pressureLevel >= MemoryPressureHigh {
		overallHealth = "critical"
	} else if pressureLevel >= MemoryPressureMedium {
		overallHealth = "warning"
	}
	
	// Generate recommendations and alerts
	recommendations := r.generateRecommendations(stats, pressureLevel)
	alerts := r.generateAlerts(stats, pressureLevel)
	
	// Analyze configuration
	configAnalysis := r.analyzeConfiguration()
	
	return &RegistryHealthReport{
		Timestamp:              time.Now(),
		OverallHealth:          overallHealth,
		MemoryPressureLevel:    pressureLevel,
		MemoryPressurePercent:  memoryPressurePercent,
		Stats:                  stats,
		Recommendations:        recommendations,
		AlertConditions:        alerts,
		ConfigurationAnalysis:  configAnalysis,
	}
}

// generateRecommendations provides actionable recommendations based on registry state
func (r *CoreRegistry) generateRecommendations(stats map[string]interface{}, pressureLevel MemoryPressureLevel) []string {
	var recommendations []string
	
	memoryPressurePercent := stats["memory_pressure_percent"].(int)
	totalEntries := stats["total_entries"].(int)
	cleanupOperations := stats["cleanup_operations"].(int64)
	totalCleaned := stats["total_cleaned"].(int64)
	
	// Memory pressure recommendations
	if pressureLevel >= MemoryPressureMedium {
		recommendations = append(recommendations, "Consider reducing TTL or MaxEntries to manage memory usage")
	}
	
	if pressureLevel >= MemoryPressureHigh {
		recommendations = append(recommendations, "Immediate action needed: Trigger manual cleanup or restart registry")
	}
	
	// Activity recommendations
	if cleanupOperations > 0 && totalCleaned == 0 {
		recommendations = append(recommendations, "Cleanup operations are running but not removing entries - check TTL configuration")
	}
	
	if totalEntries > 1500 && r.config.MaxEntries == 0 {
		recommendations = append(recommendations, "Consider setting MaxEntries limit to prevent unbounded growth")
	}
	
	// Configuration recommendations
	if r.config.CleanupInterval > 30*time.Minute {
		recommendations = append(recommendations, "Consider reducing cleanup interval for more frequent maintenance")
	}
	
	if !r.config.EnableLRU && memoryPressurePercent > 70 {
		recommendations = append(recommendations, "Enable LRU eviction to better manage memory under pressure")
	}
	
	return recommendations
}

// generateAlerts identifies critical conditions that require immediate attention
func (r *CoreRegistry) generateAlerts(stats map[string]interface{}, pressureLevel MemoryPressureLevel) []string {
	var alerts []string
	
	memoryPressurePercent := stats["memory_pressure_percent"].(int)
	maxEntriesReachedCount := stats["max_entries_reached_count"].(int64)
	totalEntries := stats["total_entries"].(int)
	
	// Critical memory conditions
	if memoryPressurePercent >= 95 {
		alerts = append(alerts, "CRITICAL: Memory pressure at 95%+ - immediate intervention required")
	}
	
	if pressureLevel >= MemoryPressureHigh && maxEntriesReachedCount > 10 {
		alerts = append(alerts, "ALERT: MaxEntries limit reached multiple times - potential memory leak")
	}
	
	// Potential memory leak detection
	if totalEntries > 5000 && r.config.MaxEntries == 0 {
		alerts = append(alerts, "WARNING: Very high entry count with no limits - possible memory leak")
	}
	
	// Configuration issues
	if !r.config.EnableBackgroundCleanup && totalEntries > 1000 {
		alerts = append(alerts, "WARNING: Background cleanup disabled with high entry count")
	}
	
	return alerts
}

// analyzeConfiguration provides analysis of the current configuration
func (r *CoreRegistry) analyzeConfiguration() ConfigAnalysis {
	analysis := ConfigAnalysis{
		Issues:                 []string{},
		Suggestions:            []string{},
		MemoryEfficiencyRating: "good",
		PerformanceRating:      "good",
	}
	
	// Analyze memory efficiency
	if r.config.MaxEntries == 0 {
		analysis.Issues = append(analysis.Issues, "No MaxEntries limit set - potential for unbounded growth")
		analysis.MemoryEfficiencyRating = "poor"
	}
	
	if r.config.TTL == 0 {
		analysis.Issues = append(analysis.Issues, "No TTL set - entries never expire")
		if analysis.MemoryEfficiencyRating != "poor" {
			analysis.MemoryEfficiencyRating = "fair"
		}
	}
	
	// Analyze performance configuration
	if !r.config.EnableLRU {
		analysis.Issues = append(analysis.Issues, "LRU eviction disabled - less efficient memory management")
		analysis.PerformanceRating = "fair"
	}
	
	if r.config.CleanupInterval > 15*time.Minute {
		analysis.Suggestions = append(analysis.Suggestions, "Consider shorter cleanup interval for better memory management")
	}
	
	// Determine if configuration is optimal
	analysis.IsOptimal = len(analysis.Issues) == 0 && 
		r.config.MaxEntries > 0 && r.config.MaxEntries <= 2000 &&
		r.config.TTL > 0 && r.config.TTL <= 2*time.Hour &&
		r.config.EnableLRU && r.config.EnableBackgroundCleanup
	
	return analysis
}

// LogHealthReport logs a formatted health report
func (r *CoreRegistry) LogHealthReport() {
	report := r.GenerateHealthReport()
	
	log.Printf("Registry Health Report - Status: %s, Memory Pressure: %s (%d%%)",
		report.OverallHealth, report.MemoryPressureLevel, report.MemoryPressurePercent)
	
	if len(report.AlertConditions) > 0 {
		for _, alert := range report.AlertConditions {
			log.Printf("REGISTRY ALERT: %s", alert)
		}
	}
	
	if len(report.Recommendations) > 0 {
		log.Printf("Registry recommendations: %d items", len(report.Recommendations))
		for i, rec := range report.Recommendations {
			log.Printf("  %d. %s", i+1, rec)
		}
	}
}

// GetHealthReportJSON returns the health report as JSON string
func (r *CoreRegistry) GetHealthReportJSON() (string, error) {
	report := r.GenerateHealthReport()
	jsonBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal health report: %w", err)
	}
	return string(jsonBytes), nil
}

// PerformHealthCheck runs a comprehensive health check and returns the results
func (r *CoreRegistry) PerformHealthCheck() (bool, []string) {
	report := r.GenerateHealthReport()
	
	isHealthy := report.OverallHealth == "healthy"
	issues := append(report.AlertConditions, report.ConfigurationAnalysis.Issues...)
	
	return isHealthy, issues
}