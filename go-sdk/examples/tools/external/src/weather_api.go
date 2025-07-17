package external

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ag-ui/go-sdk/pkg/tools"
)

// WeatherAPIExecutor implements weather data retrieval from external APIs.
// This example demonstrates HTTP client integration, API key management,
// error handling, rate limiting, and response transformation.
type WeatherAPIExecutor struct {
	client      *http.Client
	apiKey      string
	baseURL     string
	rateLimiter *RateLimiter
	cache       *ResponseCache
}

// WeatherResponse represents the structured weather data
type WeatherResponse struct {
	Location     LocationInfo     `json:"location"`
	Current      CurrentWeather   `json:"current"`
	Forecast     []ForecastDay    `json:"forecast,omitempty"`
	Alerts       []WeatherAlert   `json:"alerts,omitempty"`
	DataSources  []string         `json:"data_sources"`
	LastUpdated  string           `json:"last_updated"`
	RequestInfo  WeatherRequestInfo      `json:"request_info"`
}

// LocationInfo contains location details
type LocationInfo struct {
	Name        string  `json:"name"`
	Region      string  `json:"region,omitempty"`
	Country     string  `json:"country"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Timezone    string  `json:"timezone,omitempty"`
	LocalTime   string  `json:"local_time,omitempty"`
}

// CurrentWeather contains current weather conditions
type CurrentWeather struct {
	Temperature     float64 `json:"temperature"`
	FeelsLike       float64 `json:"feels_like"`
	Humidity        int     `json:"humidity"`
	Pressure        float64 `json:"pressure"`
	Visibility      float64 `json:"visibility"`
	UVIndex         float64 `json:"uv_index"`
	WindSpeed       float64 `json:"wind_speed"`
	WindDirection   int     `json:"wind_direction"`
	WindGust        float64 `json:"wind_gust,omitempty"`
	CloudCover      int     `json:"cloud_cover"`
	Condition       string  `json:"condition"`
	ConditionCode   string  `json:"condition_code"`
	IsDay           bool    `json:"is_day"`
	ObservationTime string  `json:"observation_time"`
}

// ForecastDay contains forecast information for a single day
type ForecastDay struct {
	Date        string           `json:"date"`
	Sunrise     string           `json:"sunrise,omitempty"`
	Sunset      string           `json:"sunset,omitempty"`
	MaxTemp     float64          `json:"max_temp"`
	MinTemp     float64          `json:"min_temp"`
	AvgTemp     float64          `json:"avg_temp"`
	MaxWind     float64          `json:"max_wind"`
	TotalPrecip float64          `json:"total_precip"`
	AvgHumidity int              `json:"avg_humidity"`
	Condition   string           `json:"condition"`
	ChanceOfRain int             `json:"chance_of_rain"`
	ChanceOfSnow int             `json:"chance_of_snow"`
	Hours       []HourlyForecast `json:"hours,omitempty"`
}

// HourlyForecast contains hourly forecast data
type HourlyForecast struct {
	Time        string  `json:"time"`
	Temperature float64 `json:"temperature"`
	Condition   string  `json:"condition"`
	Humidity    int     `json:"humidity"`
	WindSpeed   float64 `json:"wind_speed"`
	Precipitation float64 `json:"precipitation"`
	ChanceOfRain  int     `json:"chance_of_rain"`
}

// WeatherAlert contains weather alert information
type WeatherAlert struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Urgency     string `json:"urgency"`
	Areas       string `json:"areas"`
	Category    string `json:"category"`
	Certainty   string `json:"certainty"`
	Event       string `json:"event"`
	Note        string `json:"note,omitempty"`
	Effective   string `json:"effective"`
	Expires     string `json:"expires"`
	MsgType     string `json:"msg_type"`
	Instruction string `json:"instruction,omitempty"`
}

// WeatherRequestInfo contains information about the API request
type WeatherRequestInfo struct {
	Provider      string        `json:"provider"`
	QueryType     string        `json:"query_type"`
	Units         string        `json:"units"`
	Language      string        `json:"language"`
	CacheHit      bool          `json:"cache_hit"`
	ResponseTime  time.Duration `json:"response_time"`
	RateLimitInfo RateLimitInfo `json:"rate_limit_info"`
}

// RateLimitInfo contains rate limiting information
type RateLimitInfo struct {
	RequestsRemaining int           `json:"requests_remaining"`
	ResetTime         time.Time     `json:"reset_time"`
	RequestsPerHour   int           `json:"requests_per_hour"`
	BackoffDelay      time.Duration `json:"backoff_delay,omitempty"`
}

// RateLimiter implements simple rate limiting
type RateLimiter struct {
	requests     map[string][]time.Time
	maxRequests  int
	timeWindow   time.Duration
	backoffDelay time.Duration
}

// ResponseCache implements simple response caching
type ResponseCache struct {
	cache   map[string]*CacheEntry
	maxSize int
	ttl     time.Duration
}

// CacheEntry represents a cached response
type CacheEntry struct {
	Data      interface{}
	ExpiresAt time.Time
	HitCount  int
}

// NewWeatherAPIExecutor creates a new weather API executor
func NewWeatherAPIExecutor(apiKey string) *WeatherAPIExecutor {
	return &WeatherAPIExecutor{
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  false,
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
					CipherSuites: []uint16{
						tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
						tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
						tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
						tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
						tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					},
				},
			},
		},
		apiKey:  apiKey,
		baseURL: "https://api.weatherapi.com/v1", // Mock weather API
		rateLimiter: &RateLimiter{
			requests:     make(map[string][]time.Time),
			maxRequests:  100,
			timeWindow:   time.Hour,
			backoffDelay: time.Second * 5,
		},
		cache: &ResponseCache{
			cache:   make(map[string]*CacheEntry),
			maxSize: 1000,
			ttl:     15 * time.Minute,
		},
	}
}

// Execute performs weather API operations based on the provided parameters
func (w *WeatherAPIExecutor) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	startTime := time.Now()

	// Extract parameters
	operation, ok := params["operation"].(string)
	if !ok {
		return nil, fmt.Errorf("operation parameter must be a string")
	}

	location, ok := params["location"].(string)
	if !ok {
		return nil, fmt.Errorf("location parameter must be a string")
	}

	// Extract optional parameters
	options := w.extractOptions(params)

	// Check rate limiting
	if err := w.rateLimiter.checkRateLimit(ctx, "weather_api"); err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("rate limit exceeded: %v", err),
			Metadata: map[string]interface{}{
				"rate_limited": true,
				"retry_after":  w.rateLimiter.backoffDelay.Seconds(),
			},
		}, nil
	}

	// Check cache first
	cacheKey := w.generateCacheKey(operation, location, options)
	if cached := w.cache.get(cacheKey); cached != nil {
		return &tools.ToolExecutionResult{
			Success: true,
			Data: map[string]interface{}{
				"weather_data": cached,
				"cache_hit":    true,
			},
			Metadata: map[string]interface{}{
				"cached":        true,
				"response_time": time.Since(startTime),
			},
		}, nil
	}

	// Perform weather API operation
	var result *WeatherResponse
	var err error

	switch operation {
	case "current":
		result, err = w.getCurrentWeather(ctx, location, options)
	case "forecast":
		result, err = w.getForecast(ctx, location, options)
	case "history":
		result, err = w.getWeatherHistory(ctx, location, options)
	case "alerts":
		result, err = w.getWeatherAlerts(ctx, location, options)
	case "search":
		return w.searchLocations(ctx, location, options)
	default:
		return nil, fmt.Errorf("unsupported operation: %s", operation)
	}

	responseTime := time.Since(startTime)

	if err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   err.Error(),
			Metadata: map[string]interface{}{
				"operation":     operation,
				"location":      location,
				"response_time": responseTime,
				"provider":      "weather_api",
			},
		}, nil
	}

	// Update rate limiter
	w.rateLimiter.recordRequest("weather_api")

	// Cache the result
	w.cache.set(cacheKey, result)

	// Update request info
	if result != nil {
		result.RequestInfo = WeatherRequestInfo{
			Provider:     "WeatherAPI",
			QueryType:    operation,
			Units:        options.Units,
			Language:     options.Language,
			CacheHit:     false,
			ResponseTime: responseTime,
			RateLimitInfo: RateLimitInfo{
				RequestsRemaining: w.rateLimiter.getRemainingRequests("weather_api"),
				ResetTime:         w.rateLimiter.getResetTime("weather_api"),
				RequestsPerHour:   w.rateLimiter.maxRequests,
			},
		}
	}

	// Prepare response
	responseData := map[string]interface{}{
		"weather_data": result,
		"cache_hit":    false,
		"summary": map[string]interface{}{
			"operation":      operation,
			"location":       result.Location.Name,
			"country":        result.Location.Country,
			"temperature":    result.Current.Temperature,
			"condition":      result.Current.Condition,
			"last_updated":   result.LastUpdated,
		},
	}

	if len(result.Forecast) > 0 {
		responseData["forecast_days"] = len(result.Forecast)
	}

	if len(result.Alerts) > 0 {
		responseData["active_alerts"] = len(result.Alerts)
		responseData["alert_summary"] = w.summarizeAlerts(result.Alerts)
	}

	return &tools.ToolExecutionResult{
		Success: true,
		Data:    responseData,
		Metadata: map[string]interface{}{
			"operation":          operation,
			"provider":           "WeatherAPI",
			"response_time_ms":   responseTime.Milliseconds(),
			"data_freshness":     "real-time",
			"rate_limit_remaining": result.RequestInfo.RateLimitInfo.RequestsRemaining,
			"coordinates": map[string]float64{
				"latitude":  result.Location.Latitude,
				"longitude": result.Location.Longitude,
			},
		},
	}, nil
}

// WeatherOptions contains optional parameters for weather requests
type WeatherOptions struct {
	Units      string `json:"units"`       // metric, imperial
	Language   string `json:"language"`    // en, es, fr, etc.
	Days       int    `json:"days"`        // forecast days (1-10)
	Hours      bool   `json:"hours"`       // include hourly forecast
	Alerts     bool   `json:"alerts"`      // include weather alerts
	AQI        bool   `json:"aqi"`         // include air quality index
	Date       string `json:"date"`        // for historical data (YYYY-MM-DD)
}

// extractOptions extracts optional parameters from the request
func (w *WeatherAPIExecutor) extractOptions(params map[string]interface{}) WeatherOptions {
	options := WeatherOptions{
		Units:    "metric",
		Language: "en",
		Days:     3,
		Hours:    false,
		Alerts:   true,
		AQI:      false,
	}

	if opts, exists := params["options"]; exists {
		if optsMap, ok := opts.(map[string]interface{}); ok {
			if units, exists := optsMap["units"]; exists {
				if unitsStr, ok := units.(string); ok {
					options.Units = unitsStr
				}
			}
			if lang, exists := optsMap["language"]; exists {
				if langStr, ok := lang.(string); ok {
					options.Language = langStr
				}
			}
			if days, exists := optsMap["days"]; exists {
				if daysFloat, ok := days.(float64); ok {
					options.Days = int(daysFloat)
				}
			}
			if hours, exists := optsMap["hours"]; exists {
				if hoursBool, ok := hours.(bool); ok {
					options.Hours = hoursBool
				}
			}
			if alerts, exists := optsMap["alerts"]; exists {
				if alertsBool, ok := alerts.(bool); ok {
					options.Alerts = alertsBool
				}
			}
			if aqi, exists := optsMap["aqi"]; exists {
				if aqiBool, ok := aqi.(bool); ok {
					options.AQI = aqiBool
				}
			}
			if date, exists := optsMap["date"]; exists {
				if dateStr, ok := date.(string); ok {
					options.Date = dateStr
				}
			}
		}
	}

	return options
}

// getCurrentWeather retrieves current weather conditions
func (w *WeatherAPIExecutor) getCurrentWeather(ctx context.Context, location string, options WeatherOptions) (*WeatherResponse, error) {
	// In a real implementation, this would make an HTTP request to the weather API
	// For this example, we'll return mock data
	
	// Simulate API call delay
	time.Sleep(200 * time.Millisecond)

	// Mock response based on location
	temp := 20.0
	condition := "Partly Cloudy"
	if strings.Contains(strings.ToLower(location), "london") {
		temp = 15.0
		condition = "Overcast"
	} else if strings.Contains(strings.ToLower(location), "miami") {
		temp = 28.0
		condition = "Sunny"
	} else if strings.Contains(strings.ToLower(location), "moscow") {
		temp = -5.0
		condition = "Snow"
	}

	response := &WeatherResponse{
		Location: LocationInfo{
			Name:      w.parseLocationName(location),
			Country:   w.parseLocationCountry(location),
			Latitude:  40.7128,
			Longitude: -74.0060,
			Timezone:  "America/New_York",
			LocalTime: time.Now().Format("2006-01-02 15:04"),
		},
		Current: CurrentWeather{
			Temperature:     temp,
			FeelsLike:       temp - 2,
			Humidity:        65,
			Pressure:        1013.2,
			Visibility:      10.0,
			UVIndex:         5.0,
			WindSpeed:       15.0,
			WindDirection:   230,
			CloudCover:      40,
			Condition:       condition,
			ConditionCode:   "1003",
			IsDay:           time.Now().Hour() >= 6 && time.Now().Hour() <= 18,
			ObservationTime: time.Now().Format(time.RFC3339),
		},
		DataSources: []string{"WeatherAPI", "NOAA", "MetOffice"},
		LastUpdated: time.Now().Format(time.RFC3339),
	}

	// Add alerts if requested and conditions warrant them
	if options.Alerts && temp < 0 {
		response.Alerts = []WeatherAlert{
			{
				Title:       "Winter Weather Advisory",
				Description: "Snow and ice expected. Travel may be hazardous.",
				Severity:    "Minor",
				Urgency:     "Expected",
				Category:    "Met",
				Event:       "Winter Weather Advisory",
				Effective:   time.Now().Format(time.RFC3339),
				Expires:     time.Now().Add(24 * time.Hour).Format(time.RFC3339),
			},
		}
	}

	return response, nil
}

// getForecast retrieves weather forecast
func (w *WeatherAPIExecutor) getForecast(ctx context.Context, location string, options WeatherOptions) (*WeatherResponse, error) {
	// Get current weather first
	response, err := w.getCurrentWeather(ctx, location, options)
	if err != nil {
		return nil, err
	}

	// Add forecast data
	forecast := make([]ForecastDay, options.Days)
	baseTemp := response.Current.Temperature

	for i := 0; i < options.Days; i++ {
		date := time.Now().AddDate(0, 0, i+1)
		
		// Simulate temperature variation
		tempVariation := float64(i) * 2.0
		maxTemp := baseTemp + tempVariation + 5
		minTemp := baseTemp + tempVariation - 5

		forecastDay := ForecastDay{
			Date:         date.Format("2006-01-02"),
			Sunrise:      "06:30",
			Sunset:       "19:45",
			MaxTemp:      maxTemp,
			MinTemp:      minTemp,
			AvgTemp:      (maxTemp + minTemp) / 2,
			MaxWind:      20.0,
			TotalPrecip:  2.5,
			AvgHumidity:  70,
			Condition:    "Partly Cloudy",
			ChanceOfRain: 30,
			ChanceOfSnow: 0,
		}

		// Add hourly forecast if requested
		if options.Hours {
			hours := make([]HourlyForecast, 24)
			for h := 0; h < 24; h++ {
				hourTime := date.Add(time.Duration(h) * time.Hour)
				hours[h] = HourlyForecast{
					Time:          hourTime.Format("2006-01-02 15:04"),
					Temperature:   minTemp + (maxTemp-minTemp)*float64(h)/24.0,
					Condition:     "Partly Cloudy",
					Humidity:      70 - h*2,
					WindSpeed:     15.0,
					Precipitation: 0.1,
					ChanceOfRain:  20,
				}
			}
			forecastDay.Hours = hours
		}

		forecast[i] = forecastDay
	}

	response.Forecast = forecast
	return response, nil
}

// getWeatherHistory retrieves historical weather data
func (w *WeatherAPIExecutor) getWeatherHistory(ctx context.Context, location string, options WeatherOptions) (*WeatherResponse, error) {
	if options.Date == "" {
		return nil, fmt.Errorf("date parameter is required for historical weather data")
	}

	// Parse the date
	date, err := time.Parse("2006-01-02", options.Date)
	if err != nil {
		return nil, fmt.Errorf("invalid date format, expected YYYY-MM-DD: %w", err)
	}

	// Check if date is not too far in the past (mock limitation)
	if time.Since(date) > 365*24*time.Hour {
		return nil, fmt.Errorf("historical data only available for the past year")
	}

	// Get base response
	response, err := w.getCurrentWeather(ctx, location, options)
	if err != nil {
		return nil, err
	}

	// Modify for historical data
	response.Current.Temperature -= 5 // Historical data is typically different
	response.Current.ObservationTime = date.Format(time.RFC3339)
	response.LastUpdated = date.Format(time.RFC3339)

	return response, nil
}

// getWeatherAlerts retrieves weather alerts
func (w *WeatherAPIExecutor) getWeatherAlerts(ctx context.Context, location string, options WeatherOptions) (*WeatherResponse, error) {
	response, err := w.getCurrentWeather(ctx, location, options)
	if err != nil {
		return nil, err
	}

	// Mock some weather alerts based on conditions
	alerts := []WeatherAlert{}

	if response.Current.Temperature > 35 {
		alerts = append(alerts, WeatherAlert{
			Title:       "Heat Warning",
			Description: "Dangerously high temperatures expected. Stay hydrated and avoid prolonged outdoor exposure.",
			Severity:    "Moderate",
			Urgency:     "Immediate",
			Category:    "Met",
			Event:       "Heat Warning",
			Effective:   time.Now().Format(time.RFC3339),
			Expires:     time.Now().Add(48 * time.Hour).Format(time.RFC3339),
		})
	}

	if response.Current.WindSpeed > 50 {
		alerts = append(alerts, WeatherAlert{
			Title:       "Wind Advisory",
			Description: "Strong winds may cause damage to trees and power lines.",
			Severity:    "Minor",
			Urgency:     "Expected",
			Category:    "Met",
			Event:       "Wind Advisory",
			Effective:   time.Now().Format(time.RFC3339),
			Expires:     time.Now().Add(12 * time.Hour).Format(time.RFC3339),
		})
	}

	response.Alerts = alerts
	return response, nil
}

// searchLocations searches for locations matching the query
func (w *WeatherAPIExecutor) searchLocations(ctx context.Context, query string, options WeatherOptions) (*tools.ToolExecutionResult, error) {
	// Mock location search results
	locations := []LocationInfo{}

	query = strings.ToLower(query)
	
	// Mock some location matches
	if strings.Contains(query, "london") {
		locations = append(locations, LocationInfo{
			Name:      "London",
			Region:    "England",
			Country:   "United Kingdom",
			Latitude:  51.5074,
			Longitude: -0.1278,
		})
	}
	
	if strings.Contains(query, "new york") || strings.Contains(query, "nyc") {
		locations = append(locations, LocationInfo{
			Name:      "New York",
			Region:    "New York",
			Country:   "United States",
			Latitude:  40.7128,
			Longitude: -74.0060,
		})
	}
	
	if strings.Contains(query, "tokyo") {
		locations = append(locations, LocationInfo{
			Name:      "Tokyo",
			Region:    "Tokyo",
			Country:   "Japan",
			Latitude:  35.6762,
			Longitude: 139.6503,
		})
	}

	// Add some generic results if nothing specific matched
	if len(locations) == 0 {
		locations = append(locations, LocationInfo{
			Name:      query,
			Country:   "Unknown",
			Latitude:  0.0,
			Longitude: 0.0,
		})
	}

	return &tools.ToolExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"locations": locations,
			"query":     query,
			"count":     len(locations),
		},
		Metadata: map[string]interface{}{
			"operation": "search",
			"provider":  "WeatherAPI",
		},
	}, nil
}

// Helper methods

func (w *WeatherAPIExecutor) parseLocationName(location string) string {
	parts := strings.Split(location, ",")
	return strings.TrimSpace(parts[0])
}

func (w *WeatherAPIExecutor) parseLocationCountry(location string) string {
	parts := strings.Split(location, ",")
	if len(parts) > 1 {
		return strings.TrimSpace(parts[len(parts)-1])
	}
	return "Unknown"
}

func (w *WeatherAPIExecutor) generateCacheKey(operation, location string, options WeatherOptions) string {
	return fmt.Sprintf("%s:%s:%s:%d:%s", operation, location, options.Units, options.Days, options.Date)
}

func (w *WeatherAPIExecutor) summarizeAlerts(alerts []WeatherAlert) map[string]interface{} {
	severityCounts := make(map[string]int)
	categories := make(map[string]int)
	
	for _, alert := range alerts {
		severityCounts[alert.Severity]++
		categories[alert.Category]++
	}
	
	return map[string]interface{}{
		"total":              len(alerts),
		"by_severity":        severityCounts,
		"by_category":        categories,
		"most_recent_title":  alerts[0].Title,
	}
}

// Rate limiting methods

func (r *RateLimiter) checkRateLimit(ctx context.Context, key string) error {
	now := time.Now()
	
	// Clean old requests
	if requests, exists := r.requests[key]; exists {
		var validRequests []time.Time
		for _, reqTime := range requests {
			if now.Sub(reqTime) < r.timeWindow {
				validRequests = append(validRequests, reqTime)
			}
		}
		r.requests[key] = validRequests
	}
	
	// Check if limit is exceeded
	if len(r.requests[key]) >= r.maxRequests {
		return fmt.Errorf("rate limit exceeded, try again in %v", r.backoffDelay)
	}
	
	return nil
}

func (r *RateLimiter) recordRequest(key string) {
	now := time.Now()
	if r.requests[key] == nil {
		r.requests[key] = []time.Time{}
	}
	r.requests[key] = append(r.requests[key], now)
}

func (r *RateLimiter) getRemainingRequests(key string) int {
	if requests, exists := r.requests[key]; exists {
		return r.maxRequests - len(requests)
	}
	return r.maxRequests
}

func (r *RateLimiter) getResetTime(key string) time.Time {
	if requests, exists := r.requests[key]; exists && len(requests) > 0 {
		oldestRequest := requests[0]
		return oldestRequest.Add(r.timeWindow)
	}
	return time.Now().Add(r.timeWindow)
}

// Caching methods

func (c *ResponseCache) get(key string) interface{} {
	if entry, exists := c.cache[key]; exists {
		if time.Now().Before(entry.ExpiresAt) {
			entry.HitCount++
			return entry.Data
		}
		delete(c.cache, key)
	}
	return nil
}

func (c *ResponseCache) set(key string, data interface{}) {
	// Simple eviction if cache is full
	if len(c.cache) >= c.maxSize {
		// Remove oldest entry (simple strategy)
		for k := range c.cache {
			delete(c.cache, k)
			break
		}
	}
	
	c.cache[key] = &CacheEntry{
		Data:      data,
		ExpiresAt: time.Now().Add(c.ttl),
		HitCount:  0,
	}
}

// CreateWeatherAPITool creates and configures the weather API tool
func CreateWeatherAPITool() *tools.Tool {
	// In a real application, the API key would come from environment variables or configuration
	apiKey := "demo_weather_api_key"
	
	return &tools.Tool{
		ID:          "weather_api",
		Name:        "Weather API Integration",
		Description: "Comprehensive weather data retrieval with current conditions, forecasts, alerts, and location search",
		Version:     "1.0.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"operation": {
					Type:        "string",
					Description: "Weather operation to perform",
					Enum: []interface{}{
						"current", "forecast", "history", "alerts", "search",
					},
				},
				"location": {
					Type:        "string",
					Description: "Location query (city name, coordinates, etc.)",
					MinLength:   &[]int{1}[0],
					MaxLength:   &[]int{200}[0],
				},
				"options": {
					Type:        "object",
					Description: "Additional options for the weather request",
					Properties: map[string]*tools.Property{
						"units": {
							Type:        "string",
							Description: "Temperature units",
							Enum: []interface{}{
								"metric", "imperial", "kelvin",
							},
							Default: "metric",
						},
						"language": {
							Type:        "string",
							Description: "Response language",
							Enum: []interface{}{
								"en", "es", "fr", "de", "it", "pt", "ru", "ja", "ko", "zh",
							},
							Default: "en",
						},
						"days": {
							Type:        "number",
							Description: "Number of forecast days (1-10)",
							Minimum:     &[]float64{1}[0],
							Maximum:     &[]float64{10}[0],
							Default:     3,
						},
						"hours": {
							Type:        "boolean",
							Description: "Include hourly forecast data",
							Default:     false,
						},
						"alerts": {
							Type:        "boolean",
							Description: "Include weather alerts",
							Default:     true,
						},
						"aqi": {
							Type:        "boolean",
							Description: "Include air quality index",
							Default:     false,
						},
						"date": {
							Type:        "string",
							Description: "Date for historical data (YYYY-MM-DD)",
							Pattern:     `^\d{4}-\d{2}-\d{2}$`,
						},
					},
				},
			},
			Required: []string{"operation", "location"},
		},
		Metadata: &tools.ToolMetadata{
			Author:        "AG-UI SDK Examples",
			License:       "MIT",
			Documentation: "https://github.com/mattsp1290/ag-ui/blob/main/go-sdk/examples/tools/external/README.md",
			Tags:          []string{"weather", "api", "external", "http", "real-time"},
			Examples: []tools.ToolExample{
				{
					Name:        "Current Weather",
					Description: "Get current weather conditions for a city",
					Input: map[string]interface{}{
						"operation": "current",
						"location":  "London, UK",
						"options": map[string]interface{}{
							"units": "metric",
							"alerts": true,
						},
					},
				},
				{
					Name:        "Weather Forecast",
					Description: "Get 5-day weather forecast with hourly data",
					Input: map[string]interface{}{
						"operation": "forecast",
						"location":  "New York, NY",
						"options": map[string]interface{}{
							"days":  5,
							"hours": true,
							"units": "imperial",
						},
					},
				},
				{
					Name:        "Location Search",
					Description: "Search for locations matching a query",
					Input: map[string]interface{}{
						"operation": "search",
						"location":  "Paris",
					},
				},
			},
		},
		Capabilities: &tools.ToolCapabilities{
			Streaming:  false,
			Async:      true,  // Can be run asynchronously
			Cancelable: true,  // HTTP requests can be cancelled
			Retryable:  true,  // Failed requests can be retried
			Cacheable:  true,  // Results can be cached
			RateLimit:  100,   // 100 requests per hour
			Timeout:    30 * time.Second,
		},
		Executor: NewWeatherAPIExecutor(apiKey),
	}
}

// RunWeatherApiExample demonstrates the weather API tool functionality
func RunWeatherApiExample() {
	// Create registry and register the weather API tool
	registry := tools.NewRegistry()
	weatherTool := CreateWeatherAPITool()

	if err := registry.Register(weatherTool); err != nil {
		log.Fatalf("Failed to register weather API tool: %v", err)
	}

	// Create execution engine with caching and rate limiting
	engine := tools.NewExecutionEngine(registry,
		tools.WithCaching(1000, 15*time.Minute),
		tools.WithDefaultTimeout(30*time.Second),
	)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := engine.Shutdown(ctx); err != nil {
			log.Printf("Engine shutdown error: %v", err)
		}
	}()

	ctx := context.Background()

	fmt.Println("=== Weather API Integration Tool Example ===")
	fmt.Println("Demonstrates: External API integration, HTTP clients, rate limiting, and caching")
	fmt.Println()

	// Example 1: Current weather
	fmt.Println("1. Getting current weather for London...")
	result, err := engine.Execute(ctx, "weather_api", map[string]interface{}{
		"operation": "current",
		"location":  "London, UK",
		"options": map[string]interface{}{
			"units":  "metric",
			"alerts": true,
		},
	})
	printWeatherResult(result, err, "Current weather")

	// Example 2: Weather forecast
	fmt.Println("2. Getting weather forecast for New York...")
	result, err = engine.Execute(ctx, "weather_api", map[string]interface{}{
		"operation": "forecast",
		"location":  "New York, NY",
		"options": map[string]interface{}{
			"days":  5,
			"hours": false,
			"units": "imperial",
		},
	})
	printWeatherResult(result, err, "Weather forecast")

	// Example 3: Location search
	fmt.Println("3. Searching for locations...")
	result, err = engine.Execute(ctx, "weather_api", map[string]interface{}{
		"operation": "search",
		"location":  "Tokyo",
	})
	printWeatherResult(result, err, "Location search")

	// Example 4: Weather alerts
	fmt.Println("4. Getting weather alerts for Miami...")
	result, err = engine.Execute(ctx, "weather_api", map[string]interface{}{
		"operation": "alerts",
		"location":  "Miami, FL",
		"options": map[string]interface{}{
			"units": "imperial",
		},
	})
	printWeatherResult(result, err, "Weather alerts")

	// Example 5: Historical weather (with error handling)
	fmt.Println("5. Getting historical weather data...")
	result, err = engine.Execute(ctx, "weather_api", map[string]interface{}{
		"operation": "history",
		"location":  "Moscow, Russia",
		"options": map[string]interface{}{
			"date":  "2024-01-01",
			"units": "metric",
		},
	})
	printWeatherResult(result, err, "Historical weather")

	// Example 6: Test caching (same request as example 1)
	fmt.Println("6. Testing cache with same London request...")
	result, err = engine.Execute(ctx, "weather_api", map[string]interface{}{
		"operation": "current",
		"location":  "London, UK",
		"options": map[string]interface{}{
			"units":  "metric",
			"alerts": true,
		},
	})
	printWeatherResult(result, err, "Cached weather")
}

func printWeatherResult(result *tools.ToolExecutionResult, err error, title string) {
	fmt.Printf("=== %s ===\n", title)
	
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		fmt.Println()
		return
	}

	if !result.Success {
		fmt.Printf("  Failed: %s\n", result.Error)
		if metadata := result.Metadata; metadata != nil {
			if rateLimited, exists := metadata["rate_limited"]; exists && rateLimited.(bool) {
				fmt.Printf("  Rate limited - retry after: %v seconds\n", metadata["retry_after"])
			}
		}
		fmt.Println()
		return
	}

	data := result.Data.(map[string]interface{})
	
	// Handle different operation types
	if _, exists := data["weather_data"]; exists {
		// Weather data response
		fmt.Printf("  Success: Weather data retrieved\n")
		if summary, exists := data["summary"]; exists {
			summaryMap := summary.(map[string]interface{})
			fmt.Printf("  Location: %v, %v\n", summaryMap["location"], summaryMap["country"])
			fmt.Printf("  Temperature: %v°\n", summaryMap["temperature"])
			fmt.Printf("  Condition: %v\n", summaryMap["condition"])
			fmt.Printf("  Last updated: %v\n", summaryMap["last_updated"])
		}
		
		if forecastDays, exists := data["forecast_days"]; exists {
			fmt.Printf("  Forecast days: %v\n", forecastDays)
		}
		
		if alertCount, exists := data["active_alerts"]; exists {
			fmt.Printf("  Active alerts: %v\n", alertCount)
			if alertSummary, exists := data["alert_summary"]; exists {
				summary := alertSummary.(map[string]interface{})
				fmt.Printf("  Most recent alert: %v\n", summary["most_recent_title"])
			}
		}
		
		if cacheHit, exists := data["cache_hit"]; exists && cacheHit.(bool) {
			fmt.Printf("  Data source: Cache (hit)\n")
		} else {
			fmt.Printf("  Data source: API (fresh)\n")
		}
	} else if locations, exists := data["locations"]; exists {
		// Location search response
		fmt.Printf("  Success: Found %v location(s)\n", data["count"])
		locationList := locations.([]interface{})
		for i, loc := range locationList {
			if i < 3 { // Show first 3 results
				locMap := loc.(map[string]interface{})
				fmt.Printf("    %v: %v, %v (%.4f, %.4f)\n", 
					i+1, locMap["name"], locMap["country"], 
					locMap["latitude"], locMap["longitude"])
			}
		}
		if len(locationList) > 3 {
			fmt.Printf("    ... and %d more results\n", len(locationList)-3)
		}
	}

	// Show metadata
	if metadata := result.Metadata; metadata != nil {
		fmt.Printf("  Response time: %vms\n", metadata["response_time_ms"])
		if rateLimitRemaining, exists := metadata["rate_limit_remaining"]; exists {
			fmt.Printf("  Rate limit remaining: %v requests\n", rateLimitRemaining)
		}
		if cached, exists := metadata["cached"]; exists && cached.(bool) {
			fmt.Printf("  Served from cache\n")
		}
	}

	fmt.Printf("  Duration: %v\n", result.Duration)
	fmt.Println()
}