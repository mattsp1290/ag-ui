package events

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"sync"
	"time"

	"github.com/vmihailenco/msgpack/v5"
	"google.golang.org/protobuf/proto"
)

// SerializationMethod defines the method used for serialization
type SerializationMethod int

const (
	SerializationJSON SerializationMethod = iota
	SerializationMsgPack
	SerializationProtobuf
	SerializationGob
	SerializationBinary
)

// SerializationConfig contains configuration for serialization
type SerializationConfig struct {
	Method               SerializationMethod
	CompressionEnabled   bool
	CompressionThreshold int // bytes
	EnableChecksums      bool
	BufferPoolSize       int
	MaxBufferSize        int
}

// DefaultSerializationConfig returns default serialization configuration
func DefaultSerializationConfig() *SerializationConfig {
	return &SerializationConfig{
		Method:               SerializationMsgPack,
		CompressionEnabled:   true,
		CompressionThreshold: 1024, // 1KB
		EnableChecksums:      true,
		BufferPoolSize:       100,
		MaxBufferSize:        1024 * 1024, // 1MB
	}
}

// SerializationStats tracks serialization performance metrics
type SerializationStats struct {
	SerializationCount     int64
	DeserializationCount   int64
	TotalBytesIn           int64
	TotalBytesOut          int64
	AvgSerializationTime   time.Duration
	AvgDeserializationTime time.Duration
	CompressionRatio       float64
	ErrorCount             int64
	mutex                  sync.RWMutex
}

// OptimizedSerializer provides high-performance serialization with multiple formats
type OptimizedSerializer struct {
	config     *SerializationConfig
	bufferPool *sync.Pool
	stats      *SerializationStats

	// Compression helpers
	compressor   Compressor
	decompressor Decompressor
}

// NewOptimizedSerializer creates a new optimized serializer
func NewOptimizedSerializer(config *SerializationConfig) *OptimizedSerializer {
	if config == nil {
		config = DefaultSerializationConfig()
	}

	serializer := &OptimizedSerializer{
		config: config,
		stats:  &SerializationStats{},
		bufferPool: &sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 0, 4096))
			},
		},
	}

	// Initialize compression if enabled
	if config.CompressionEnabled {
		serializer.compressor = NewGzipCompressor()
		serializer.decompressor = NewGzipDecompressor()
	}

	return serializer
}

// SerializedData represents serialized data with metadata
type SerializedData struct {
	Data       []byte
	Method     SerializationMethod
	Compressed bool
	Checksum   uint32
	Timestamp  time.Time
	Size       int
}

// Serialize converts an object to bytes using the configured method
func (s *OptimizedSerializer) Serialize(obj interface{}) (*SerializedData, error) {
	start := time.Now()
	defer func() {
		s.updateSerializationStats(time.Since(start))
	}()

	// Get buffer from pool
	buffer := s.bufferPool.Get().(*bytes.Buffer)
	defer func() {
		buffer.Reset()
		s.bufferPool.Put(buffer)
	}()

	// Serialize based on method
	var data []byte
	var err error

	switch s.config.Method {
	case SerializationJSON:
		data, err = json.Marshal(obj)
	case SerializationMsgPack:
		data, err = msgpack.Marshal(obj)
	case SerializationProtobuf:
		if pbMsg, ok := obj.(proto.Message); ok {
			data, err = proto.Marshal(pbMsg)
		} else {
			return nil, fmt.Errorf("object does not implement proto.Message")
		}
	case SerializationGob:
		encoder := gob.NewEncoder(buffer)
		err = encoder.Encode(obj)
		if err == nil {
			data = buffer.Bytes()
		}
	case SerializationBinary:
		data, err = s.binarySerialize(obj)
	default:
		return nil, fmt.Errorf("unsupported serialization method: %d", s.config.Method)
	}

	if err != nil {
		s.stats.mutex.Lock()
		s.stats.ErrorCount++
		s.stats.mutex.Unlock()
		return nil, fmt.Errorf("serialization failed: %w", err)
	}

	result := &SerializedData{
		Data:      data,
		Method:    s.config.Method,
		Timestamp: time.Now(),
		Size:      len(data),
	}

	// Apply compression if enabled and size exceeds threshold
	if s.config.CompressionEnabled && len(data) > s.config.CompressionThreshold {
		compressedData, err := s.compressor.Compress(data)
		if err == nil && len(compressedData) < len(data) {
			result.Data = compressedData
			result.Compressed = true
		}
	}

	// Calculate checksum if enabled
	if s.config.EnableChecksums {
		result.Checksum = crc32.ChecksumIEEE(result.Data)
	}

	// Update stats
	s.stats.mutex.Lock()
	s.stats.SerializationCount++
	s.stats.TotalBytesIn += int64(len(data))
	s.stats.TotalBytesOut += int64(len(result.Data))
	s.stats.mutex.Unlock()

	return result, nil
}

