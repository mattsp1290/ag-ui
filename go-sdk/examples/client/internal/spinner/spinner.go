package spinner

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// Spinner provides a visual feedback indicator for long-running operations
type Spinner struct {
	frames   []string
	delay    time.Duration
	writer   io.Writer
	mu       sync.Mutex
	active   bool
	stopChan chan struct{}
	message  string
	started  time.Time
}

// Config holds spinner configuration
type Config struct {
	Writer  io.Writer
	Message string
	Style   Style
}

// Style represents different spinner styles
type Style int

const (
	StyleDots Style = iota
	StyleLine
	StyleCircle
	StyleSquare
	StyleBraille
)

var styles = map[Style][]string{
	StyleDots:    {"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	StyleLine:    {"-", "\\", "|", "/"},
	StyleCircle:  {"◐", "◓", "◑", "◒"},
	StyleSquare:  {"◰", "◳", "◲", "◱"},
	StyleBraille: {"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
}

// New creates a new spinner with the given configuration
func New(config Config) *Spinner {
	frames := styles[config.Style]
	if frames == nil {
		frames = styles[StyleDots]
	}

	return &Spinner{
		frames:  frames,
		delay:   100 * time.Millisecond,
		writer:  config.Writer,
		message: config.Message,
	}
}

// Start begins the spinner animation
func (s *Spinner) Start() {
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return
	}
	s.active = true
	s.stopChan = make(chan struct{})
	s.started = time.Now()
	s.mu.Unlock()

	go s.run()
}

// Stop stops the spinner animation
func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active {
		return
	}

	s.active = false
	close(s.stopChan)
	
	// Clear the spinner line
	fmt.Fprint(s.writer, "\r\033[K")
}

// StopWithMessage stops the spinner and displays a final message
func (s *Spinner) StopWithMessage(message string) {
	s.Stop()
	fmt.Fprintln(s.writer, message)
}

// UpdateMessage updates the spinner message while it's running
func (s *Spinner) UpdateMessage(message string) {
	s.mu.Lock()
	s.message = message
	s.mu.Unlock()
}

// run is the internal animation loop
func (s *Spinner) run() {
	ticker := time.NewTicker(s.delay)
	defer ticker.Stop()

	frameIndex := 0
	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.mu.Lock()
			if s.active {
				elapsed := time.Since(s.started)
				frame := s.frames[frameIndex]
				message := s.message
				
				// Format the output with elapsed time
				output := fmt.Sprintf("\r%s %s", frame, message)
				if elapsed > 1*time.Second {
					output = fmt.Sprintf("\r%s %s (%.1fs)", frame, message, elapsed.Seconds())
				}
				
				fmt.Fprint(s.writer, output)
				frameIndex = (frameIndex + 1) % len(s.frames)
			}
			s.mu.Unlock()
		}
	}
}

// ProgressSpinner extends Spinner with progress tracking
type ProgressSpinner struct {
	*Spinner
	total   int
	current int
}

// NewProgress creates a new progress spinner
func NewProgress(config Config, total int) *ProgressSpinner {
	return &ProgressSpinner{
		Spinner: New(config),
		total:   total,
	}
}

// Update updates the progress
func (p *ProgressSpinner) Update(current int) {
	p.current = current
	percentage := float64(current) / float64(p.total) * 100
	p.UpdateMessage(fmt.Sprintf("%s [%d/%d] %.0f%%", p.message, current, p.total, percentage))
}

// ToolExecutionSpinner provides specialized feedback for tool execution
type ToolExecutionSpinner struct {
	*Spinner
	toolName string
}

// NewToolExecution creates a spinner for tool execution
func NewToolExecution(writer io.Writer, toolName string) *ToolExecutionSpinner {
	config := Config{
		Writer:  writer,
		Message: fmt.Sprintf("Executing %s", toolName),
		Style:   StyleDots,
	}
	
	return &ToolExecutionSpinner{
		Spinner:  New(config),
		toolName: toolName,
	}
}

// StartWithPhase starts the spinner with a specific phase message
func (t *ToolExecutionSpinner) StartWithPhase(phase string) {
	t.UpdateMessage(fmt.Sprintf("Executing %s: %s", t.toolName, phase))
	t.Start()
}

// CompleteWithResult stops the spinner and shows the result
func (t *ToolExecutionSpinner) CompleteWithResult(success bool) {
	if success {
		t.StopWithMessage(fmt.Sprintf("✅ %s completed successfully", t.toolName))
	} else {
		t.StopWithMessage(fmt.Sprintf("❌ %s failed", t.toolName))
	}
}