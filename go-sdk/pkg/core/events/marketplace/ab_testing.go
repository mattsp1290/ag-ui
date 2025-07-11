package marketplace

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"sync"
	"time"
)

// ABTesting manages A/B testing for validation rules
type ABTesting struct {
	experiments    map[string]*Experiment
	participations map[string]*Participation // userID -> participation
	metrics        *MetricsCollector
	allocator      *TrafficAllocator
	analyzer       *StatisticalAnalyzer
	mu             sync.RWMutex
}

// Experiment represents an A/B test experiment
type Experiment struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Status      ExperimentStatus       `json:"status"`
	Type        ExperimentType         `json:"type"`
	
	// Configuration
	StartDate    time.Time              `json:"start_date"`
	EndDate      time.Time              `json:"end_date"`
	MinDuration  time.Duration          `json:"min_duration"`
	MaxDuration  time.Duration          `json:"max_duration"`
	
	// Traffic allocation
	TrafficAllocation float64           `json:"traffic_allocation"` // 0.0 to 1.0
	TargetCriteria    *TargetCriteria   `json:"target_criteria"`
	
	// Variants
	Variants         []*Variant         `json:"variants"`
	ControlVariant   string             `json:"control_variant"`
	
	// Metrics
	PrimaryMetric    string             `json:"primary_metric"`
	SecondaryMetrics []string           `json:"secondary_metrics"`
	
	// Statistical configuration
	StatisticalPower   float64          `json:"statistical_power"`    // typically 0.8
	SignificanceLevel  float64          `json:"significance_level"`   // typically 0.05
	MinimumSampleSize  int              `json:"minimum_sample_size"`
	
	// Results
	Results          *ExperimentResults `json:"results,omitempty"`
	
	// Metadata
	CreatedBy        string             `json:"created_by"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
	Tags             []string           `json:"tags"`
	Metadata         map[string]interface{} `json:"metadata"`
}

// ExperimentStatus defines the status of an experiment
type ExperimentStatus string

const (
	StatusDraft    ExperimentStatus = "draft"
	StatusRunning  ExperimentStatus = "running"
	StatusPaused   ExperimentStatus = "paused"
	StatusCompleted ExperimentStatus = "completed"
	StatusCancelled ExperimentStatus = "cancelled"
)

// ExperimentType defines the type of experiment
type ExperimentType string

const (
	TypeRuleComparison  ExperimentType = "rule_comparison"
	TypeConfigTest      ExperimentType = "config_test"
	TypePerformanceTest ExperimentType = "performance_test"
	TypeUserExperience  ExperimentType = "user_experience"
)

// Variant represents a variant in an A/B test
type Variant struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Weight      float64                `json:"weight"` // 0.0 to 1.0
	IsControl   bool                   `json:"is_control"`
	
	// Rule configuration
	RulePackageID string                 `json:"rule_package_id"`
	RuleVersion   string                 `json:"rule_version"`
	RuleConfig    map[string]interface{} `json:"rule_config"`
	
	// Feature flags
	FeatureFlags  map[string]bool        `json:"feature_flags"`
	
	// Metadata
	Metadata      map[string]interface{} `json:"metadata"`
}

// TargetCriteria defines who should be included in the experiment
type TargetCriteria struct {
	UserSegments    []string           `json:"user_segments"`
	Countries       []string           `json:"countries"`
	Platforms       []string           `json:"platforms"`
	UserTypes       []string           `json:"user_types"`
	MinUserAge      int                `json:"min_user_age"`
	MaxUserAge      int                `json:"max_user_age"`
	CustomFilters   map[string]interface{} `json:"custom_filters"`
	ExcludePrevious bool               `json:"exclude_previous"` // Exclude users from previous experiments
}

// Participation represents a user's participation in experiments
type Participation struct {
	UserID       string                      `json:"user_id"`
	Experiments  map[string]*UserExperiment  `json:"experiments"` // experimentID -> user experiment
	JoinedAt     time.Time                   `json:"joined_at"`
	LastActivity time.Time                   `json:"last_activity"`
	Segments     []string                    `json:"segments"`
	Properties   map[string]interface{}      `json:"properties"`
}

// UserExperiment represents a user's participation in a specific experiment
type UserExperiment struct {
	ExperimentID string                 `json:"experiment_id"`
	VariantID    string                 `json:"variant_id"`
	AssignedAt   time.Time              `json:"assigned_at"`
	FirstSeen    time.Time              `json:"first_seen"`
	LastSeen     time.Time              `json:"last_seen"`
	EventCount   int                    `json:"event_count"`
	Conversions  int                    `json:"conversions"`
	Revenue      float64                `json:"revenue"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// ExperimentResults contains the results of an experiment
