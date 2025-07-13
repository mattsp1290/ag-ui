package encoding_test

import (
	"context"
	"testing"

	"github.com/ag-ui/go-sdk/pkg/encoding"
	"github.com/ag-ui/go-sdk/pkg/encoding/json"
	"github.com/ag-ui/go-sdk/pkg/encoding/protobuf"
)

// TestExplicitRegistration demonstrates explicit registration
func TestExplicitRegistration(t *testing.T) {
	t.Run("JSON registration", func(t *testing.T) {
		// Ensure JSON codec is registered
		if err := json.EnsureRegistered(); err != nil {
			t.Fatalf("Failed to register JSON codec: %v", err)
		}

		// Verify it's registered
		registry := encoding.GetGlobalRegistry()
		if !registry.SupportsFormat("application/json") {
			t.Error("JSON format not registered")
		}
		if !registry.SupportsEncoding("application/json") {
			t.Error("JSON encoding not supported")
		}
		if !registry.SupportsDecoding("application/json") {
			t.Error("JSON decoding not supported")
		}
	})

	t.Run("Protobuf registration", func(t *testing.T) {
		// Ensure Protobuf codec is registered
		if err := protobuf.EnsureRegistered(); err != nil {
			t.Fatalf("Failed to register Protobuf codec: %v", err)
		}

		// Verify it's registered
		registry := encoding.GetGlobalRegistry()
		if !registry.SupportsFormat("application/x-protobuf") {
			t.Error("Protobuf format not registered")
		}
		if !registry.SupportsEncoding("application/x-protobuf") {
			t.Error("Protobuf encoding not supported")
		}
		if !registry.SupportsDecoding("application/x-protobuf") {
			t.Error("Protobuf decoding not supported")
		}
	})
}

// TestCustomRegistry demonstrates using a custom registry
func TestCustomRegistry(t *testing.T) {
	t.Run("JSON with custom registry", func(t *testing.T) {
		// Create a custom registry
		customRegistry := encoding.NewFormatRegistry()

		// Register JSON to the custom registry
		if err := json.RegisterTo(customRegistry); err != nil {
			t.Fatalf("Failed to register JSON to custom registry: %v", err)
		}

		// Verify it's registered
		if !customRegistry.SupportsFormat("application/json") {
			t.Error("JSON format not registered in custom registry")
		}

		// Create an encoder from the custom registry  
		encoder, err := customRegistry.GetEncoder(context.Background(), "application/json", nil)
		if err != nil {
			t.Fatalf("Failed to get JSON encoder from custom registry: %v", err)
		}
		if encoder == nil {
			t.Error("Got nil encoder from custom registry")
		}
		
		// Create a decoder from the custom registry
		decoder, err := customRegistry.GetDecoder(context.Background(), "application/json", nil)
		if err != nil {
			t.Fatalf("Failed to get JSON decoder from custom registry: %v", err)
		}
		if decoder == nil {
			t.Error("Got nil decoder from custom registry")
		}
	})

	t.Run("Protobuf with custom registry", func(t *testing.T) {
		// Create a custom registry
		customRegistry := encoding.NewFormatRegistry()

		// Register Protobuf to the custom registry
		if err := protobuf.RegisterTo(customRegistry); err != nil {
			t.Fatalf("Failed to register Protobuf to custom registry: %v", err)
		}

		// Verify it's registered
		if !customRegistry.SupportsFormat("application/x-protobuf") {
			t.Error("Protobuf format not registered in custom registry")
		}

		// Create an encoder from the custom registry
		encoder, err := customRegistry.GetEncoder(context.Background(), "application/x-protobuf", nil)
		if err != nil {
			t.Fatalf("Failed to get Protobuf encoder from custom registry: %v", err)
		}
		if encoder == nil {
			t.Error("Got nil encoder from custom registry")
		}
		
		// Create a decoder from the custom registry
		decoder, err := customRegistry.GetDecoder(context.Background(), "application/x-protobuf", nil)
		if err != nil {
			t.Fatalf("Failed to get Protobuf decoder from custom registry: %v", err)
		}
		if decoder == nil {
			t.Error("Got nil decoder from custom registry")
		}
	})
}

// TestIdempotentRegistration verifies that multiple registrations don't cause issues
func TestIdempotentRegistration(t *testing.T) {
	t.Run("JSON multiple registrations", func(t *testing.T) {
		// Register multiple times - should be idempotent
		for i := 0; i < 3; i++ {
			err := json.Register()
			if err != nil {
				t.Fatalf("Registration %d failed: %v", i+1, err)
			}
		}

		// Verify it's still registered correctly
		registry := encoding.GetGlobalRegistry()
		if !registry.SupportsFormat("application/json") {
			t.Error("JSON format not registered after multiple attempts")
		}
	})

	t.Run("Protobuf multiple registrations", func(t *testing.T) {
		// Register multiple times - should be idempotent
		for i := 0; i < 3; i++ {
			err := protobuf.Register()
			if err != nil {
				t.Fatalf("Registration %d failed: %v", i+1, err)
			}
		}

		// Verify it's still registered correctly
		registry := encoding.GetGlobalRegistry()
		if !registry.SupportsFormat("application/x-protobuf") {
			t.Error("Protobuf format not registered after multiple attempts")
		}
	})
}

// TestRegistrationError demonstrates handling registration errors
func TestRegistrationError(t *testing.T) {
	t.Run("nil registry error", func(t *testing.T) {
		// Attempt to register to nil registry
		err := json.RegisterTo(nil)
		if err == nil {
			t.Error("Expected error when registering to nil registry")
		}

		err = protobuf.RegisterTo(nil)
		if err == nil {
			t.Error("Expected error when registering to nil registry")
		}
	})
}