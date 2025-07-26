package memory

import (
	"sync"
)

// Slice is a thread-safe slice implementation
type Slice struct {
	mu    sync.RWMutex
	items []interface{}
}

// NewSlice creates a new thread-safe slice
func NewSlice() *Slice {
	return &Slice{
		items: make([]interface{}, 0),
	}
}

// Append adds an item to the slice
func (s *Slice) Append(item interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, item)
}

// RemoveFunc removes the first item that matches the predicate
func (s *Slice) RemoveFunc(f func(interface{}) bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for i, item := range s.items {
		if f(item) {
			// Remove item efficiently
			s.items = append(s.items[:i], s.items[i+1:]...)
			return true
		}
	}
	return false
}

// RemoveAt removes an item at the specified index
func (s *Slice) RemoveAt(index int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if index < 0 || index >= len(s.items) {
		return false
	}
	
	s.items = append(s.items[:index], s.items[index+1:]...)
	return true
}

// Get returns the item at the specified index
func (s *Slice) Get(index int) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if index < 0 || index >= len(s.items) {
		return nil, false
	}
	
	return s.items[index], true
}

// Len returns the length of the slice
func (s *Slice) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

// Range calls f for each item in the slice
func (s *Slice) Range(f func(interface{}) bool) {
	s.mu.RLock()
	// Make a copy to avoid holding the lock during callback
	items := make([]interface{}, len(s.items))
	copy(items, s.items)
	s.mu.RUnlock()
	
	for _, item := range items {
		if !f(item) {
			break
		}
	}
}

// Clear removes all items from the slice
func (s *Slice) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = s.items[:0]
}

// ToSlice returns a copy of the slice
func (s *Slice) ToSlice() []interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	result := make([]interface{}, len(s.items))
	copy(result, s.items)
	return result
}

// Filter returns a new slice containing only items that match the predicate
func (s *Slice) Filter(f func(interface{}) bool) *Slice {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	result := NewSlice()
	for _, item := range s.items {
		if f(item) {
			result.items = append(result.items, item)
		}
	}
	
	return result
}

// Map applies a function to each item and returns a new slice
func (s *Slice) Map(f func(interface{}) interface{}) *Slice {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	result := NewSlice()
	result.items = make([]interface{}, len(s.items))
	for i, item := range s.items {
		result.items[i] = f(item)
	}
	
	return result
}

// Any returns true if any item matches the predicate
func (s *Slice) Any(f func(interface{}) bool) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	for _, item := range s.items {
		if f(item) {
			return true
		}
	}
	return false
}

// All returns true if all items match the predicate
func (s *Slice) All(f func(interface{}) bool) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	for _, item := range s.items {
		if !f(item) {
			return false
		}
	}
	return true
}

// Find returns the first item that matches the predicate
func (s *Slice) Find(f func(interface{}) bool) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	for _, item := range s.items {
		if f(item) {
			return item, true
		}
	}
	return nil, false
}

// FindIndex returns the index of the first item that matches the predicate
func (s *Slice) FindIndex(f func(interface{}) bool) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	for i, item := range s.items {
		if f(item) {
			return i
		}
	}
	return -1
}

// Count returns the number of items that match the predicate
func (s *Slice) Count(f func(interface{}) bool) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	count := 0
	for _, item := range s.items {
		if f(item) {
			count++
		}
	}
	return count
}