package encoding_test

import (
	"fmt"
	"log"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/json"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/protobuf"
)

// Example demonstrating the new registration API
func Example_registration() {
	// Method 1: Import with side effects (backward compatible)
	// The init() functions will attempt registration automatically
	// import _ "github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/json"

	// Method 2: Explicit registration with error handling
	if err := json.EnsureRegistered(); err != nil {
		log.Fatalf("Failed to register JSON codec: %v", err)
	}

	if err := protobuf.EnsureRegistered(); err != nil {
		log.Fatalf("Failed to register Protobuf codec: %v", err)
	}

	// Now you can use the codecs
	registry := encoding.GetGlobalRegistry()
	formats := registry.ListFormats()

	fmt.Printf("Registered formats: %d\n", len(formats))
	// Output: Registered formats: 2
}

// Example using a custom registry for testing
func Example_customRegistry() {
	// Create an isolated registry for testing
	testRegistry := encoding.NewFormatRegistry()

	// Register only the codecs you need
	if err := json.RegisterTo(testRegistry); err != nil {
		log.Fatalf("Failed to register JSON: %v", err)
	}

	// Use the custom registry
	if testRegistry.SupportsFormat("application/json") {
		fmt.Println("JSON is supported")
	}

	// Output: JSON is supported
}

// Example demonstrating error handling
func Example_errorHandling() {
	// Attempting to register to nil registry returns an error
	err := json.RegisterTo(nil)
	if err != nil {
		fmt.Println("Got expected error:", err.Error())
	}

	// Output: Got expected error: [ERROR] JSON_NIL_REGISTRY: registry cannot be nil (operation: register)
}
