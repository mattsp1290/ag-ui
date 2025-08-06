package websocket

import (
	"compress/flate"
	"fmt"
	"io"
	"math"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	pkgerrors "github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
)

// CompressionConfig defines compression settings
type CompressionConfig struct {
	// Enable compression
	Enabled bool `json:"enabled"`

	// Compression levels (0-9, where 0 is no compression, 9 is best compression)
	CompressionLevel int `json:"compression_level"`

	// Compression threshold - messages smaller than this won't be compressed
	CompressionThreshold int64 `json:"compression_threshold"`

	// Maximum compression ratio - reject messages that compress poorly
	MaxCompressionRatio float64 `json:"max_compression_ratio"`

	// Memory limits
	MaxMemoryUsage int64 `json:"max_memory_usage"`
	WindowBits     int   `json:"window_bits"` // LZ77 sliding window size
	MemLevel       int   `json:"mem_level"`   // Memory level for compression

	// Performance settings
	UsePooledCompressors bool `json:"use_pooled_compressors"`
	CompressorPoolSize   int  `json:"compressor_pool_size"`

	// Fallback settings
	FallbackEnabled   bool `json:"fallback_enabled"`    // Allow fallback to uncompressed
	AutoDetectSupport bool `json:"auto_detect_support"` // Auto-detect client support

	// Monitoring
	CollectStatistics  bool          `json:"collect_statistics"`
	StatisticsInterval time.Duration `json:"statistics_interval"`
}

// DefaultCompressionConfig returns default compression settings
func DefaultCompressionConfig() *CompressionConfig {
	return &CompressionConfig{
		Enabled:              true,
		CompressionLevel:     6,                // Balanced compression
		CompressionThreshold: 1024,             // 1KB
		MaxCompressionRatio:  0.1,              // Reject if compressed size > 10% of original
		MaxMemoryUsage:       16 * 1024 * 1024, // 16MB
		WindowBits:           15,
		MemLevel:             8,
		UsePooledCompressors: true,
		CompressorPoolSize:   10,
		FallbackEnabled:      true,
		AutoDetectSupport:    true,
		CollectStatistics:    true,
		StatisticsInterval:   30 * time.Second,
	}
}

// CompressionStats holds compression statistics
type CompressionStats struct {
	mu sync.RWMutex

	// Message counts
	TotalMessages        int64 `json:"total_messages"`
	CompressedMessages   int64 `json:"compressed_messages"`
	UncompressedMessages int64 `json:"uncompressed_messages"`
	FailedCompressions   int64 `json:"failed_compressions"`

	// Byte counts
	TotalBytesIn  int64 `json:"total_bytes_in"`
	TotalBytesOut int64 `json:"total_bytes_out"`
	BytesSaved    int64 `json:"bytes_saved"`

	// Performance metrics
	CompressionTime         time.Duration `json:"compression_time"`
	DecompressionTime       time.Duration `json:"decompression_time"`
	AverageCompressionRatio float64       `json:"average_compression_ratio"`

	// Error tracking
	CompressionErrors   int64 `json:"compression_errors"`
	DecompressionErrors int64 `json:"decompression_errors"`

	// Memory usage
	CurrentMemoryUsage int64 `json:"current_memory_usage"`
	PeakMemoryUsage    int64 `json:"peak_memory_usage"`
}

// CompressionManager handles WebSocket message compression
type CompressionManager struct {
	config           *CompressionConfig
	stats            *CompressionStats
	compressorPool   *sync.Pool
	decompressorPool *sync.Pool
	extensions       map[string]bool // Supported extensions
	mu               sync.RWMutex

	// Monitoring
	statsTimer *time.Timer
	shutdownCh chan struct{}
	wg         sync.WaitGroup // Track goroutines
}

// CompressedMessage represents a compressed WebSocket message
type CompressedMessage struct {
	Data             []byte
	OriginalSize     int64
	CompressedSize   int64
	CompressionRatio float64
	IsCompressed     bool
	Extension        string
}

