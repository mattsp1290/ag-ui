// Package streaming provides advanced streaming capabilities for the AG-UI Go SDK.
// It includes stream management, chunked encoding, flow control, and metrics collection.
//
// The streaming package is designed to work with any encoding format (JSON, Protobuf, etc.)
// and provides memory-efficient processing of large event sequences.
//
// # Components
//
// StreamManager: Coordinates streaming operations with lifecycle management, backpressure
// handling, and buffer management. It provides a unified interface for both reading and
// writing streams with proper error handling and resource cleanup.
//
// ChunkedEncoder: Breaks large event sequences into manageable chunks with configurable
// sizes. It supports parallel processing, compression, and progress tracking. This is
// essential for handling streams with more than 10,000 events efficiently.
//
// FlowController: Implements flow control mechanisms including rate limiting, backpressure
// signaling, and buffer overflow prevention. It uses a token bucket algorithm for rate
// limiting and circular buffers for efficient memory usage.
//
// StreamMetrics: Collects comprehensive metrics including throughput, latency, memory
// usage, and progress monitoring. Metrics are collected in real-time with minimal overhead.
//
// UnifiedStreamCodec: Provides a format-agnostic streaming wrapper that integrates all
// components. It works with any base codec (JSON, Protobuf, etc.) and adds streaming
// enhancements transparently.
//
// # Usage Examples
//
// Basic streaming with automatic chunking:
//
//	config := streaming.DefaultUnifiedStreamConfig()
//	config.EnableChunking = true
//	config.ChunkConfig.MaxEventsPerChunk = 1000
//
//	baseCodec := json.NewJSONStreamCodec(nil, nil)
//	unifiedCodec := streaming.NewUnifiedStreamCodec(baseCodec, config)
//
//	// Stream encode
//	err := unifiedCodec.StreamEncode(ctx, eventChan, writer)
//
// Stream with flow control and metrics:
//
//	config := streaming.DefaultUnifiedStreamConfig()
//	config.EnableFlowControl = true
//	config.EnableMetrics = true
//
//	unifiedCodec := streaming.NewUnifiedStreamCodec(baseCodec, config)
//
//	// Register progress callback
//	unifiedCodec.RegisterProgressCallback(func(processed, total int64) {
//	    fmt.Printf("Progress: %d/%d\n", processed, total)
//	})
//
//	// Stream and get metrics
//	err := unifiedCodec.StreamEncode(ctx, eventChan, writer)
//	metrics := unifiedCodec.GetMetrics().GetSnapshot()
//	fmt.Printf("Processed %d events at %d events/sec\n",
//	    metrics.EventsProcessed, metrics.EventsPerSecond)
//
// # Memory Efficiency
//
// The streaming implementation maintains constant memory usage regardless of stream size
// through:
// - Chunked processing with configurable chunk sizes
// - Circular buffers for flow control
// - Object pooling for chunk reuse
// - Streaming I/O without full buffering
//
// # Error Handling
//
// All components provide comprehensive error handling with:
// - Context cancellation support
// - Graceful shutdown on errors
// - Error callbacks for custom handling
// - Automatic resource cleanup
//
// # Performance
//
// The implementation is optimized for high throughput:
// - Parallel chunk processing
// - Lock-free circular buffers
// - Atomic operations for metrics
// - Minimal allocations through pooling
package streaming