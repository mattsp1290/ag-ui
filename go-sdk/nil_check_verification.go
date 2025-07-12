package main

import (
	"context"
	"fmt"
	"github.com/ag-ui/go-sdk/pkg/encoding"
	"github.com/ag-ui/go-sdk/pkg/transport"
)

func main() {
	fmt.Println("Testing nil checks...")
	
	// Test codec factory nil checks
	var factory *encoding.DefaultCodecFactory
	_, err := factory.CreateCodec(context.Background(), "application/json", nil, nil)
	if err != nil {
		fmt.Printf("✓ DefaultCodecFactory.CreateCodec correctly handles nil factory: %s\n", err.Error())
	} else {
		fmt.Println("✗ DefaultCodecFactory.CreateCodec should handle nil factory")
	}
	
	// Test caching factory nil checks
	cachingFactory := encoding.NewCachingCodecFactory(nil)
	if cachingFactory == nil {
		fmt.Println("✓ NewCachingCodecFactory correctly returns nil for nil underlying factory")
	} else {
		fmt.Println("✗ NewCachingCodecFactory should return nil for nil underlying factory")
	}
	
	// Test transport registry nil checks
	var registry *transport.DefaultTransportRegistry
	err = registry.Register("test", nil)
	if err != nil {
		fmt.Printf("✓ DefaultTransportRegistry.Register correctly handles nil registry: %s\n", err.Error())
	} else {
		fmt.Println("✗ DefaultTransportRegistry.Register should handle nil registry")
	}
	
	// Test transport manager nil checks
	manager := transport.NewDefaultTransportManager(nil)
	if manager == nil {
		fmt.Println("✓ NewDefaultTransportManager correctly returns nil for nil registry")
	} else {
		fmt.Println("✗ NewDefaultTransportManager should return nil for nil registry")
	}
	
	fmt.Println("Nil check testing completed!")
}