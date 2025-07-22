// Package registry provides focused components for the encoding format registry.
//
// This package contains the modular components that were extracted from the
// large registry.go file to improve maintainability and separation of concerns:
//
//   - Cache Management: LRU caching and memory management
//   - Priority Management: Format priority handling and selection
//   - Lifecycle Management: Cleanup and resource management
//   - Metrics: Statistics and monitoring
//   - Types: Common types and interfaces
//
// All components work together to provide a comprehensive format registry
// with memory management, caching, and performance optimization capabilities.
package registry