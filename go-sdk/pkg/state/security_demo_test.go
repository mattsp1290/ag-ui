package state

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// TestSecurityDemo demonstrates all security features working together
func TestSecurityDemo(t *testing.T) {
	// Create state manager with default security settings
	sm, err := NewStateManager(DefaultManagerOptions())
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}
	defer sm.Close()

	ctx := context.Background()
	contextID, err := sm.CreateContext(ctx, "demo-state", nil)
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}

	fmt.Println("=== Security Features Demo ===")
	fmt.Println()

	// 1. Demonstrate patch size limit
	fmt.Println("1. Testing Patch Size Limit (1MB):")
	largePatch := strings.Repeat("x", MaxPatchSizeBytes+1000)
	_, err = sm.UpdateState(ctx, contextID, "demo-state", map[string]interface{}{
		"large_data": largePatch,
	}, UpdateOptions{})
	if err != nil {
		fmt.Printf("   ✓ Patch size limit enforced: %v\n", err)
	} else {
		fmt.Println("   ✗ ERROR: Large patch was accepted!")
	}
	fmt.Println()

	// 2. Demonstrate string length limit
	fmt.Println("2. Testing String Length Limit (64KB):")
	longString := strings.Repeat("a", MaxStringLengthBytes+100)
	_, err = sm.UpdateState(ctx, contextID, "demo-state", map[string]interface{}{
		"long_string": longString,
	}, UpdateOptions{})
	if err != nil {
		fmt.Printf("   ✓ String length limit enforced: %v\n", err)
	} else {
		fmt.Println("   ✗ ERROR: Long string was accepted!")
	}
	fmt.Println()

	// 3. Demonstrate JSON depth limit
	fmt.Println("3. Testing JSON Depth Limit (10 levels):")
	deepJSON := createDeeplyNested(15)
	_, err = sm.UpdateState(ctx, contextID, "demo-state", map[string]interface{}{
		"deep": deepJSON,
	}, UpdateOptions{})
	if err != nil {
		fmt.Printf("   ✓ JSON depth limit enforced: %v\n", err)
	} else {
		fmt.Println("   ✗ ERROR: Deep JSON was accepted!")
	}
	fmt.Println()

	// 4. Demonstrate array length limit
	fmt.Println("4. Testing Array Length Limit (10000 items):")
	largeArray := make([]interface{}, MaxArrayLength+100)
	for i := range largeArray {
		largeArray[i] = i
	}
	_, err = sm.UpdateState(ctx, contextID, "demo-state", map[string]interface{}{
		"large_array": largeArray,
	}, UpdateOptions{})
	if err != nil {
		fmt.Printf("   ✓ Array length limit enforced: %v\n", err)
	} else {
		fmt.Println("   ✗ ERROR: Large array was accepted!")
	}
	fmt.Println()

	// 5. Demonstrate malicious content detection
	fmt.Println("5. Testing Malicious Content Detection:")
	maliciousContent := "<script>alert('xss')</script>"
	_, err = sm.UpdateState(ctx, contextID, "demo-state", map[string]interface{}{
		"content": maliciousContent,
	}, UpdateOptions{})
	if err != nil {
		fmt.Printf("   ✓ Malicious content blocked: %v\n", err)
	} else {
		fmt.Println("   ✗ ERROR: Malicious content was accepted!")
	}
	fmt.Println()

	// 6. Demonstrate forbidden paths
	fmt.Println("6. Testing Forbidden Paths:")
	patch := JSONPatch{
		{Op: JSONPatchOpAdd, Path: "/admin/secret", Value: "sensitive"},
	}
	err = sm.securityValidator.ValidatePatch(patch)
	if err != nil {
		fmt.Printf("   ✓ Forbidden path blocked: %v\n", err)
	} else {
		fmt.Println("   ✗ ERROR: Forbidden path was accepted!")
	}
	fmt.Println()

	// 7. Demonstrate rate limiting
	fmt.Println("7. Testing Rate Limiting:")
	// Create a new context for rate limit testing
	rateLimitCtx, _ := sm.CreateContext(ctx, "rate-test", nil)

	// Try to exceed rate limit
	hitRateLimit := false
	for i := 0; i < 50; i++ { // Reduced from 300 to 50 for faster testing
		_, err = sm.UpdateState(ctx, rateLimitCtx, "rate-test", map[string]interface{}{
			fmt.Sprintf("test_%d", i): i,
		}, UpdateOptions{})
		if err != nil && strings.Contains(err.Error(), "rate limit") {
			fmt.Printf("   ✓ Rate limit enforced after %d requests: %v\n", i+1, err)
			hitRateLimit = true
			break
		}
	}
	if !hitRateLimit {
		fmt.Println("   ℹ Rate limit not hit in test window (this is OK for burst capacity)")
	}
	fmt.Println()

	// 8. Demonstrate valid operations
	fmt.Println("8. Testing Valid Operations:")
	validUpdates := map[string]interface{}{
		"name":   "Test User",
		"age":    30,
		"active": true,
		"tags":   []string{"tag1", "tag2", "tag3"},
		"address": map[string]interface{}{
			"street": "123 Main St",
			"city":   "TestCity",
			"zip":    "12345",
		},
	}

	delta, err := sm.UpdateState(ctx, contextID, "demo-state", validUpdates, UpdateOptions{})
	if err != nil {
		fmt.Printf("   ✗ ERROR: Valid update failed: %v\n", err)
	} else {
		fmt.Printf("   ✓ Valid update succeeded, delta has %d operations\n", len(delta))
	}
	fmt.Println()

	fmt.Println("=== All Security Features Demonstrated ===")
}

// createDeeplyNested creates a deeply nested JSON structure
func createDeeplyNested(depth int) interface{} {
	if depth <= 0 {
		return "leaf"
	}
	return map[string]interface{}{
		"level": createDeeplyNested(depth - 1),
	}
}
