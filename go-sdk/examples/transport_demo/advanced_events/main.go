package main

import (
	"fmt"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/transport"
)

func main() {
	fmt.Println("🚀 Testing Advanced Type-Safe Events")
	
	// Test Message Event
	messageData := &transport.MessageEventData{
		Content:     "Hello, this is a type-safe message!",
		Role:        "user",
		Model:       "gpt-4",
		MessageID:   "msg-123",
		ThreadID:    "thread-456",
		Temperature: 0.7,
		MaxTokens:   150,
		TokenUsage: &transport.TokenUsage{
			PromptTokens:     50,
			CompletionTokens: 100,
			TotalTokens:      150,
		},
		Metadata: map[string]string{
			"source": "api",
			"version": "1.0",
		},
		Attachments: []transport.MessageAttachment{
			{
				Type:        "image",
				URL:         "https://example.com/image.png",
				ContentType: "image/png",
				Size:        1024,
				Name:        "example.png",
			},
		},
		ProcessingTime: 250 * time.Millisecond,
	}
	
	if err := messageData.Validate(); err != nil {
		fmt.Printf("❌ Message validation failed: %v\n", err)
		return
	}
	
	messageEvent := transport.CreateMessageEvent("evt-msg-001", messageData)
	fmt.Printf("✅ Message Event: ID=%s, Type=%s\n", messageEvent.ID(), messageEvent.Type())
	
	// Test Security Event  
	securityData := &transport.SecurityEventData{
		EventType:   transport.SecurityEventLogin,
		Severity:    transport.SecuritySeverityInfo,
		Actor:       "user-123",
		Target:      "api-endpoint",
		Resource:    "/api/v1/messages",
		UserID:      "user-123",
		SessionID:   "sess-789",
		SourceIP:    "192.168.1.100",
		UserAgent:   "Mozilla/5.0...",
		ThreatLevel: transport.ThreatLevelNone,
		Blocked:     false,
		Automatic:   true,
		Permissions: []string{"read", "write"},
		Context: map[string]string{
			"method": "POST",
			"endpoint": "/api/v1/messages",
		},
	}
	
	if err := securityData.Validate(); err != nil {
		fmt.Printf("❌ Security validation failed: %v\n", err)
		return
	}
	
	securityEvent := transport.CreateSecurityEvent("evt-sec-001", securityData)
	fmt.Printf("✅ Security Event: ID=%s, Type=%s\n", securityEvent.ID(), securityEvent.Type())
	
	// Test Performance Event
	perfData := &transport.PerformanceEventData{
		MetricName:    "api_response_time",
		Value:         125.5,
		Unit:          "milliseconds",
		MetricType:    transport.MetricTypeTimer,
		Component:     "api-server",
		Operation:     "POST /api/v1/messages",
		Threshold:     200.0,
		Baseline:      100.0,
		Percentile:    95.0,
		SampleSize:    1000,
		Duration:      time.Hour,
		Trend:         transport.TrendStable,
		AlertLevel:    transport.AlertLevelNone,
		Tags: map[string]string{
			"service": "message-api",
			"version": "1.2.3",
		},
		Dimensions: map[string]float64{
			"cpu_usage":    45.2,
			"memory_usage": 67.8,
		},
	}
	
	if err := perfData.Validate(); err != nil {
		fmt.Printf("❌ Performance validation failed: %v\n", err)
		return
	}
	
	perfEvent := transport.CreatePerformanceEvent("evt-perf-001", perfData)
	fmt.Printf("✅ Performance Event: ID=%s, Type=%s\n", perfEvent.ID(), perfEvent.Type())
	
	// Test System Event
	systemData := &transport.SystemEventData{
		EventType:     transport.SystemEventStartup,
		Component:     "message-service",
		Instance:      "msg-svc-01",
		Version:       "1.2.3",
		PreviousState: "stopped",
		CurrentState:  "running",
		TargetState:   "running",
		Resources: transport.ResourceUsage{
			CPU:       2.5,
			Memory:    512,
			Disk:      1024,
			Network:   100,
			Instances: 3,
		},
		Dependencies: []string{"database", "cache", "auth-service"},
		Initiated:    true,
		Automated:    false,
		Reason:       "manual startup",
		Context: map[string]string{
			"operator": "admin",
			"reason":   "deployment",
		},
	}
	
	if err := systemData.Validate(); err != nil {
		fmt.Printf("❌ System validation failed: %v\n", err)
		return
	}
	
	systemEvent := transport.CreateSystemEvent("evt-sys-001", systemData)
	fmt.Printf("✅ System Event: ID=%s, Type=%s\n", systemEvent.ID(), systemEvent.Type())
	
	// Test Configuration Event
	configData := &transport.ConfigurationEventData{
		Key:         "api.timeout",
		OldValue:    "30s",
		NewValue:    "45s",
		ValueType:   "duration",
		ChangedBy:   "admin",
		Reason:      "performance optimization",
		Source:      "web-console",
		Namespace:   "api-config",
		Validated:   true,
		Applied:     true,
		CanRollback: true,
		Impact:      transport.ConfigImpactLow,
	}
	
	if err := configData.Validate(); err != nil {
		fmt.Printf("❌ Configuration validation failed: %v\n", err)
		return
	}
	
	configEvent := transport.CreateConfigurationEvent("evt-cfg-001", configData)
	fmt.Printf("✅ Configuration Event: ID=%s, Type=%s\n", configEvent.ID(), configEvent.Type())
	
	// Test State Change Event
	stateData := &transport.StateChangeEventData{
		FromState:  "idle",
		ToState:    "processing",
		Reason:     "new message received",
		Trigger:    "api_request",
		EntityID:   "worker-123",
		EntityType: "message_processor",
		Context: map[string]string{
			"message_id": "msg-123",
			"priority":   "high",
		},
		Duration:  150 * time.Millisecond,
		Automatic: true,
		Rollback:  false,
		Version:   "1.0",
	}
	
	if err := stateData.Validate(); err != nil {
		fmt.Printf("❌ State change validation failed: %v\n", err)
		return
	}
	
	stateEvent := transport.CreateStateChangeEvent("evt-state-001", stateData)
	fmt.Printf("✅ State Change Event: ID=%s, Type=%s\n", stateEvent.ID(), stateEvent.Type())
	
	fmt.Println("\n🎉 All advanced type-safe events created and validated successfully!")
	fmt.Println("✨ Type safety migration is complete with comprehensive event support!")
}