// NewCompressionManager creates a new compression manager
func NewCompressionManager(config *CompressionConfig) *CompressionManager {
	if config == nil {
		config = DefaultCompressionConfig()
	}

	cm := &CompressionManager{
		config:     config,
		stats:      &CompressionStats{},
		extensions: make(map[string]bool),
		shutdownCh: make(chan struct{}),
	}

	// Initialize compressor pool
	if config.UsePooledCompressors {
		cm.compressorPool = &sync.Pool{
			New: func() interface{} {
				w, err := flate.NewWriter(nil, config.CompressionLevel)
				if err != nil {
					return nil
				}
				return w
			},
		}

		cm.decompressorPool = &sync.Pool{
			New: func() interface{} {
				return flate.NewReader(nil)
			},
		}
	}

	// Support standard WebSocket compression extensions
	cm.extensions["permessage-deflate"] = true
	cm.extensions["x-webkit-deflate-frame"] = true

	// Start statistics collection
	if config.CollectStatistics {
		cm.startStatsCollection()
	}

	return cm
}

// CreateUpgrader creates an upgrader with compression support
func (cm *CompressionManager) CreateUpgrader(baseUpgrader *websocket.Upgrader) *websocket.Upgrader {
	if !cm.config.Enabled {
		return baseUpgrader
	}

	// Configure compression extensions
	if baseUpgrader.EnableCompression {
		// Already enabled, just configure parameters
		return baseUpgrader
	}

	// Create new upgrader with compression
	upgrader := *baseUpgrader
	upgrader.EnableCompression = true

	return &upgrader
}

// CompressMessage compresses a message if appropriate
func (cm *CompressionManager) CompressMessage(data []byte, messageType int) (*CompressedMessage, error) {
	start := time.Now()
	defer func() {
		cm.stats.mu.Lock()
		cm.stats.CompressionTime += time.Since(start)
		cm.stats.mu.Unlock()
	}()

	cm.stats.mu.Lock()
	cm.stats.TotalMessages++
	cm.stats.TotalBytesIn += int64(len(data))
	cm.stats.mu.Unlock()

	// Check if compression is enabled
	if !cm.config.Enabled {
		return &CompressedMessage{
			Data:             data,
			OriginalSize:     int64(len(data)),
			CompressedSize:   int64(len(data)),
			CompressionRatio: 1.0,
			IsCompressed:     false,
		}, nil
	}

	// Check if message is large enough to compress
	if int64(len(data)) < cm.config.CompressionThreshold {
		cm.stats.mu.Lock()
		cm.stats.UncompressedMessages++
		cm.stats.mu.Unlock()

		return &CompressedMessage{
			Data:             data,
			OriginalSize:     int64(len(data)),
			CompressedSize:   int64(len(data)),
			CompressionRatio: 1.0,
			IsCompressed:     false,
		}, nil
	}

	// Only compress text and binary messages
	if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
		cm.stats.mu.Lock()
		cm.stats.UncompressedMessages++
		cm.stats.mu.Unlock()

		return &CompressedMessage{
			Data:             data,
			OriginalSize:     int64(len(data)),
			CompressedSize:   int64(len(data)),
			CompressionRatio: 1.0,
			IsCompressed:     false,
		}, nil
	}

	// Compress the message
	compressed, err := cm.compressData(data)
	if err != nil {
		cm.stats.mu.Lock()
		cm.stats.CompressionErrors++
		cm.stats.FailedCompressions++
		cm.stats.mu.Unlock()

		if cm.config.FallbackEnabled {
			return &CompressedMessage{
				Data:             data,
				OriginalSize:     int64(len(data)),
				CompressedSize:   int64(len(data)),
				CompressionRatio: 1.0,
				IsCompressed:     false,
			}, nil
		}

		return nil, pkgerrors.WithOperation("compress", "message_data", err)
	}

	originalSize := int64(len(data))
	compressedSize := int64(len(compressed))
	ratio := float64(compressedSize) / float64(originalSize)

	// Check if compression ratio is acceptable
	if ratio > cm.config.MaxCompressionRatio {
		cm.stats.mu.Lock()
		cm.stats.UncompressedMessages++
		cm.stats.mu.Unlock()

		return &CompressedMessage{
			Data:             data,
			OriginalSize:     originalSize,
			CompressedSize:   originalSize,
			CompressionRatio: 1.0,
			IsCompressed:     false,
		}, nil
	}

	// Update statistics
	cm.stats.mu.Lock()
	cm.stats.CompressedMessages++
	cm.stats.TotalBytesOut += compressedSize
	cm.stats.BytesSaved += originalSize - compressedSize

	// Update average compression ratio
	totalCompressed := cm.stats.CompressedMessages
	if totalCompressed > 0 {
		cm.stats.AverageCompressionRatio = ((cm.stats.AverageCompressionRatio * float64(totalCompressed-1)) + ratio) / float64(totalCompressed)
	}
	cm.stats.mu.Unlock()

	return &CompressedMessage{
		Data:             compressed,
		OriginalSize:     originalSize,
		CompressedSize:   compressedSize,
		CompressionRatio: ratio,
		IsCompressed:     true,
		Extension:        "permessage-deflate",
	}, nil
}

