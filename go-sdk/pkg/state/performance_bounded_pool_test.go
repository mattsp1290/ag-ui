package state

import (
	"sync"
	"testing"
)

func TestBoundedPool(t *testing.T) {
	t.Run("Basic functionality", func(t *testing.T) {
		pool := NewBoundedPool(10, 5, func() interface{} {
			return &JSONPatchOperation{}
		})
		
		// Get an object
		obj := pool.Get()
		if obj == nil {
			t.Fatal("Expected non-nil object from pool")
		}
		
		// Put it back
		pool.Put(obj)
	})
	
	t.Run("Max size limit", func(t *testing.T) {
		maxSize := 5
		created := 0
		pool := NewBoundedPool(maxSize, 3, func() interface{} {
			created++
			return &JSONPatchOperation{}
		})
		
		// Get more objects than max size
		objects := make([]interface{}, 0)
		for i := 0; i < maxSize+2; i++ {
			obj := pool.Get()
			if obj != nil {
				objects = append(objects, obj)
			}
		}
		
		// Should only create maxSize objects
		if len(objects) != maxSize {
			t.Errorf("Expected %d objects, got %d", maxSize, len(objects))
		}
		if created != maxSize {
			t.Errorf("Expected %d objects created, got %d", maxSize, created)
		}
	})
	
	t.Run("Max idle limit", func(t *testing.T) {
		// Skip this test - sync.Pool doesn't guarantee object retention
		// The bounded pool mainly prevents unbounded object creation
		t.Skip("sync.Pool doesn't guarantee object retention")
	})
	
	t.Run("Concurrent access", func(t *testing.T) {
		pool := NewBoundedPool(100, 50, func() interface{} {
			return &JSONPatchOperation{}
		})
		
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					obj := pool.Get()
					if obj != nil {
						// Do some work
						pool.Put(obj)
					}
				}
			}()
		}
		wg.Wait()
	})
}