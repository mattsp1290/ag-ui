package transport

import (
	"time"
)

// TransportStats contains general transport statistics.
//
// Example usage:
//
//	// Monitoring transport health
//	func monitorTransportHealth(transport StatsProvider) {
//		stats := transport.Stats()
//		
//		// Check connection health
//		if stats.Uptime > 24*time.Hour && stats.ReconnectCount == 0 {
//			log.Println("Transport connection is stable")
//		}
//		
//		// Monitor error rate
//		if stats.ErrorCount > 0 {
//			errorRate := float64(stats.ErrorCount) / float64(stats.EventsSent+stats.EventsReceived)
//			if errorRate > 0.01 { // 1% error rate threshold
//				log.Printf("High error rate detected: %.2f%%", errorRate*100)
//			}
//		}
//		
//		// Check throughput
//		if !stats.LastEventSentAt.IsZero() {
//			timeSinceLastEvent := time.Since(stats.LastEventSentAt)
//			if timeSinceLastEvent > 5*time.Minute {
//				log.Println("Warning: No events sent in the last 5 minutes")
//			}
//		}
//		
//		// Log key metrics
//		log.Printf("Stats - Sent: %d, Received: %d, Avg Latency: %v, Uptime: %v",
//			stats.EventsSent, stats.EventsReceived, stats.AverageLatency, stats.Uptime)
//	}
//
//	// Creating a stats reporter
//	func reportStats(stats TransportStats) map[string]interface{} {
//		return map[string]interface{}{
//			"connection_uptime_hours": stats.Uptime.Hours(),
//			"total_events":           stats.EventsSent + stats.EventsReceived,
//			"throughput_mbps":        float64(stats.BytesSent+stats.BytesReceived) / stats.Uptime.Seconds() / 1024 / 1024,
//			"error_rate":             float64(stats.ErrorCount) / float64(stats.EventsSent+stats.EventsReceived),
//			"avg_latency_ms":         stats.AverageLatency.Milliseconds(),
//		}
//	}
type TransportStats struct {
	// Connection statistics
	ConnectedAt    time.Time     `json:"connected_at"`
	ReconnectCount int           `json:"reconnect_count"`
	LastError      error         `json:"last_error,omitempty"`
	Uptime         time.Duration `json:"uptime"`

	// Event statistics
	EventsSent       int64         `json:"events_sent"`
	EventsReceived   int64         `json:"events_received"`
	BytesSent        int64         `json:"bytes_sent"`
	BytesReceived    int64         `json:"bytes_received"`
	AverageLatency   time.Duration `json:"average_latency"`
	ErrorCount       int64         `json:"error_count"`
	LastEventSentAt  time.Time     `json:"last_event_sent_at"`
	LastEventRecvAt  time.Time     `json:"last_event_recv_at"`
}

// StreamingStats contains streaming-specific statistics.
//
// Example usage:
//
//	// Monitoring streaming performance
//	func monitorStreamingPerformance(transport StreamingStatsProvider) {
//		stats := transport.GetStreamingStats()
//		
//		// Check buffer utilization
//		if stats.BufferUtilization > 0.8 {
//			log.Printf("Warning: High buffer utilization: %.1f%%", stats.BufferUtilization*100)
//		}
//		
//		// Monitor backpressure
//		if stats.BackpressureEvents > 0 {
//			backpressureRate := float64(stats.BackpressureEvents) / float64(stats.EventsSent)
//			log.Printf("Backpressure rate: %.2f%%", backpressureRate*100)
//		}
//		
//		// Check for dropped events
//		if stats.DroppedEvents > 0 {
//			dropRate := float64(stats.DroppedEvents) / float64(stats.EventsSent)
//			if dropRate > 0.001 { // 0.1% drop rate threshold
//				log.Printf("High drop rate detected: %.3f%%", dropRate*100)
//			}
//		}
//		
//		// Log streaming metrics
//		log.Printf("Streaming - Active: %d/%d streams, Throughput: %.1f events/s, %.1f KB/s",
//			stats.StreamsActive, stats.StreamsTotal,
//			stats.ThroughputEventsPerSec, stats.ThroughputBytesPerSec/1024)
//	}
//
//	// Calculating streaming efficiency
//	func calculateStreamingEfficiency(stats StreamingStats) map[string]float64 {
//		efficiency := make(map[string]float64)
//		
//		if stats.EventsSent > 0 {
//			efficiency["delivery_rate"] = float64(stats.EventsSent-stats.DroppedEvents) / float64(stats.EventsSent)
//			efficiency["backpressure_rate"] = float64(stats.BackpressureEvents) / float64(stats.EventsSent)
//		}
//		
//		if stats.StreamsTotal > 0 {
//			efficiency["stream_utilization"] = float64(stats.StreamsActive) / float64(stats.StreamsTotal)
//		}
//		
//		efficiency["buffer_efficiency"] = 1.0 - stats.BufferUtilization
//		
//		return efficiency
//	}
type StreamingStats struct {
	TransportStats

	// Streaming-specific metrics
	StreamsActive        int           `json:"streams_active"`
	StreamsTotal         int           `json:"streams_total"`
	BufferUtilization    float64       `json:"buffer_utilization"`
	BackpressureEvents   int64         `json:"backpressure_events"`
	DroppedEvents        int64         `json:"dropped_events"`
	AverageEventSize     int64         `json:"average_event_size"`
	ThroughputEventsPerSec float64     `json:"throughput_events_per_sec"`
	ThroughputBytesPerSec  float64     `json:"throughput_bytes_per_sec"`
}