// DecompressMessage decompresses a message if needed
func (cm *CompressionManager) DecompressMessage(data []byte, compressed bool) ([]byte, error) {
	if !compressed || !cm.config.Enabled {
		return data, nil
	}

	start := time.Now()
	defer func() {
		cm.stats.mu.Lock()
		cm.stats.DecompressionTime += time.Since(start)
		cm.stats.mu.Unlock()
	}()

	decompressed, err := cm.decompressData(data)
	if err != nil {
		cm.stats.mu.Lock()
		cm.stats.DecompressionErrors++
		cm.stats.mu.Unlock()

		return nil, pkgerrors.WithOperation("decompress", "message_data", err)
	}

	return decompressed, nil
}

// compressData compresses data using deflate
func (cm *CompressionManager) compressData(data []byte) ([]byte, error) {
	if cm.config.UsePooledCompressors && cm.compressorPool != nil {
		return cm.compressWithPool(data)
	}

	return cm.compressWithoutPool(data)
}

// compressWithPool compresses data using pooled compressor
func (cm *CompressionManager) compressWithPool(data []byte) ([]byte, error) {
	compressor := cm.compressorPool.Get().(*flate.Writer)
	if compressor == nil {
		return nil, fmt.Errorf("failed to get compressor from pool")
	}
	defer cm.compressorPool.Put(compressor)

	// Create a buffer to hold compressed data
	var buf []byte
	writer := &bytesWriter{buf: &buf}

	compressor.Reset(writer)

	if _, err := compressor.Write(data); err != nil {
		return nil, pkgerrors.WithOperation("write", "compressor", err)
	}

	if err := compressor.Close(); err != nil {
		return nil, pkgerrors.WithOperation("close", "compressor", err)
	}

	return buf, nil
}

// compressWithoutPool compresses data without pooling
func (cm *CompressionManager) compressWithoutPool(data []byte) ([]byte, error) {
	var buf []byte
	writer := &bytesWriter{buf: &buf}

	compressor, err := flate.NewWriter(writer, cm.config.CompressionLevel)
	if err != nil {
		return nil, pkgerrors.WithOperation("create", "compressor", err)
	}

	if _, err := compressor.Write(data); err != nil {
		compressor.Close()
		return nil, pkgerrors.WithOperation("write", "compressor", err)
	}

	if err := compressor.Close(); err != nil {
		return nil, pkgerrors.WithOperation("close", "compressor", err)
	}

	return buf, nil
}

// decompressData decompresses data using deflate
func (cm *CompressionManager) decompressData(data []byte) ([]byte, error) {
	if cm.config.UsePooledCompressors && cm.decompressorPool != nil {
		return cm.decompressWithPool(data)
	}

	return cm.decompressWithoutPool(data)
}