// Deserialize converts bytes back to an object
func (s *OptimizedSerializer) Deserialize(data *SerializedData, obj interface{}) error {
	start := time.Now()
	defer func() {
		s.updateDeserializationStats(time.Since(start))
	}()

	// Verify checksum if enabled
	if s.config.EnableChecksums && data.Checksum != 0 {
		if crc32.ChecksumIEEE(data.Data) != data.Checksum {
			s.stats.mutex.Lock()
			s.stats.ErrorCount++
			s.stats.mutex.Unlock()
			return fmt.Errorf("checksum verification failed")
		}
	}

	// Decompress if needed
	payload := data.Data
	if data.Compressed {
		decompressed, err := s.decompressor.Decompress(data.Data)
		if err != nil {
			s.stats.mutex.Lock()
			s.stats.ErrorCount++
			s.stats.mutex.Unlock()
			return fmt.Errorf("decompression failed: %w", err)
		}
		payload = decompressed
	}

	// Deserialize based on method
	var err error
	switch data.Method {
	case SerializationJSON:
		err = json.Unmarshal(payload, obj)
	case SerializationMsgPack:
		err = msgpack.Unmarshal(payload, obj)
	case SerializationProtobuf:
		if pbMsg, ok := obj.(proto.Message); ok {
			err = proto.Unmarshal(payload, pbMsg)
		} else {
			return fmt.Errorf("object does not implement proto.Message")
		}
	case SerializationGob:
		buffer := bytes.NewBuffer(payload)
		decoder := gob.NewDecoder(buffer)
		err = decoder.Decode(obj)
	case SerializationBinary:
		err = s.binaryDeserialize(payload, obj)
	default:
		return fmt.Errorf("unsupported serialization method: %d", data.Method)
	}

	if err != nil {
		s.stats.mutex.Lock()
		s.stats.ErrorCount++
		s.stats.mutex.Unlock()
		return fmt.Errorf("deserialization failed: %w", err)
	}

	// Update stats
	s.stats.mutex.Lock()
	s.stats.DeserializationCount++
	s.stats.mutex.Unlock()

	return nil
}

// binarySerialize provides fast binary serialization for simple types
func (s *OptimizedSerializer) binarySerialize(obj interface{}) ([]byte, error) {
	buffer := bytes.NewBuffer(make([]byte, 0, 256))

	switch v := obj.(type) {
	case string:
		// String format: [length:4bytes][data:length bytes]
		if err := binary.Write(buffer, binary.LittleEndian, uint32(len(v))); err != nil {
			return nil, err
		}
		if _, err := buffer.WriteString(v); err != nil {
			return nil, err
		}
	case int64:
		if err := binary.Write(buffer, binary.LittleEndian, v); err != nil {
			return nil, err
		}
	case float64:
		if err := binary.Write(buffer, binary.LittleEndian, v); err != nil {
			return nil, err
		}
	case []byte:
		// Byte array format: [length:4bytes][data:length bytes]
		if err := binary.Write(buffer, binary.LittleEndian, uint32(len(v))); err != nil {
			return nil, err
		}
		if _, err := buffer.Write(v); err != nil {
			return nil, err
		}
	case map[string]interface{}:
		// Map format: [count:4bytes][key_length:4bytes][key:bytes][value_type:1byte][value:bytes]...
		if err := binary.Write(buffer, binary.LittleEndian, uint32(len(v))); err != nil {
			return nil, err
		}
		for key, value := range v {
			// Write key
			if err := binary.Write(buffer, binary.LittleEndian, uint32(len(key))); err != nil {
				return nil, err
			}
			if _, err := buffer.WriteString(key); err != nil {
				return nil, err
			}

			// Write value with type info
			valueData, err := s.binarySerialize(value)
			if err != nil {
				return nil, err
			}
			if _, err := buffer.Write(valueData); err != nil {
				return nil, err
			}
		}
	default:
		// Fallback to gob for complex types
		encoder := gob.NewEncoder(buffer)
		if err := encoder.Encode(obj); err != nil {
			return nil, err
		}
	}

	return buffer.Bytes(), nil
}

