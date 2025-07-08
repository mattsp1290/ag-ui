package sse

import "time"

// Monitoring constants for SSE transport
const (
	// Default intervals and timeouts
	DefaultMetricsInterval        = 30 * time.Second
	DefaultHealthCheckInterval    = 30 * time.Second
	DefaultHealthCheckTimeout     = 5 * time.Second
	DefaultResourceSampleInterval = 10 * time.Second

	// Alert thresholds
	DefaultErrorRateThreshold       = 5.0  // 5% error rate
	DefaultLatencyThreshold         = 1000 // 1000ms
	DefaultMemoryUsageThreshold     = 80.0 // 80% memory usage
	DefaultCPUUsageThreshold        = 80.0 // 80% CPU usage
	DefaultConnectionCountThreshold = 1000 // 1000 connections

	// History and buffer sizes
	DefaultMaxAlertHistory      = 1000
	DefaultMaxConnectionHistory = 10000
	DefaultMaxEventHistory      = 1000
	DefaultLatencySampleSize    = 1000
	DefaultResourceHistorySize  = 1000

	// Log sampling
	DefaultLogSamplingInitial    = 100
	DefaultLogSamplingThereafter = 100

	// Performance thresholds
	DefaultSlowOperationThreshold   = 100 * time.Millisecond
	DefaultConnectionErrorThreshold = 100

	// Alert suppression
	DefaultAlertSuppressionWindow = 5 * time.Minute
	DefaultDuplicateAlertWindow   = 5 * time.Minute

	// Trace sampling
	DefaultTraceSampleRate = 0.1

	// Buffer pool sizes
	DefaultEventBufferPoolSize = 100
	DefaultMetricsBufferSize   = 1000

	// Benchmark defaults
	DefaultBenchmarkDuration = 60 * time.Second
	DefaultBenchmarkWarmup   = 5 * time.Second
)
