package common

import (
	"bytes"
	"sync"
)

// BufferPool manages a pool of byte buffers for efficient memory reuse
type BufferPool struct {
	pool sync.Pool
}

// NewBufferPool creates a new buffer pool with the specified initial size
func NewBufferPool(initialSize int) *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 0, initialSize))
			},
		},
	}
}

// Get retrieves a buffer from the pool
func (bp *BufferPool) Get() *bytes.Buffer {
	return bp.pool.Get().(*bytes.Buffer)
}

// Put returns a buffer to the pool after resetting it
func (bp *BufferPool) Put(buf *bytes.Buffer) {
	buf.Reset()
	bp.pool.Put(buf)
}

// Default buffer pools for common sizes
var (
	// SmallBufferPool for buffers up to 1KB
	SmallBufferPool = NewBufferPool(1024)
	
	// MediumBufferPool for buffers up to 64KB
	MediumBufferPool = NewBufferPool(65536)
	
	// LargeBufferPool for buffers up to 1MB
	LargeBufferPool = NewBufferPool(1048576)
)

// GetPooledBuffer returns an appropriate buffer from the pool based on size hint
func GetPooledBuffer(sizeHint int) *bytes.Buffer {
	switch {
	case sizeHint <= 1024:
		return SmallBufferPool.Get()
	case sizeHint <= 65536:
		return MediumBufferPool.Get()
	default:
		return LargeBufferPool.Get()
	}
}

// ReturnPooledBuffer returns a buffer to the appropriate pool
func ReturnPooledBuffer(buf *bytes.Buffer) {
	capacity := buf.Cap()
	switch {
	case capacity <= 1024:
		SmallBufferPool.Put(buf)
	case capacity <= 65536:
		MediumBufferPool.Put(buf)
	default:
		LargeBufferPool.Put(buf)
	}
}

// SafeBuffer wraps a bytes.Buffer with thread-safe operations
type SafeBuffer struct {
	buf *bytes.Buffer
	mu  sync.RWMutex
}

// NewSafeBuffer creates a new thread-safe buffer
func NewSafeBuffer() *SafeBuffer {
	return &SafeBuffer{
		buf: new(bytes.Buffer),
	}
}

// NewSafeBufferFromBytes creates a new thread-safe buffer from existing bytes
func NewSafeBufferFromBytes(data []byte) *SafeBuffer {
	return &SafeBuffer{
		buf: bytes.NewBuffer(data),
	}
}

// Write writes data to the buffer
func (sb *SafeBuffer) Write(p []byte) (int, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.Write(p)
}

// WriteString writes a string to the buffer
func (sb *SafeBuffer) WriteString(s string) (int, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.WriteString(s)
}

// WriteByte writes a single byte to the buffer
func (sb *SafeBuffer) WriteByte(c byte) error {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.WriteByte(c)
}

// Read reads data from the buffer
func (sb *SafeBuffer) Read(p []byte) (int, error) {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.buf.Read(p)
}

// ReadByte reads a single byte from the buffer
func (sb *SafeBuffer) ReadByte() (byte, error) {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.buf.ReadByte()
}

// Bytes returns a copy of the buffer's contents
func (sb *SafeBuffer) Bytes() []byte {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.buf.Bytes()
}

// String returns the buffer's contents as a string
func (sb *SafeBuffer) String() string {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.buf.String()
}

// Len returns the length of the buffer
func (sb *SafeBuffer) Len() int {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.buf.Len()
}

// Cap returns the capacity of the buffer
func (sb *SafeBuffer) Cap() int {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.buf.Cap()
}

// Reset resets the buffer
func (sb *SafeBuffer) Reset() {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.buf.Reset()
}

// Grow grows the buffer's capacity
func (sb *SafeBuffer) Grow(n int) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.buf.Grow(n)
}

// BufferWriter provides utilities for writing to buffers
type BufferWriter struct {
	buf *bytes.Buffer
}

// NewBufferWriter creates a new buffer writer
func NewBufferWriter(buf *bytes.Buffer) *BufferWriter {
	return &BufferWriter{buf: buf}
}

// WriteUint32 writes a uint32 in big-endian format
func (bw *BufferWriter) WriteUint32(value uint32) error {
	return bw.buf.WriteByte(byte(value>>24))
}

// WriteUint16 writes a uint16 in big-endian format
func (bw *BufferWriter) WriteUint16(value uint16) error {
	if err := bw.buf.WriteByte(byte(value >> 8)); err != nil {
		return err
	}
	return bw.buf.WriteByte(byte(value))
}

// WriteUint8 writes a uint8
func (bw *BufferWriter) WriteUint8(value uint8) error {
	return bw.buf.WriteByte(byte(value))
}

// WriteBytes writes a byte slice with length prefix
func (bw *BufferWriter) WriteBytes(data []byte) error {
	if err := bw.WriteUint32(uint32(len(data))); err != nil {
		return err
	}
	_, err := bw.buf.Write(data)
	return err
}

// WriteString writes a string with length prefix
func (bw *BufferWriter) WriteString(s string) error {
	return bw.WriteBytes([]byte(s))
}

// BufferReader provides utilities for reading from buffers
type BufferReader struct {
	buf *bytes.Buffer
}

// NewBufferReader creates a new buffer reader
func NewBufferReader(buf *bytes.Buffer) *BufferReader {
	return &BufferReader{buf: buf}
}

// ReadUint32 reads a uint32 in big-endian format
func (br *BufferReader) ReadUint32() (uint32, error) {
	if br.buf.Len() < 4 {
		return 0, ErrBufferTooSmall
	}
	
	var value uint32
	for i := 0; i < 4; i++ {
		b, err := br.buf.ReadByte()
		if err != nil {
			return 0, err
		}
		value = (value << 8) | uint32(b)
	}
	
	return value, nil
}

// ReadUint16 reads a uint16 in big-endian format
func (br *BufferReader) ReadUint16() (uint16, error) {
	if br.buf.Len() < 2 {
		return 0, ErrBufferTooSmall
	}
	
	var value uint16
	for i := 0; i < 2; i++ {
		b, err := br.buf.ReadByte()
		if err != nil {
			return 0, err
		}
		value = (value << 8) | uint16(b)
	}
	
	return value, nil
}

// ReadUint8 reads a uint8
func (br *BufferReader) ReadUint8() (uint8, error) {
	b, err := br.buf.ReadByte()
	return uint8(b), err
}

// ReadBytes reads a byte slice with length prefix
func (br *BufferReader) ReadBytes() ([]byte, error) {
	length, err := br.ReadUint32()
	if err != nil {
		return nil, err
	}
	
	if br.buf.Len() < int(length) {
		return nil, ErrBufferTooSmall
	}
	
	data := make([]byte, length)
	_, err = br.buf.Read(data)
	return data, err
}

// ReadString reads a string with length prefix
func (br *BufferReader) ReadString() (string, error) {
	data, err := br.ReadBytes()
	if err != nil {
		return "", err
	}
	return string(data), nil
}