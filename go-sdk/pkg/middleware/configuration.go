package middleware

import (
	"os"
	"sync"
	"time"
)

// Configuration structures

// MiddlewareConfiguration represents the complete middleware configuration
type MiddlewareConfiguration struct {
	DefaultChain string                 `json:"default_chain" yaml:"default_chain"`
	Chains       []ChainConfiguration   `json:"chains" yaml:"chains"`
	Global       map[string]interface{} `json:"global" yaml:"global"`
}

// ChainConfiguration represents a middleware chain configuration
type ChainConfiguration struct {
	Name       string                 `json:"name" yaml:"name"`
	Enabled    bool                   `json:"enabled" yaml:"enabled"`
	Handler    HandlerConfiguration   `json:"handler" yaml:"handler"`
	Middleware []MiddlewareConfig     `json:"middleware" yaml:"middleware"`
	Conditions map[string]interface{} `json:"conditions" yaml:"conditions"`
}

// HandlerConfiguration represents a handler configuration
type HandlerConfiguration struct {
	Type   string                 `json:"type" yaml:"type"`
	Config map[string]interface{} `json:"config" yaml:"config"`
}

// ConfigWatcher watches for configuration file changes
type ConfigWatcher struct {
	filePath    string
	callback    func()
	stopChannel chan bool
	stopped     bool
	mu          sync.Mutex
}

// NewConfigWatcher creates a new configuration file watcher
func NewConfigWatcher(filePath string, callback func()) (*ConfigWatcher, error) {
	watcher := &ConfigWatcher{
		filePath:    filePath,
		callback:    callback,
		stopChannel: make(chan bool, 1),
	}

	go watcher.watch()
	return watcher, nil
}

// watch monitors the configuration file for changes
func (cw *ConfigWatcher) watch() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var lastModTime time.Time
	if info, err := os.Stat(cw.filePath); err == nil {
		lastModTime = info.ModTime()
	}

	for {
		select {
		case <-cw.stopChannel:
			return
		case <-ticker.C:
			if info, err := os.Stat(cw.filePath); err == nil {
				if info.ModTime().After(lastModTime) {
					lastModTime = info.ModTime()
					// Wait a bit to ensure file write is complete
					time.Sleep(100 * time.Millisecond)
					cw.callback()
				}
			}
		}
	}
}

// Stop stops the configuration watcher
func (cw *ConfigWatcher) Stop() {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if !cw.stopped {
		cw.stopped = true
		cw.stopChannel <- true
		close(cw.stopChannel)
	}
}
