package spinner

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// ToolStatus represents the status of a tool in the execution queue
type ToolStatus int

const (
	ToolPending ToolStatus = iota
	ToolRunning
	ToolCompleted
	ToolFailed
)

// ToolInfo holds information about a tool in the execution queue
type ToolInfo struct {
	Name      string
	Status    ToolStatus
	StartTime time.Time
	EndTime   time.Time
	Error     error
}

// MultiToolSpinner provides visual feedback for multiple tool execution
type MultiToolSpinner struct {
	*Spinner
	mu            sync.RWMutex
	tools         []ToolInfo
	currentIndex  int
	parallelCount int
	startTime     time.Time
}

// NewMultiTool creates a new multi-tool execution spinner
func NewMultiTool(writer io.Writer, toolNames []string) *MultiToolSpinner {
	// Initialize tool info
	tools := make([]ToolInfo, len(toolNames))
	for i, name := range toolNames {
		tools[i] = ToolInfo{
			Name:   name,
			Status: ToolPending,
		}
	}
	
	config := Config{
		Writer:  writer,
		Message: fmt.Sprintf("Preparing %d tools", len(toolNames)),
		Style:   StyleCircle,
	}
	
	return &MultiToolSpinner{
		Spinner:       New(config),
		tools:         tools,
		currentIndex:  -1,
		parallelCount: 0,
		startTime:     time.Now(),
	}
}

// StartTool marks a tool as started
func (m *MultiToolSpinner) StartTool(toolName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for i := range m.tools {
		if m.tools[i].Name == toolName {
			m.tools[i].Status = ToolRunning
			m.tools[i].StartTime = time.Now()
			if m.currentIndex < i {
				m.currentIndex = i
			}
			m.parallelCount++
			break
		}
	}
	
	m.updateDisplay()
}

// CompleteTool marks a tool as completed
func (m *MultiToolSpinner) CompleteTool(toolName string, success bool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for i := range m.tools {
		if m.tools[i].Name == toolName {
			if success {
				m.tools[i].Status = ToolCompleted
			} else {
				m.tools[i].Status = ToolFailed
				m.tools[i].Error = err
			}
			m.tools[i].EndTime = time.Now()
			if m.parallelCount > 0 {
				m.parallelCount--
			}
			break
		}
	}
	
	m.updateDisplay()
}

// updateDisplay updates the spinner message with current progress
func (m *MultiToolSpinner) updateDisplay() {
	completed := 0
	running := 0
	failed := 0
	
	for _, tool := range m.tools {
		switch tool.Status {
		case ToolCompleted:
			completed++
		case ToolRunning:
			running++
		case ToolFailed:
			failed++
		}
	}
	
	// Build progress bar
	progressBar := m.buildProgressBar(completed, failed, len(m.tools))
	
	// Current tool info
	currentTool := ""
	if running > 0 {
		runningTools := []string{}
		for _, tool := range m.tools {
			if tool.Status == ToolRunning {
				runningTools = append(runningTools, tool.Name)
			}
		}
		if len(runningTools) == 1 {
			currentTool = fmt.Sprintf(" | Running: %s", runningTools[0])
		} else {
			currentTool = fmt.Sprintf(" | Running %d tools", len(runningTools))
		}
	}
	
	// Elapsed time
	elapsed := time.Since(m.startTime)
	
	// Build complete message
	message := fmt.Sprintf("[%d/%d] %s%s | %.1fs",
		completed+failed,
		len(m.tools),
		progressBar,
		currentTool,
		elapsed.Seconds(),
	)
	
	m.UpdateMessage(message)
}

// buildProgressBar creates a visual progress bar
func (m *MultiToolSpinner) buildProgressBar(completed, failed, total int) string {
	if total == 0 {
		return ""
	}
	
	const barWidth = 20
	completedWidth := (completed * barWidth) / total
	failedWidth := (failed * barWidth) / total
	pendingWidth := barWidth - completedWidth - failedWidth
	
	bar := strings.Builder{}
	
	// Completed portion (green)
	for i := 0; i < completedWidth; i++ {
		bar.WriteString("█")
	}
	
	// Failed portion (red) 
	for i := 0; i < failedWidth; i++ {
		bar.WriteString("▒")
	}
	
	// Pending portion (gray)
	for i := 0; i < pendingWidth; i++ {
		bar.WriteString("░")
	}
	
	return bar.String()
}

// CompleteAll stops the spinner with final summary
func (m *MultiToolSpinner) CompleteAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	completed := 0
	failed := 0
	failedNames := []string{}
	
	for _, tool := range m.tools {
		switch tool.Status {
		case ToolCompleted:
			completed++
		case ToolFailed:
			failed++
			failedNames = append(failedNames, tool.Name)
		}
	}
	
	elapsed := time.Since(m.startTime)
	
	var message string
	if failed == 0 {
		message = fmt.Sprintf("✅ All %d tools completed successfully | %.1fs",
			completed, elapsed.Seconds())
	} else if completed == 0 {
		message = fmt.Sprintf("❌ All %d tools failed | %.1fs",
			failed, elapsed.Seconds())
	} else {
		message = fmt.Sprintf("⚠️  %d completed, %d failed (%s) | %.1fs",
			completed, failed, strings.Join(failedNames, ", "), elapsed.Seconds())
	}
	
	m.StopWithMessage(message)
}

// GetToolStatus returns the current status of all tools
func (m *MultiToolSpinner) GetToolStatus() []ToolInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	result := make([]ToolInfo, len(m.tools))
	copy(result, m.tools)
	return result
}