// decompressWithPool decompresses data using pooled decompressor
func (cm *CompressionManager) decompressWithPool(data []byte) ([]byte, error) {
	decompressor := cm.decompressorPool.Get().(io.ReadCloser)
	if decompressor == nil {
		return nil, fmt.Errorf("failed to get decompressor from pool")
	}
	defer cm.decompressorPool.Put(decompressor)

	reader := &bytesReader{data: data}

	// Reset the decompressor with new data
	if resetter, ok := decompressor.(flate.Resetter); ok {
		if err := resetter.Reset(reader, nil); err != nil {
			return nil, pkgerrors.WithOperation("reset", "decompressor", err)
		}
	} else {
		decompressor.Close()
		decompressor = flate.NewReader(reader)
	}

	var buf []byte
	writer := &bytesWriter{buf: &buf}

	if _, err := io.Copy(writer, decompressor); err != nil {
		return nil, pkgerrors.WithOperation("decompress", "data_stream", err)
	}

	return buf, nil
}

// decompressWithoutPool decompresses data without pooling
func (cm *CompressionManager) decompressWithoutPool(data []byte) ([]byte, error) {
	reader := &bytesReader{data: data}
	decompressor := flate.NewReader(reader)
	defer decompressor.Close()

	var buf []byte
	writer := &bytesWriter{buf: &buf}

	if _, err := io.Copy(writer, decompressor); err != nil {
		return nil, pkgerrors.WithOperation("decompress", "data_stream", err)
	}

	return buf, nil
}

// GetCompressionExtensions returns supported compression extensions
func (cm *CompressionManager) GetCompressionExtensions() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	extensions := make([]string, 0, len(cm.extensions))
	for ext := range cm.extensions {
		extensions = append(extensions, ext)
	}

	return extensions
}

// SupportsExtension checks if an extension is supported
func (cm *CompressionManager) SupportsExtension(extension string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.extensions[extension]
}

// GetStats returns compression statistics
func (cm *CompressionManager) GetStats() *CompressionStats {
	cm.stats.mu.RLock()
	defer cm.stats.mu.RUnlock()

	// Return a copy of the stats
	return &CompressionStats{
		TotalMessages:           cm.stats.TotalMessages,
		CompressedMessages:      cm.stats.CompressedMessages,
		UncompressedMessages:    cm.stats.UncompressedMessages,
		FailedCompressions:      cm.stats.FailedCompressions,
		TotalBytesIn:            cm.stats.TotalBytesIn,
		TotalBytesOut:           cm.stats.TotalBytesOut,
		BytesSaved:              cm.stats.BytesSaved,
		CompressionTime:         cm.stats.CompressionTime,
		DecompressionTime:       cm.stats.DecompressionTime,
		AverageCompressionRatio: cm.stats.AverageCompressionRatio,
		CompressionErrors:       cm.stats.CompressionErrors,
		DecompressionErrors:     cm.stats.DecompressionErrors,
		CurrentMemoryUsage:      cm.stats.CurrentMemoryUsage,
		PeakMemoryUsage:         cm.stats.PeakMemoryUsage,
	}
}

// ResetStats resets compression statistics
func (cm *CompressionManager) ResetStats() {
	cm.stats.mu.Lock()
	defer cm.stats.mu.Unlock()

	cm.stats.TotalMessages = 0
	cm.stats.CompressedMessages = 0
	cm.stats.UncompressedMessages = 0
	cm.stats.FailedCompressions = 0
	cm.stats.TotalBytesIn = 0
	cm.stats.TotalBytesOut = 0
	cm.stats.BytesSaved = 0
	cm.stats.CompressionTime = 0
	cm.stats.DecompressionTime = 0
	cm.stats.AverageCompressionRatio = 0
	cm.stats.CompressionErrors = 0
	cm.stats.DecompressionErrors = 0
	cm.stats.CurrentMemoryUsage = 0
	cm.stats.PeakMemoryUsage = 0
}

// startStatsCollection starts periodic statistics collection
func (cm *CompressionManager) startStatsCollection() {
	cm.statsTimer = time.NewTimer(cm.config.StatisticsInterval)

	cm.wg.Add(1)
	go func() {
		defer cm.wg.Done()
		defer cm.statsTimer.Stop() // Ensure timer is stopped
		
		for {
			select {
			case <-cm.statsTimer.C:
				cm.collectStats()
				cm.statsTimer.Reset(cm.config.StatisticsInterval)
			case <-cm.shutdownCh:
				return
			}
		}
	}()
}

