package distributed_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events/distributed"
)

// ExampleDistributedValidator demonstrates basic usage of the distributed validator
func ExampleDistributedValidator() {
	// Create a local validator first
	localValidator := events.NewEventValidator(nil)

	// Configure the distributed validator for testing (no goroutine restarts)
	config := distributed.TestingDistributedValidatorConfig("node-1")
	config.ConsensusConfig.Algorithm = distributed.ConsensusMajority

	// Create the distributed validator
	dv, err := distributed.NewDistributedValidator(config, localValidator)
	if err != nil {
		log.Fatal(err)
	}

	// Start the validator
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = dv.Start(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		// Suppress cleanup output for cleaner example
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		dv.Stop()
		w.Close()
		r.Close()
		os.Stdout = oldStdout
	}()

	// Register additional nodes
	node2 := &distributed.NodeInfo{
		ID:              "node-2",
		Address:         "node2:8080",
		State:           distributed.NodeStateActive,
		LastHeartbeat:   time.Now(),
		ValidationCount: 0,
		ErrorRate:       0.0,
		ResponseTimeMs:  50,
		Load:            0.3,
	}

	err = dv.RegisterNode(node2)
	if err != nil {
		log.Printf("Failed to register node: %v", err)
	}

	// Create a test event
	event := &events.RunStartedEvent{
		BaseEvent: &events.BaseEvent{
			EventType:   events.EventTypeRunStarted,
			TimestampMs: timePtr(time.Now().UnixMilli()),
		},
		RunIDValue:    "test-run-1",
		ThreadIDValue: "test-thread-1",
	}

	// Validate the event using distributed consensus with timeout
	validationCtx, validationCancel := context.WithTimeout(ctx, 2*time.Second)
	defer validationCancel()
	result := dv.ValidateEvent(validationCtx, event)

	if result.IsValid {
		fmt.Println("Event validation successful")
	} else {
		fmt.Printf("Event validation failed with %d errors\n", len(result.Errors))
	}

	// Get metrics
	metrics := dv.GetMetrics()
	fmt.Printf("Validation count: %d\n", metrics.GetValidationCount())

	// Output:
	// Event validation successful
	// Validation count: 1
}

// ExampleConsensusAlgorithms demonstrates different consensus algorithms
func ExampleDistributedValidator_consensus() {
	algorithms := []distributed.ConsensusAlgorithm{
		distributed.ConsensusMajority,
		distributed.ConsensusUnanimous,
	}

	for _, algo := range algorithms {
		config := &distributed.ConsensusConfig{
			Algorithm:  algo,
			MinNodes:   3,
			QuorumSize: 2,
		}

		cm, err := distributed.NewConsensusManager(config, "node-1")
		if err != nil {
			log.Fatal(err)
		}

		// Create test decisions
		decisions := []*distributed.ValidationDecision{
			{NodeID: "node-1", IsValid: true},
			{NodeID: "node-2", IsValid: true},
			{NodeID: "node-3", IsValid: false},
		}

		result := cm.AggregateDecisions(decisions)
		fmt.Printf("Algorithm %s: Valid=%t\n", algo, result.IsValid)
	}

	// Output:
	// Algorithm majority: Valid=true
	// Algorithm unanimous: Valid=false
}

// ExampleLoadBalancing demonstrates load balancing across nodes
func ExampleDistributedValidator_loadBalancing() {
	config := distributed.DefaultLoadBalancerConfig()
	config.Algorithm = distributed.LoadBalancingLeastResponseTime

	lb := distributed.NewLoadBalancer(config)

	// Add nodes with different response times
	nodes := []distributed.NodeID{"node-1", "node-2", "node-3"}
	for i, node := range nodes {
		responseTime := float64((i + 1) * 10) // 10ms, 20ms, 30ms
		lb.UpdateNodeMetrics(node, 0.5, responseTime)
	}

	// Select 2 nodes for validation
	selected := lb.SelectNodes(nodes, 2)
	fmt.Printf("Selected nodes: %v\n", selected)

	// The least response time algorithm should prefer faster nodes
	// Output will vary but should favor node-1 (fastest)
}

// ExamplePartitionHandling demonstrates partition detection and handling
func ExampleDistributedValidator_partitionHandling() {
	config := distributed.DefaultPartitionHandlerConfig()
	config.HeartbeatTimeout = 100 * time.Millisecond

	ph := distributed.NewPartitionHandler(config, "node-1")

	// Set up callbacks
	partitionDetected := make(chan *distributed.PartitionInfo, 1)
	ph.SetPartitionCallbacks(
		func(p *distributed.PartitionInfo) {
			partitionDetected <- p
		},
		nil,
	)

	ctx := context.Background()
	err := ph.Start(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer ph.Stop()

	// Register healthy nodes
	ph.UpdateNodeHealth("node-2", true, 10*time.Millisecond)
	ph.UpdateNodeHealth("node-3", true, 10*time.Millisecond)

	// Simulate node failures
	ph.HandleNodeFailure("node-2")
	ph.HandleNodeFailure("node-3")

	// Wait for partition detection
	select {
	case partition := <-partitionDetected:
		fmt.Printf("Partition detected: Type=%s, Severity=%s\n",
			partition.Type, partition.Severity)
	case <-time.After(1 * time.Second):
		fmt.Println("No partition detected within timeout")
	}
}

// Helper function for creating time pointers
func timePtr(t int64) *int64 {
	return &t
}