type ExperimentResults struct {
	ExperimentID     string                         `json:"experiment_id"`
	Status           ResultStatus                   `json:"status"`
	Winner           string                         `json:"winner,omitempty"`
	Confidence       float64                        `json:"confidence"`
	PValue           float64                        `json:"p_value"`
	EffectSize       float64                        `json:"effect_size"`
	
	// Variant results
	VariantResults   map[string]*VariantResult      `json:"variant_results"`
	
	// Statistical tests
	StatisticalTests []*StatisticalTest             `json:"statistical_tests"`
	
	// Timing
	StartedAt        time.Time                      `json:"started_at"`
	CompletedAt      *time.Time                     `json:"completed_at,omitempty"`
	Duration         time.Duration                  `json:"duration"`
	
	// Metadata
	GeneratedAt      time.Time                      `json:"generated_at"`
	GeneratedBy      string                         `json:"generated_by"`
}

// ResultStatus defines the status of experiment results
type ResultStatus string

const (
	ResultInProgress    ResultStatus = "in_progress"
	ResultSignificant   ResultStatus = "significant"
	ResultInsignificant ResultStatus = "insignificant"
	ResultInconclusive  ResultStatus = "inconclusive"
)

// VariantResult contains results for a specific variant
type VariantResult struct {
	VariantID        string                 `json:"variant_id"`
	Participants     int                    `json:"participants"`
	Conversions      int                    `json:"conversions"`
	ConversionRate   float64                `json:"conversion_rate"`
	Revenue          float64                `json:"revenue"`
	RevenuePerUser   float64                `json:"revenue_per_user"`
	ConfidenceInterval *ConfidenceInterval  `json:"confidence_interval"`
	Metrics          map[string]float64     `json:"metrics"`
}

// ConfidenceInterval represents a confidence interval
type ConfidenceInterval struct {
	Lower      float64 `json:"lower"`
	Upper      float64 `json:"upper"`
	Confidence float64 `json:"confidence"`
}

