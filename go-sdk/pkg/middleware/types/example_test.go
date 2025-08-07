package types_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/types"
)

// ExampleNewRequest demonstrates how to create and use a new Request
func ExampleNewRequest() {
	// Create a new request
	req := types.NewRequest("req-123", "POST", "/api/users")

	// Add headers
	req.SetHeader("Content-Type", "application/json")
	req.SetHeader("Authorization", "Bearer token123")

	// Add metadata
	req.SetMetadata(types.RequestMetadataKeys.UserID, "user-456")
	req.SetMetadata(types.RequestMetadataKeys.TraceID, "trace-789")

	// Set request body
	req.Body = map[string]interface{}{
		"name":  "John Doe",
		"email": "john@example.com",
	}

	fmt.Printf("Request ID: %s\n", req.ID)
	fmt.Printf("Method: %s\n", req.Method)
	fmt.Printf("Path: %s\n", req.Path)

	// Output:
	// Request ID: req-123
	// Method: POST
	// Path: /api/users
}

// ExampleNewResponse demonstrates how to create and use a new Response
func ExampleNewResponse() {
	// Create a new response
	resp := types.NewResponse("req-123", 200)

	// Add headers
	resp.SetHeader("Content-Type", "application/json")
	resp.SetHeader("Cache-Control", "no-cache")

	// Add metadata
	resp.SetMetadata(types.ResponseMetadataKeys.ProcessedBy, "auth-middleware")
	resp.SetMetadata(types.ResponseMetadataKeys.ProcessingTime, time.Millisecond*150)

	// Set response body
	resp.Body = map[string]interface{}{
		"status":  "success",
		"message": "User created successfully",
	}

	// Set processing duration
	resp.Duration = time.Millisecond * 150

	fmt.Printf("Response ID: %s\n", resp.ID)
	fmt.Printf("Status Code: %d\n", resp.StatusCode)
	fmt.Printf("Is Successful: %t\n", resp.IsSuccessful())

	// Output:
	// Response ID: req-123
	// Status Code: 200
	// Is Successful: true
}

// ExampleNextHandler demonstrates how to implement middleware using shared types
func ExampleNextHandler() {
	// Define a simple middleware that adds processing metadata
	middleware := func(ctx context.Context, req *types.Request, next types.NextHandler) (*types.Response, error) {
		// Record start time
		start := time.Now()

		// Add processing metadata to request
		req.SetMetadata("processed_by", "example-middleware")
		req.SetMetadata("start_time", start)

		// Call next handler
		resp, err := next(ctx, req)
		if err != nil {
			return resp, err
		}

		// Add processing metadata to response
		if resp != nil {
			resp.SetMetadata(types.ResponseMetadataKeys.ProcessedBy, "example-middleware")
			resp.SetMetadata(types.ResponseMetadataKeys.ProcessingTime, time.Since(start))
		}

		return resp, nil
	}

	// Define a simple handler
	handler := func(ctx context.Context, req *types.Request) (*types.Response, error) {
		resp := types.NewResponse(req.ID, 200)
		resp.Body = map[string]string{"message": "Hello World"}
		return resp, nil
	}

	// Create a request
	req := types.NewRequest("test-123", "GET", "/api/hello")

	// Execute middleware chain
	resp, err := middleware(context.Background(), req, handler)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Response Status: %d\n", resp.StatusCode)

	// Check if middleware processed the response
	if processedBy, exists := resp.GetMetadata(types.ResponseMetadataKeys.ProcessedBy); exists {
		fmt.Printf("Processed by: %s\n", processedBy)
	}

	// Output:
	// Response Status: 200
	// Processed by: example-middleware
}

// ExampleRequest_Clone demonstrates request cloning for immutable operations
func ExampleRequest_Clone() {
	// Create original request
	original := types.NewRequest("req-123", "POST", "/api/users")
	original.SetHeader("Authorization", "Bearer token123")
	original.SetMetadata(types.RequestMetadataKeys.UserID, "user-456")

	// Clone the request
	clone := original.Clone()

	// Modify clone without affecting original
	clone.SetHeader("Content-Type", "application/json")
	clone.SetMetadata(types.RequestMetadataKeys.TraceID, "trace-789")

	// Original remains unchanged
	_, hasContentType := original.GetHeader("Content-Type")
	fmt.Printf("Original has Content-Type: %t\n", hasContentType)

	// Clone has the new header
	cloneContentType, hasCloneContentType := clone.GetHeader("Content-Type")
	fmt.Printf("Clone has Content-Type: %t (%s)\n", hasCloneContentType, cloneContentType)

	// Output:
	// Original has Content-Type: false
	// Clone has Content-Type: true (application/json)
}

// ExampleResponse_IsSuccessful demonstrates response status helper methods
func ExampleResponse_IsSuccessful() {
	responses := []*types.Response{
		types.NewResponse("req-1", 200), // Success
		types.NewResponse("req-2", 400), // Client Error
		types.NewResponse("req-3", 500), // Server Error
	}

	// Add an error to the last response
	responses[2].Error = fmt.Errorf("internal server error")

	for i, resp := range responses {
		fmt.Printf("Response %d (Status %d):\n", i+1, resp.StatusCode)
		fmt.Printf("  Is Successful: %t\n", resp.IsSuccessful())
		fmt.Printf("  Is Client Error: %t\n", resp.IsClientError())
		fmt.Printf("  Is Server Error: %t\n", resp.IsServerError())
		fmt.Printf("  Has Error: %t\n", resp.HasError())
		fmt.Println()
	}

	// Output:
	// Response 1 (Status 200):
	//   Is Successful: true
	//   Is Client Error: false
	//   Is Server Error: false
	//   Has Error: false
	//
	// Response 2 (Status 400):
	//   Is Successful: false
	//   Is Client Error: true
	//   Is Server Error: false
	//   Has Error: false
	//
	// Response 3 (Status 500):
	//   Is Successful: false
	//   Is Client Error: false
	//   Is Server Error: true
	//   Has Error: true
}