// ReliabilityStats contains reliability-specific statistics.
//
// Example usage:
//
//	// Monitoring reliability metrics
//	func monitorReliability(transport ReliabilityStatsProvider) {
//		stats := transport.GetReliabilityStats()
//		
//		// Calculate acknowledgment rate
//		totalEvents := stats.EventsAcknowledged + stats.EventsUnacknowledged
//		if totalEvents > 0 {
//			ackRate := float64(stats.EventsAcknowledged) / float64(totalEvents)
//			if ackRate < 0.95 { // 95% acknowledgment threshold
//				log.Printf("Low acknowledgment rate: %.2f%%", ackRate*100)
//			}
//		}
//		
//		// Monitor timeout rate
//		if stats.EventsTimedOut > 0 {
//			timeoutRate := float64(stats.EventsTimedOut) / float64(stats.EventsSent)
//			if timeoutRate > 0.01 { // 1% timeout threshold
//				log.Printf("High timeout rate: %.2f%%", timeoutRate*100)
//			}
//		}
//		
//		// Check retry efficiency
//		if stats.EventsRetried > 0 {
//			retrySuccessRate := float64(stats.EventsAcknowledged) / float64(stats.EventsRetried)
//			log.Printf("Retry success rate: %.2f%%", retrySuccessRate*100)
//		}
//		
//		// Monitor ordering issues
//		if stats.OutOfOrderEvents > 0 || stats.DuplicateEvents > 0 {
//			log.Printf("Ordering issues - Out of order: %d, Duplicates: %d",
//				stats.OutOfOrderEvents, stats.DuplicateEvents)
//		}
//		
//		log.Printf("Reliability - Ack: %d/%d (%.1f%%), Avg ack time: %v, Redelivery rate: %.2f%%",
//			stats.EventsAcknowledged, totalEvents, float64(stats.EventsAcknowledged)/float64(totalEvents)*100,
//			stats.AverageAckTime, stats.RedeliveryRate*100)
//	}
//
//	// Calculating reliability score
//	func calculateReliabilityScore(stats ReliabilityStats) float64 {
//		score := 1.0
//		
//		// Penalty for unacknowledged events
//		totalEvents := stats.EventsAcknowledged + stats.EventsUnacknowledged
//		if totalEvents > 0 {
//			ackRate := float64(stats.EventsAcknowledged) / float64(totalEvents)
//			score *= ackRate
//		}
//		
//		// Penalty for timeouts
//		if stats.EventsSent > 0 {
//			timeoutRate := float64(stats.EventsTimedOut) / float64(stats.EventsSent)
//			score *= (1.0 - timeoutRate)
//		}
//		
//		// Penalty for duplicates and ordering issues
//		if stats.EventsReceived > 0 {
//			orderingPenalty := float64(stats.OutOfOrderEvents+stats.DuplicateEvents) / float64(stats.EventsReceived)
//			score *= (1.0 - orderingPenalty)
//		}
//		
//		return score
//	}
type ReliabilityStats struct {
	TransportStats

	// Reliability-specific metrics
	EventsAcknowledged     int64         `json:"events_acknowledged"`
	EventsUnacknowledged   int64         `json:"events_unacknowledged"`
	EventsRetried          int64         `json:"events_retried"`
	EventsTimedOut         int64         `json:"events_timed_out"`
	AverageAckTime         time.Duration `json:"average_ack_time"`
	DuplicateEvents        int64         `json:"duplicate_events"`
	OutOfOrderEvents       int64         `json:"out_of_order_events"`
	RedeliveryRate         float64       `json:"redelivery_rate"`
}