// StatisticalTest represents a statistical test performed
type StatisticalTest struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	PValue      float64                `json:"p_value"`
	Statistic   float64                `json:"statistic"`
	DegreesOfFreedom int               `json:"degrees_of_freedom,omitempty"`
	Result      string                 `json:"result"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// MetricsCollector collects and aggregates experiment metrics
type MetricsCollector struct {
	events    []ExperimentEvent
	metrics   map[string]*MetricAggregation
	mu        sync.RWMutex
}

// ExperimentEvent represents an event in an experiment
type ExperimentEvent struct {
	ID           string                 `json:"id"`
	ExperimentID string                 `json:"experiment_id"`
	VariantID    string                 `json:"variant_id"`
	UserID       string                 `json:"user_id"`
	EventType    string                 `json:"event_type"`
	Timestamp    time.Time              `json:"timestamp"`
	Properties   map[string]interface{} `json:"properties"`
	Value        float64                `json:"value"`
}

// MetricAggregation contains aggregated metrics
type MetricAggregation struct {
	MetricName   string                 `json:"metric_name"`
	Values       []float64              `json:"values"`
	Count        int                    `json:"count"`
	Sum          float64                `json:"sum"`
	Mean         float64                `json:"mean"`
	Median       float64                `json:"median"`
	StdDev       float64                `json:"std_dev"`
	Min          float64                `json:"min"`
	Max          float64                `json:"max"`
	Percentiles  map[string]float64     `json:"percentiles"`
}

// TrafficAllocator manages traffic allocation for experiments
type TrafficAllocator struct {
	hashSalt string
	mu       sync.RWMutex
}

// StatisticalAnalyzer performs statistical analysis on experiment data
type StatisticalAnalyzer struct {
	tests map[string]StatisticalTestFunc
}

// StatisticalTestFunc defines a statistical test function
type StatisticalTestFunc func(control, variant *VariantResult) *StatisticalTest

// NewABTesting creates a new A/B testing system
func NewABTesting() *ABTesting {
	return &ABTesting{
		experiments:    make(map[string]*Experiment),
		participations: make(map[string]*Participation),
		metrics:        NewMetricsCollector(),
		allocator:      NewTrafficAllocator(),
		analyzer:       NewStatisticalAnalyzer(),
	}
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		events:  make([]ExperimentEvent, 0),
		metrics: make(map[string]*MetricAggregation),
	}
}

// NewTrafficAllocator creates a new traffic allocator
func NewTrafficAllocator() *TrafficAllocator {
	salt := make([]byte, 16)
	rand.Read(salt)
	
	return &TrafficAllocator{
		hashSalt: hex.EncodeToString(salt),
	}
}

// NewStatisticalAnalyzer creates a new statistical analyzer
func NewStatisticalAnalyzer() *StatisticalAnalyzer {
	analyzer := &StatisticalAnalyzer{
		tests: make(map[string]StatisticalTestFunc),
	}
	
	// Register default statistical tests
	analyzer.RegisterTest("t_test", analyzer.tTest)
	analyzer.RegisterTest("chi_square", analyzer.chiSquareTest)
	analyzer.RegisterTest("mann_whitney", analyzer.mannWhitneyTest)
	
	return analyzer
}

// CreateExperiment creates a new A/B test experiment
func (ab *ABTesting) CreateExperiment(exp *Experiment) error {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	// Validate experiment
	if err := ab.validateExperiment(exp); err != nil {
		return fmt.Errorf("experiment validation failed: %w", err)
	}

	// Set metadata
	exp.ID = ab.generateExperimentID()
	exp.CreatedAt = time.Now()
	exp.UpdatedAt = time.Now()
	exp.Status = StatusDraft

	// Store experiment
	ab.experiments[exp.ID] = exp

	return nil
}

// StartExperiment starts an experiment
func (ab *ABTesting) StartExperiment(experimentID string) error {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	exp, exists := ab.experiments[experimentID]
	if !exists {
		return fmt.Errorf("experiment %s not found", experimentID)
	}

	if exp.Status != StatusDraft {
		return fmt.Errorf("experiment must be in draft status to start")
	}

	// Final validation before starting
	if err := ab.validateExperimentForStart(exp); err != nil {
		return fmt.Errorf("experiment cannot be started: %w", err)
	}

	// Start the experiment
	exp.Status = StatusRunning
	exp.StartDate = time.Now()
	exp.UpdatedAt = time.Now()

	return nil
}

// StopExperiment stops an experiment
func (ab *ABTesting) StopExperiment(experimentID string) error {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	exp, exists := ab.experiments[experimentID]
	if !exists {
		return fmt.Errorf("experiment %s not found", experimentID)
	}

	if exp.Status != StatusRunning {
		return fmt.Errorf("experiment must be running to stop")
	}

	// Stop the experiment
	exp.Status = StatusCompleted
	exp.EndDate = time.Now()
	exp.UpdatedAt = time.Now()

	// Generate final results
	results, err := ab.generateResults(exp)
	if err != nil {
		return fmt.Errorf("failed to generate results: %w", err)
	}

	exp.Results = results

	return nil
}

// AssignUserToExperiment assigns a user to an experiment variant
func (ab *ABTesting) AssignUserToExperiment(userID, experimentID string, userProperties map[string]interface{}) (*UserExperiment, error) {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	exp, exists := ab.experiments[experimentID]
	if !exists {
		return nil, fmt.Errorf("experiment %s not found", experimentID)
	}

	if exp.Status != StatusRunning {
		return nil, fmt.Errorf("experiment is not running")
	}

	// Check if user is already assigned
	participation := ab.participations[userID]
	if participation != nil {
		if userExp, exists := participation.Experiments[experimentID]; exists {
			return userExp, nil
		}
	}

	// Check targeting criteria
	if !ab.meetsTargetCriteria(userProperties, exp.TargetCriteria) {
		return nil, fmt.Errorf("user doesn't meet target criteria")
	}

	// Check traffic allocation
	if !ab.allocator.shouldIncludeUser(userID, experimentID, exp.TrafficAllocation) {
		return nil, fmt.Errorf("user not selected for experiment traffic")
	}

	// Assign variant
	variant := ab.allocator.assignVariant(userID, experimentID, exp.Variants)

	// Create user experiment
	userExp := &UserExperiment{
		ExperimentID: experimentID,
		VariantID:    variant.ID,
		AssignedAt:   time.Now(),
		FirstSeen:    time.Now(),
		LastSeen:     time.Now(),
		EventCount:   0,
		Conversions:  0,
		Revenue:      0,
		Metadata:     make(map[string]interface{}),
	}

	// Update participation
	if participation == nil {
		participation = &Participation{
			UserID:       userID,
			Experiments:  make(map[string]*UserExperiment),
			JoinedAt:     time.Now(),
			LastActivity: time.Now(),
			Properties:   userProperties,
		}
		ab.participations[userID] = participation
	}

	participation.Experiments[experimentID] = userExp
	participation.LastActivity = time.Now()

	return userExp, nil
}

// TrackEvent tracks an event for an experiment
func (ab *ABTesting) TrackEvent(userID, experimentID, eventType string, properties map[string]interface{}, value float64) error {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	// Check if user is in experiment
	participation := ab.participations[userID]
	if participation == nil {
		return fmt.Errorf("user %s not participating in any experiments", userID)
	}

	userExp, exists := participation.Experiments[experimentID]
	if !exists {
		return fmt.Errorf("user %s not in experiment %s", userID, experimentID)
	}

	// Create event
	event := ExperimentEvent{
		ID:           ab.generateEventID(),
		ExperimentID: experimentID,
		VariantID:    userExp.VariantID,
		UserID:       userID,
		EventType:    eventType,
		Timestamp:    time.Now(),
		Properties:   properties,
		Value:        value,
	}

	// Track event
	ab.metrics.TrackEvent(event)

	// Update user experiment stats
	userExp.LastSeen = time.Now()
	userExp.EventCount++

	if eventType == "conversion" {
		userExp.Conversions++
	}

	if value > 0 {
		userExp.Revenue += value
	}

	return nil
}

// GetExperimentResults returns the current results of an experiment
func (ab *ABTesting) GetExperimentResults(experimentID string) (*ExperimentResults, error) {
	ab.mu.RLock()
	defer ab.mu.RUnlock()

	exp, exists := ab.experiments[experimentID]
	if !exists {
		return nil, fmt.Errorf("experiment %s not found", experimentID)
	}

	// Generate current results
	return ab.generateResults(exp)
}

// GetUserVariant returns the variant assigned to a user for an experiment
func (ab *ABTesting) GetUserVariant(userID, experimentID string) (*Variant, error) {
	ab.mu.RLock()
	defer ab.mu.RUnlock()

	participation := ab.participations[userID]
	if participation == nil {
		return nil, fmt.Errorf("user %s not participating in experiments", userID)
	}

	userExp, exists := participation.Experiments[experimentID]
	if !exists {
		return nil, fmt.Errorf("user %s not in experiment %s", userID, experimentID)
	}

	exp := ab.experiments[experimentID]
	for _, variant := range exp.Variants {
		if variant.ID == userExp.VariantID {
			return variant, nil
		}
	}

	return nil, fmt.Errorf("variant %s not found", userExp.VariantID)
}

// validateExperiment validates an experiment configuration
func (ab *ABTesting) validateExperiment(exp *Experiment) error {
	if exp.Name == "" {
		return fmt.Errorf("experiment name is required")
	}

	if len(exp.Variants) < 2 {
		return fmt.Errorf("experiment must have at least 2 variants")
	}

	// Validate variant weights sum to 1.0
	totalWeight := 0.0
	hasControl := false
	for _, variant := range exp.Variants {
		totalWeight += variant.Weight
		if variant.IsControl {
			if hasControl {
				return fmt.Errorf("experiment can only have one control variant")
			}
			hasControl = true
			exp.ControlVariant = variant.ID
		}
	}

	if math.Abs(totalWeight-1.0) > 0.001 {
		return fmt.Errorf("variant weights must sum to 1.0, got %f", totalWeight)
	}

	if !hasControl {
		return fmt.Errorf("experiment must have a control variant")
	}

	if exp.TrafficAllocation <= 0 || exp.TrafficAllocation > 1.0 {
		return fmt.Errorf("traffic allocation must be between 0 and 1")
	}

	return nil
}

// validateExperimentForStart validates an experiment before starting
func (ab *ABTesting) validateExperimentForStart(exp *Experiment) error {
	if exp.PrimaryMetric == "" {
		return fmt.Errorf("primary metric is required")
	}

	if exp.MinimumSampleSize <= 0 {
		return fmt.Errorf("minimum sample size must be positive")
	}

	return nil
}

// meetsTargetCriteria checks if user meets targeting criteria
func (ab *ABTesting) meetsTargetCriteria(userProperties map[string]interface{}, criteria *TargetCriteria) bool {
	if criteria == nil {
		return true
	}

	// Check user segments
	if len(criteria.UserSegments) > 0 {
		userSegments, ok := userProperties["segments"].([]string)
		if !ok {
			return false
		}
		
		hasMatchingSegment := false
		for _, segment := range criteria.UserSegments {
			for _, userSegment := range userSegments {
				if segment == userSegment {
					hasMatchingSegment = true
					break
				}
			}
			if hasMatchingSegment {
				break
			}
		}
		
		if !hasMatchingSegment {
			return false
		}
	}

	// Check country
	if len(criteria.Countries) > 0 {
		userCountry, ok := userProperties["country"].(string)
		if !ok {
			return false
		}
		
		hasMatchingCountry := false
		for _, country := range criteria.Countries {
			if country == userCountry {
				hasMatchingCountry = true
				break
			}
		}
		
		if !hasMatchingCountry {
			return false
		}
	}

	// Add more criteria checks as needed

	return true
}

// generateResults generates experiment results
func (ab *ABTesting) generateResults(exp *Experiment) (*ExperimentResults, error) {
	variantResults := make(map[string]*VariantResult)

	// Calculate results for each variant
	for _, variant := range exp.Variants {
		result := ab.calculateVariantResult(exp.ID, variant.ID)
		variantResults[variant.ID] = result
	}

	// Perform statistical analysis
	controlResult := variantResults[exp.ControlVariant]
	var statisticalTests []*StatisticalTest

	for variantID, variantResult := range variantResults {
		if variantID != exp.ControlVariant {
			test := ab.analyzer.performTTest(controlResult, variantResult)
			statisticalTests = append(statisticalTests, test)
		}
	}

	// Determine winner and confidence
	winner, confidence := ab.determineWinner(variantResults, statisticalTests)

	results := &ExperimentResults{
		ExperimentID:     exp.ID,
		Status:           ab.determineResultStatus(statisticalTests),
		Winner:           winner,
		Confidence:       confidence,
		VariantResults:   variantResults,
		StatisticalTests: statisticalTests,
		StartedAt:        exp.StartDate,
		Duration:         time.Since(exp.StartDate),
		GeneratedAt:      time.Now(),
	}

	if exp.Status == StatusCompleted {
		completedAt := exp.EndDate
		results.CompletedAt = &completedAt
	}

	return results, nil
}

// Helper methods for traffic allocation
func (ta *TrafficAllocator) shouldIncludeUser(userID, experimentID string, allocation float64) bool {
	hash := ta.hashUser(userID, experimentID)
	return hash < allocation
}

func (ta *TrafficAllocator) assignVariant(userID, experimentID string, variants []*Variant) *Variant {
	hash := ta.hashUser(userID, experimentID+"_variant")
	
	cumulative := 0.0
	for _, variant := range variants {
		cumulative += variant.Weight
		if hash <= cumulative {
			return variant
		}
	}
	
	// Fallback to first variant
	return variants[0]
}

func (ta *TrafficAllocator) hashUser(userID, experimentID string) float64 {
	// Simple hash function - in production would use proper hash
	combined := userID + experimentID + ta.hashSalt
	hash := 0
	for _, char := range combined {
		hash = hash*31 + int(char)
	}
	return float64(hash%1000000) / 1000000.0
}

// Helper methods for metrics collection
func (mc *MetricsCollector) TrackEvent(event ExperimentEvent) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.events = append(mc.events, event)
}

// Helper methods for statistical analysis
func (sa *StatisticalAnalyzer) RegisterTest(name string, testFunc StatisticalTestFunc) {
	sa.tests[name] = testFunc
}

func (sa *StatisticalAnalyzer) performTTest(control, variant *VariantResult) *StatisticalTest {
	// Simplified t-test implementation
	// In production, would use proper statistical library
	
	n1, n2 := float64(control.Participants), float64(variant.Participants)
	x1, x2 := control.ConversionRate, variant.ConversionRate
	
	// Calculate pooled standard error
	p := (float64(control.Conversions) + float64(variant.Conversions)) / (n1 + n2)
	se := math.Sqrt(p * (1 - p) * (1/n1 + 1/n2))
	
	// Calculate t-statistic
	t := (x2 - x1) / se
	
	// Simple p-value approximation
	pValue := 2 * (1 - math.Abs(t)/3) // Very simplified
	
	result := "insignificant"
	if pValue < 0.05 {
		result = "significant"
	}
	
	return &StatisticalTest{
		Name:      "t_test",
		Type:      "two_sample",
		PValue:    pValue,
		Statistic: t,
		Result:    result,
	}
}

func (sa *StatisticalAnalyzer) tTest(control, variant *VariantResult) *StatisticalTest {
	return sa.performTTest(control, variant)
}

func (sa *StatisticalAnalyzer) chiSquareTest(control, variant *VariantResult) *StatisticalTest {
	// Placeholder implementation
	return &StatisticalTest{
		Name:   "chi_square",
		Type:   "independence",
		Result: "not_implemented",
	}
}

func (sa *StatisticalAnalyzer) mannWhitneyTest(control, variant *VariantResult) *StatisticalTest {
	// Placeholder implementation
	return &StatisticalTest{
		Name:   "mann_whitney",
		Type:   "non_parametric",
		Result: "not_implemented",
	}
}

// calculateVariantResult calculates results for a specific variant
func (ab *ABTesting) calculateVariantResult(experimentID, variantID string) *VariantResult {
	participants := 0
	conversions := 0
	revenue := 0.0
	
	// Count participants and conversions
	for _, participation := range ab.participations {
		if userExp, exists := participation.Experiments[experimentID]; exists {
			if userExp.VariantID == variantID {
				participants++
				conversions += userExp.Conversions
				revenue += userExp.Revenue
			}
		}
	}
	
	conversionRate := 0.0
	revenuePerUser := 0.0
	
	if participants > 0 {
		conversionRate = float64(conversions) / float64(participants)
		revenuePerUser = revenue / float64(participants)
	}
	
	return &VariantResult{
		VariantID:      variantID,
		Participants:   participants,
		Conversions:    conversions,
		ConversionRate: conversionRate,
		Revenue:        revenue,
		RevenuePerUser: revenuePerUser,
		Metrics:        make(map[string]float64),
	}
}

// determineWinner determines the winning variant
func (ab *ABTesting) determineWinner(variantResults map[string]*VariantResult, tests []*StatisticalTest) (string, float64) {
	// Simple winner determination based on conversion rate and significance
	var bestVariant string
	bestRate := 0.0
	bestConfidence := 0.0
	
	for variantID, result := range variantResults {
		if result.ConversionRate > bestRate {
			bestRate = result.ConversionRate
			bestVariant = variantID
		}
	}
	
	// Check if the best variant is statistically significant
	for _, test := range tests {
		if test.Result == "significant" && test.PValue < 0.05 {
			bestConfidence = 1.0 - test.PValue
		}
	}
	
	return bestVariant, bestConfidence
}

// determineResultStatus determines the overall result status
func (ab *ABTesting) determineResultStatus(tests []*StatisticalTest) ResultStatus {
	hasSignificant := false
	for _, test := range tests {
		if test.Result == "significant" {
			hasSignificant = true
			break
		}
	}
	
	if hasSignificant {
		return ResultSignificant
	}
	
	return ResultInsignificant
}

// generateExperimentID generates a unique experiment ID
func (ab *ABTesting) generateExperimentID() string {
	return fmt.Sprintf("exp_%d", time.Now().UnixNano())
}

// generateEventID generates a unique event ID
func (ab *ABTesting) generateEventID() string {
	return fmt.Sprintf("evt_%d", time.Now().UnixNano())
}