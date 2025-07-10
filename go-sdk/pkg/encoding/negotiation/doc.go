// Package negotiation implements RFC 7231 compliant HTTP content negotiation for the AG-UI SDK.
//
// This package provides intelligent selection of content types based on client preferences
// (Accept headers), server capabilities, and real-time performance characteristics.
//
// # Features
//
//   - RFC 7231 compliant Accept header parsing
//   - Quality factor (q-value) support
//   - Wildcard matching (*/*, application/*)
//   - Performance-based adaptive selection
//   - Client capability detection
//   - Thread-safe operations
//
// # Basic Usage
//
//	negotiator := negotiation.NewContentNegotiator("application/json")
//	contentType, err := negotiator.Negotiate("application/json;q=0.9, application/x-protobuf;q=1.0")
//	// Returns: "application/x-protobuf"
//
// # Accept Header Format
//
// The package supports standard HTTP Accept headers as defined in RFC 7231:
//
//	Accept: type/subtype[;parameter=value][;q=0.X], ...
//
// Examples:
//   - application/json
//   - application/x-protobuf, application/json;q=0.8
//   - */*, application/json;q=0.9
//   - application/vnd.ag-ui+json;charset=utf-8;q=0.95
//
// # Performance-Based Selection
//
// The negotiator can track performance metrics and use them for intelligent selection:
//
//	negotiator.UpdatePerformance("application/json", PerformanceMetrics{
//	    EncodingTime: 10 * time.Millisecond,
//	    DecodingTime: 8 * time.Millisecond,
//	    SuccessRate:  0.95,
//	})
//
// When quality factors are equal, the negotiator will prefer formats with better
// performance characteristics.
//
// # Advanced Selection
//
// The FormatSelector provides fine-grained control over selection criteria:
//
//	selector := negotiation.NewFormatSelector(negotiator)
//	criteria := &SelectionCriteria{
//	    RequireStreaming: true,
//	    MinQuality:       0.5,
//	    ClientCapabilities: &ClientCapabilities{
//	        SupportsStreaming: true,
//	        CompressionSupport: []string{"gzip"},
//	    },
//	}
//	contentType, err := selector.SelectFormat(acceptHeader, criteria)
//
// # Adaptive Selection
//
// The AdaptiveSelector learns from request history to make better decisions:
//
//	adaptive := negotiation.NewAdaptiveSelector(negotiator)
//	adaptive.UpdateHistory("application/json", success, latency)
//	contentType, err := adaptive.SelectAdaptive(acceptHeader, criteria)
//
// # Custom Types
//
// Register custom content types with specific capabilities:
//
//	negotiator.RegisterType(&TypeCapabilities{
//	    ContentType:        "application/vnd.myapp+json",
//	    CanStream:          true,
//	    CompressionSupport: []string{"gzip", "br"},
//	    Priority:           0.85,
//	    Aliases:            []string{"application/x-myapp-json"},
//	})
//
// # Thread Safety
//
// All types in this package are safe for concurrent use. The negotiator uses
// read-write locks to ensure thread-safe access to its internal state.
package negotiation