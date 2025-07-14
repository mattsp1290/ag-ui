package events

// This file serves as the main entry point for the debug functionality
// which has been split into focused modules in the debug/ subdirectory:
//
// - debug/debug.go         - Core debugging functionality
// - debug/session.go       - Session management functionality
// - debug/profiling.go     - Performance profiling functionality
// - debug/export.go        - Export functionality
// - debug/interactive.go   - Interactive debugging functionality
// - debug/patterns.go      - Error pattern detection functionality
//
// All debug functionality is now organized in focused modules while
// maintaining the same public API.