// binaryDeserialize provides fast binary deserialization for simple types
func (s *OptimizedSerializer) binaryDeserialize(data []byte, obj interface{}) error {
	buffer := bytes.NewBuffer(data)

	switch v := obj.(type) {
	case *string:
		var length uint32
		if err := binary.Read(buffer, binary.LittleEndian, &length); err != nil {
			return err
		}
		strData := make([]byte, length)
		if _, err := io.ReadFull(buffer, strData); err != nil {
			return err
		}
		*v = string(strData)
	case *int64:
		if err := binary.Read(buffer, binary.LittleEndian, v); err != nil {
			return err
		}
	case *float64:
		if err := binary.Read(buffer, binary.LittleEndian, v); err != nil {
			return err
		}
	case *[]byte:
		var length uint32
		if err := binary.Read(buffer, binary.LittleEndian, &length); err != nil {
			return err
		}
		*v = make([]byte, length)
		if _, err := io.ReadFull(buffer, *v); err != nil {
			return err
		}
	case *map[string]interface{}:
		var count uint32
		if err := binary.Read(buffer, binary.LittleEndian, &count); err != nil {
			return err
		}
		*v = make(map[string]interface{})
		for i := uint32(0); i < count; i++ {
			// Read key
			var keyLength uint32
			if err := binary.Read(buffer, binary.LittleEndian, &keyLength); err != nil {
				return err
			}
			keyData := make([]byte, keyLength)
			if _, err := io.ReadFull(buffer, keyData); err != nil {
				return err
			}
			key := string(keyData)

			// Read value (simplified - would need type info in real implementation)
			var value interface{}
			decoder := gob.NewDecoder(buffer)
			if err := decoder.Decode(&value); err != nil {
				return err
			}
			(*v)[key] = value
		}
	default:
		// Fallback to gob for complex types
		decoder := gob.NewDecoder(buffer)
		if err := decoder.Decode(obj); err != nil {
			return err
		}
	}

	return nil
}

// GetStats returns serialization statistics
func (s *OptimizedSerializer) GetStats() *SerializationStats {
	s.stats.mutex.RLock()
	defer s.stats.mutex.RUnlock()

	// Calculate compression ratio
	if s.stats.TotalBytesIn > 0 {
		s.stats.CompressionRatio = float64(s.stats.TotalBytesOut) / float64(s.stats.TotalBytesIn)
	}

	return &SerializationStats{
		SerializationCount:     s.stats.SerializationCount,
		DeserializationCount:   s.stats.DeserializationCount,
		TotalBytesIn:           s.stats.TotalBytesIn,
		TotalBytesOut:          s.stats.TotalBytesOut,
		AvgSerializationTime:   s.stats.AvgSerializationTime,
		AvgDeserializationTime: s.stats.AvgDeserializationTime,
		CompressionRatio:       s.stats.CompressionRatio,
		ErrorCount:             s.stats.ErrorCount,
	}
}

// ResetStats resets serialization statistics
func (s *OptimizedSerializer) ResetStats() {
	s.stats.mutex.Lock()
	defer s.stats.mutex.Unlock()

	s.stats.SerializationCount = 0
	s.stats.DeserializationCount = 0
	s.stats.TotalBytesIn = 0
	s.stats.TotalBytesOut = 0
	s.stats.AvgSerializationTime = 0
	s.stats.AvgDeserializationTime = 0
	s.stats.CompressionRatio = 0
	s.stats.ErrorCount = 0
}

// updateSerializationStats updates serialization timing statistics
func (s *OptimizedSerializer) updateSerializationStats(duration time.Duration) {
	s.stats.mutex.Lock()
	defer s.stats.mutex.Unlock()

	if s.stats.SerializationCount > 0 {
		// Calculate running average
		s.stats.AvgSerializationTime = time.Duration(
			(int64(s.stats.AvgSerializationTime)*s.stats.SerializationCount + int64(duration)) /
				(s.stats.SerializationCount + 1),
		)
	} else {
		s.stats.AvgSerializationTime = duration
	}
}