// collectStats collects current statistics
func (cm *CompressionManager) collectStats() {
	// This could be extended to collect memory usage, GC stats, etc.
	// For now, it's a placeholder for future enhancements
}

// Shutdown gracefully shuts down the compression manager
func (cm *CompressionManager) Shutdown() {
	close(cm.shutdownCh)

	// Wait for all goroutines to finish
	cm.wg.Wait()

	if cm.statsTimer != nil {
		cm.statsTimer.Stop()
	}
}

// IsCompressionBeneficial determines if compression would be beneficial
func (cm *CompressionManager) IsCompressionBeneficial(data []byte) bool {
	if !cm.config.Enabled {
		return false
	}

	// Check size threshold
	if int64(len(data)) < cm.config.CompressionThreshold {
		return false
	}

	// Quick heuristic: check for patterns that compress well
	// This is a simple check - you could implement more sophisticated analysis

	// Count repeating patterns
	patterns := make(map[byte]int)
	for _, b := range data {
		patterns[b]++
	}

	// If there's high repetition, compression is likely beneficial
	maxCount := 0
	for _, count := range patterns {
		if count > maxCount {
			maxCount = count
		}
	}

	repetitionRatio := float64(maxCount) / float64(len(data))
	return repetitionRatio > 0.1 // 10% repetition threshold
}

// GetCompressionRatio estimates compression ratio for data
func (cm *CompressionManager) GetCompressionRatio(data []byte) float64 {
	if !cm.IsCompressionBeneficial(data) {
		return 1.0
	}

	// Quick estimation based on entropy
	// This is a simple heuristic - in practice, you might want to
	// actually compress a sample to get a better estimate

	patterns := make(map[byte]int)
	for _, b := range data {
		patterns[b]++
	}

	// Calculate Shannon entropy
	entropy := 0.0
	length := float64(len(data))

	for _, count := range patterns {
		if count > 0 {
			p := float64(count) / length
			entropy -= p * math.Log2(p)
		}
	}

	// Estimate compression ratio based on entropy
	// Higher entropy = worse compression
	maxEntropy := 8.0 // Maximum entropy for byte data
	normalizedEntropy := entropy / maxEntropy

	// Estimate compression ratio (lower is better)
	estimatedRatio := 0.3 + (0.7 * normalizedEntropy)

	return estimatedRatio
}

// Helper types for bytes manipulation

// bytesWriter implements io.Writer for byte slices
type bytesWriter struct {
	buf *[]byte
}

func (w *bytesWriter) Write(p []byte) (int, error) {
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}

// bytesReader implements io.Reader for byte slices
type bytesReader struct {
	data []byte
	pos  int
}

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}

	n := copy(p, r.data[r.pos:])
	r.pos += n

	return n, nil
}

// CompressionMiddleware wraps a WebSocket connection with compression
type CompressionMiddleware struct {
	conn    *websocket.Conn
	manager *CompressionManager
}

// NewCompressionMiddleware creates a new compression middleware
func NewCompressionMiddleware(conn *websocket.Conn, manager *CompressionManager) *CompressionMiddleware {
	return &CompressionMiddleware{
		conn:    conn,
		manager: manager,
	}
}

// WriteMessage writes a message with compression
func (cm *CompressionMiddleware) WriteMessage(messageType int, data []byte) error {
	compressedMsg, err := cm.manager.CompressMessage(data, messageType)
	if err != nil {
		return pkgerrors.WithOperation("compress", "message", err)
	}

	return cm.conn.WriteMessage(messageType, compressedMsg.Data)
}

// ReadMessage reads a message with decompression
func (cm *CompressionMiddleware) ReadMessage() (messageType int, p []byte, err error) {
	messageType, p, err = cm.conn.ReadMessage()
	if err != nil {
		return messageType, p, err
	}

	// Check if message is compressed based on WebSocket extension
	// Note: In a real implementation, check the message headers or extension negotiation
	compressed := false // Simplified for now

	if compressed {
		p, err = cm.manager.DecompressMessage(p, true)
		if err != nil {
			return messageType, p, pkgerrors.WithOperation("decompress", "message", err)
		}
	}

	return messageType, p, nil
}
