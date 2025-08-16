package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// BoxStyle defines the characters used for drawing boxes
type BoxStyle struct {
	TopLeft     string
	TopRight    string
	BottomLeft  string
	BottomRight string
	Horizontal  string
	Vertical    string
	MiddleLeft  string
	MiddleRight string
	Middle      string
}

var (
	// RoundedStyle uses rounded corners
	RoundedStyle = BoxStyle{
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "╰",
		BottomRight: "╯",
		Horizontal:  "─",
		Vertical:    "│",
		MiddleLeft:  "├",
		MiddleRight: "┤",
		Middle:      "─",
	}
	
	// ClassicStyle uses classic box drawing characters
	ClassicStyle = BoxStyle{
		TopLeft:     "┌",
		TopRight:    "┐",
		BottomLeft:  "└",
		BottomRight: "┘",
		Horizontal:  "─",
		Vertical:    "│",
		MiddleLeft:  "├",
		MiddleRight: "┤",
		Middle:      "─",
	}
)

// Box represents a terminal box with content
type Box struct {
	Title   string
	Content []string
	Width   int
	Style   BoxStyle
	Padding int
}

// NewBox creates a new box with default settings
func NewBox(title string, content []string) *Box {
	return &Box{
		Title:   title,
		Content: content,
		Width:   45, // Default width for haiku display
		Style:   RoundedStyle,
		Padding: 1,
	}
}

// Render returns the box as a string
func (b *Box) Render() string {
	var lines []string
	
	// Calculate actual width if not set
	if b.Width == 0 {
		b.Width = b.calculateWidth()
	}
	
	// Top border with title
	lines = append(lines, b.renderTopBorder())
	
	// Title section if present
	if b.Title != "" {
		lines = append(lines, b.renderTitleLine())
		lines = append(lines, b.renderDivider())
	}
	
	// Content lines
	for _, line := range b.Content {
		lines = append(lines, b.renderContentLine(line))
	}
	
	// Bottom border
	lines = append(lines, b.renderBottomBorder())
	
	return strings.Join(lines, "\n")
}

// calculateWidth determines the minimum width needed
func (b *Box) calculateWidth() int {
	maxWidth := utf8.RuneCountInString(b.Title) + 4 // Title + padding
	
	for _, line := range b.Content {
		lineWidth := utf8.RuneCountInString(line) + (b.Padding * 2) + 2
		if lineWidth > maxWidth {
			maxWidth = lineWidth
		}
	}
	
	// Minimum width
	if maxWidth < 20 {
		maxWidth = 20
	}
	
	return maxWidth
}

// renderTopBorder renders the top border
func (b *Box) renderTopBorder() string {
	border := b.Style.TopLeft
	border += strings.Repeat(b.Style.Horizontal, b.Width-2)
	border += b.Style.TopRight
	return border
}

// renderBottomBorder renders the bottom border
func (b *Box) renderBottomBorder() string {
	border := b.Style.BottomLeft
	border += strings.Repeat(b.Style.Horizontal, b.Width-2)
	border += b.Style.BottomRight
	return border
}

// renderDivider renders a horizontal divider
func (b *Box) renderDivider() string {
	divider := b.Style.MiddleLeft
	divider += strings.Repeat(b.Style.Middle, b.Width-2)
	divider += b.Style.MiddleRight
	return divider
}

// renderTitleLine renders the title line
func (b *Box) renderTitleLine() string {
	titleWidth := utf8.RuneCountInString(b.Title)
	totalPadding := b.Width - titleWidth - 2 // 2 for borders
	leftPadding := totalPadding / 2
	rightPadding := totalPadding - leftPadding
	
	line := b.Style.Vertical
	line += strings.Repeat(" ", leftPadding)
	line += b.Title
	line += strings.Repeat(" ", rightPadding)
	line += b.Style.Vertical
	
	return line
}

// renderContentLine renders a content line with padding
func (b *Box) renderContentLine(content string) string {
	contentWidth := utf8.RuneCountInString(content)
	totalPadding := b.Width - contentWidth - 2 // 2 for borders
	
	if totalPadding < 0 {
		// Content is too wide, truncate
		runes := []rune(content)
		maxRunes := b.Width - 5 // Leave room for "..." and borders
		if maxRunes > 0 && len(runes) > maxRunes {
			content = string(runes[:maxRunes]) + "..."
		}
		contentWidth = utf8.RuneCountInString(content)
		totalPadding = b.Width - contentWidth - 2
	}
	
	leftPadding := b.Padding
	rightPadding := totalPadding - leftPadding
	if rightPadding < b.Padding {
		rightPadding = b.Padding
	}
	
	line := b.Style.Vertical
	line += strings.Repeat(" ", leftPadding)
	line += content
	line += strings.Repeat(" ", rightPadding)
	line += b.Style.Vertical
	
	return line
}

// RenderHaikuBox creates a specialized box for haiku display
func RenderHaikuBox(japanese []string, english []string) string {
	return RenderHaikuBoxWithID(japanese, english, "")
}

// RenderHaikuBoxWithID creates a specialized box for haiku display with optional ID
func RenderHaikuBoxWithID(japanese []string, english []string, id string) string {
	// Interleave Japanese and English lines
	var content []string
	maxLines := len(japanese)
	if len(english) > maxLines {
		maxLines = len(english)
	}
	
	for i := 0; i < maxLines; i++ {
		if i < len(japanese) {
			content = append(content, japanese[i])
		}
		if i < len(english) {
			content = append(content, english[i])
		}
		if i < maxLines-1 {
			content = append(content, "") // Empty line between verses
		}
	}
	
	title := "🎋 Generated Haiku"
	if id != "" {
		title = fmt.Sprintf("🎋 Generated Haiku [ID: %s]", id)
	}
	
	box := NewBox(title, content)
	return box.Render()
}

// RenderToolResultBox creates a box for generic tool results
func RenderToolResultBox(toolName string, result interface{}) string {
	title := fmt.Sprintf("🔧 %s Result", toolName)
	
	// Convert result to string lines
	var content []string
	switch v := result.(type) {
	case string:
		// Split by newlines if present
		content = strings.Split(v, "\n")
	case map[string]interface{}:
		// Format as key-value pairs
		for key, value := range v {
			content = append(content, fmt.Sprintf("%s: %v", key, value))
		}
	case []interface{}:
		// Format as list
		for _, item := range v {
			content = append(content, fmt.Sprintf("• %v", item))
		}
	default:
		content = []string{fmt.Sprintf("%v", result)}
	}
	
	box := NewBox(title, content)
	return box.Render()
}

// RenderErrorBox creates a box for error messages
func RenderErrorBox(toolName string, errorMsg string) string {
	title := fmt.Sprintf("❌ %s Error", toolName)
	content := strings.Split(errorMsg, "\n")
	
	box := NewBox(title, content)
	return box.Render()
}