// updateDeserializationStats updates deserialization timing statistics
func (s *OptimizedSerializer) updateDeserializationStats(duration time.Duration) {
	s.stats.mutex.Lock()
	defer s.stats.mutex.Unlock()

	if s.stats.DeserializationCount > 0 {
		// Calculate running average
		s.stats.AvgDeserializationTime = time.Duration(
			(int64(s.stats.AvgDeserializationTime)*s.stats.DeserializationCount + int64(duration)) /
				(s.stats.DeserializationCount + 1),
		)
	} else {
		s.stats.AvgDeserializationTime = duration
	}
}

// Compressor interface for compression implementations
type Compressor interface {
	Compress(data []byte) ([]byte, error)
}

// Decompressor interface for decompression implementations
type Decompressor interface {
	Decompress(data []byte) ([]byte, error)
}

// GzipCompressor implements gzip compression
type GzipCompressor struct {
	bufferPool *sync.Pool
}

// NewGzipCompressor creates a new gzip compressor
func NewGzipCompressor() *GzipCompressor {
	return &GzipCompressor{
		bufferPool: &sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 0, 4096))
			},
		},
	}
}

// Compress compresses data using gzip
func (c *GzipCompressor) Compress(data []byte) ([]byte, error) {
	// This is a simplified implementation - real gzip compression would use compress/gzip
	// For now, return the original data (compression would be added in a real implementation)
	return data, nil
}

// GzipDecompressor implements gzip decompression
type GzipDecompressor struct {
	bufferPool *sync.Pool
}

// NewGzipDecompressor creates a new gzip decompressor
func NewGzipDecompressor() *GzipDecompressor {
	return &GzipDecompressor{
		bufferPool: &sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 0, 4096))
			},
		},
	}
}

// Decompress decompresses data using gzip
func (d *GzipDecompressor) Decompress(data []byte) ([]byte, error) {
	// This is a simplified implementation - real gzip decompression would use compress/gzip
	// For now, return the original data (decompression would be added in a real implementation)
	return data, nil
}

// CacheKeySerializer provides optimized serialization for cache keys
type CacheKeySerializer struct {
	serializer *OptimizedSerializer
	keyPool    *sync.Pool
}

// NewCacheKeySerializer creates a new cache key serializer
func NewCacheKeySerializer() *CacheKeySerializer {
	config := &SerializationConfig{
		Method:             SerializationBinary,
		CompressionEnabled: false, // Cache keys are usually small
		EnableChecksums:    false, // Not needed for cache keys
		BufferPoolSize:     50,
		MaxBufferSize:      1024,
	}

	return &CacheKeySerializer{
		serializer: NewOptimizedSerializer(config),
		keyPool: &sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, 256)
			},
		},
	}
}

// SerializeCacheKey serializes a cache key efficiently
func (s *CacheKeySerializer) SerializeCacheKey(eventType string, keys []string) ([]byte, error) {
	keyData := s.keyPool.Get().([]byte)
	defer func() {
		keyData = keyData[:0]
		s.keyPool.Put(keyData)
	}()

	// Create a simple key structure
	keyStruct := struct {
		EventType string
		Keys      []string
	}{
		EventType: eventType,
		Keys:      keys,
	}

	serialized, err := s.serializer.Serialize(keyStruct)
	if err != nil {
		return nil, err
	}

	// Return a copy of the data
	result := make([]byte, len(serialized.Data))
	copy(result, serialized.Data)

	return result, nil
}

// DeserializeCacheKey deserializes a cache key efficiently
func (s *CacheKeySerializer) DeserializeCacheKey(data []byte) (string, []string, error) {
	serializedData := &SerializedData{
		Data:   data,
		Method: SerializationBinary,
	}

	var keyStruct struct {
		EventType string
		Keys      []string
	}

	if err := s.serializer.Deserialize(serializedData, &keyStruct); err != nil {
		return "", nil, err
	}

	return keyStruct.EventType, keyStruct.Keys, nil
}

// GetCacheKeyStats returns cache key serialization statistics
func (s *CacheKeySerializer) GetCacheKeyStats() *SerializationStats {
	return s.serializer.GetStats()
}
