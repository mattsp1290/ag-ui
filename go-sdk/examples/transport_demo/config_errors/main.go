package main

import (
	"fmt"
	"log"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/transport"
)

func main() {
	fmt.Println("=== Type-Safe Configuration Error Examples ===")

	// Example 1: Type-safe string configuration error
	fmt.Println("\n1. String Configuration Error:")
	stringErr := transport.NewStringConfigError("hostname", "invalid_host", "hostname cannot contain underscores")
	fmt.Printf("   Error: %v\n", stringErr)
	fmt.Printf("   Field: %s\n", stringErr.Field)
	fmt.Printf("   Value: %s\n", stringErr.Value.Value)

	// Example 2: Type-safe integer configuration error
	fmt.Println("\n2. Integer Configuration Error:")
	intErr := transport.NewIntConfigError("port", -80, "port must be positive")
	fmt.Printf("   Error: %v\n", intErr)
	fmt.Printf("   Field: %s\n", intErr.Field)
	fmt.Printf("   Value: %d\n", intErr.Value.Value)

	// Example 3: Type-safe boolean configuration error
	fmt.Println("\n3. Boolean Configuration Error:")
	boolErr := transport.NewBoolConfigError("ssl_enabled", false, "SSL must be enabled in production")
	fmt.Printf("   Error: %v\n", boolErr)
	fmt.Printf("   Field: %s\n", boolErr.Field)
	fmt.Printf("   Value: %t\n", boolErr.Value.Value)

	// Example 4: Nil configuration error
	fmt.Println("\n4. Nil Configuration Error:")
	nilErr := transport.NewNilConfigError("api_key", "API key is required but was not provided")
	fmt.Printf("   Error: %v\n", nilErr)
	fmt.Printf("   Field: %s\n", nilErr.Field)

	// Example 5: Generic configuration error (for complex types)
	fmt.Println("\n5. Generic Configuration Error:")
	complexConfig := map[string]interface{}{
		"timeout": "invalid",
		"retries": -1,
	}
	genericErr := transport.NewGenericConfigError("connection_config", complexConfig, "invalid connection configuration")
	fmt.Printf("   Error: %v\n", genericErr)
	fmt.Printf("   Field: %s\n", genericErr.Field)

	// Example 6: Backward compatibility with legacy errors
	fmt.Println("\n6. Legacy Configuration Error (backward compatibility):")
	legacyErr := transport.NewLegacyConfigurationError("buffer_size", 0, "buffer size cannot be zero")
	fmt.Printf("   Error: %v\n", legacyErr)
	fmt.Printf("   Field: %s\n", legacyErr.Field)
	fmt.Printf("   Value: %v\n", legacyErr.Value)

	// Example 7: Using helper functions to work with any configuration error type
	fmt.Println("\n7. Using Helper Functions:")
	errors := []error{stringErr, intErr, boolErr, legacyErr}

	for i, err := range errors {
		if transport.IsConfigurationError(err) {
			field := transport.GetConfigurationErrorField(err)
			value := transport.GetConfigurationErrorValue(err)
			fmt.Printf("   Error %d - Field: %s, Value: %v\n", i+1, field, value)
		}
	}

	// Example 8: Type validation and automatic conversion
	fmt.Println("\n8. Type Validation and Conversion:")
	values := []interface{}{
		"test_string",
		42,
		true,
		3.14,
		nil,
		[]string{"complex", "type"},
	}

	for _, val := range values {
		errorValue := transport.ValidateErrorValue(val)
		fmt.Printf("   Input: %v (%T) -> ErrorString: %s\n", val, val, errorValue.ErrorString())
	}

	// Example 9: Creating typed errors from interface{} values
	fmt.Println("\n9. Creating Typed Errors from Interface{} Values:")
	interfaceValue := interface{}(8080)
	typedErr := transport.CreateTypedConfigError("port", interfaceValue, "port already in use")
	fmt.Printf("   Error: %v\n", typedErr)
	fmt.Printf("   Is Configuration Error: %t\n", transport.IsConfigurationError(typedErr))

	log.Println("Type-safe configuration error system demonstration complete!")
}
