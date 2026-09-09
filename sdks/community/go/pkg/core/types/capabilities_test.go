package types

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func capabilityPtr[T any](v T) *T { return &v }

func TestAgentCapabilitiesAllFieldsAndWireKeys(t *testing.T) {
	falseValue, trueValue := false, true
	emptyDescription := ""
	metadata := map[string]any{"nested": map[string]any{"list": []any{"value", float64(2)}}}
	custom := map[string]any{"vendor": map[string]any{"enabled": true, "levels": []any{float64(1), "two"}}}
	tools := []Tool{{Name: "lookup", Description: "Looks up a value", Parameters: map[string]any{"type": "object"}}}
	mimeTypes := []string{"text/plain", "application/json"}
	subAgents := []SubAgentInfo{{Name: "researcher", Description: &emptyDescription}}
	zero := int64(0)

	caps := AgentCapabilities{
		Identity: &IdentityCapabilities{
			Name: capabilityPtr("Example"), Type: capabilityPtr("custom"), Description: capabilityPtr("Agent"),
			Version: capabilityPtr("1.0.0"), Provider: capabilityPtr("AG-UI"),
			DocumentationURL: capabilityPtr("https://example.test/docs"), Metadata: &metadata,
		},
		Transport:  &TransportCapabilities{Streaming: &trueValue, Websocket: &falseValue, HTTPBinary: &trueValue, PushNotifications: &falseValue, Resumable: &trueValue},
		Tools:      &ToolsCapabilities{Supported: &falseValue, Items: &tools, ParallelCalls: &trueValue, ClientProvided: &falseValue},
		Output:     &OutputCapabilities{StructuredOutput: &falseValue, SupportedMimeTypes: &mimeTypes},
		State:      &StateCapabilities{Snapshots: &trueValue, Deltas: &falseValue, Memory: &trueValue, PersistentState: &falseValue},
		MultiAgent: &MultiAgentCapabilities{Supported: &trueValue, Delegation: &falseValue, Handoffs: &trueValue, SubAgents: &subAgents},
		Reasoning:  &ReasoningCapabilities{Supported: &falseValue, Streaming: &trueValue, Encrypted: &falseValue},
		Multimodal: &MultimodalCapabilities{
			Input:  &MultimodalInputCapabilities{Image: &trueValue, Audio: &falseValue, Video: &trueValue, PDF: &falseValue, File: &trueValue},
			Output: &MultimodalOutputCapabilities{Image: &falseValue, Audio: &trueValue},
		},
		Execution:      &ExecutionCapabilities{CodeExecution: &trueValue, Sandboxed: &falseValue, MaxIterations: &zero, MaxExecutionTime: capabilityPtr(int64(5000))},
		HumanInTheLoop: &HumanInTheLoopCapabilities{Supported: &trueValue, Approvals: &falseValue, Interventions: &trueValue, Feedback: &falseValue, Interrupts: &trueValue, ApproveWithEdits: &falseValue},
		Custom:         &custom,
	}

	data, err := json.Marshal(caps)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	wantTop := []string{"identity", "transport", "tools", "output", "state", "multiAgent", "reasoning", "multimodal", "execution", "humanInTheLoop", "custom"}
	for _, key := range wantTop {
		if _, ok := wire[key]; !ok {
			t.Errorf("missing top-level wire key %q in %s", key, data)
		}
	}
	for _, key := range []string{"documentationUrl", "httpBinary", "pushNotifications", "parallelCalls", "clientProvided", "structuredOutput", "supportedMimeTypes", "persistentState", "subAgents", "codeExecution", "maxIterations", "maxExecutionTime", "approveWithEdits"} {
		if !strings.Contains(string(data), `"`+key+`"`) {
			t.Errorf("missing camelCase wire key %q in %s", key, data)
		}
	}

	var roundTrip AgentCapabilities
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(caps, roundTrip) {
		t.Errorf("round trip mismatch\nwant: %#v\n got: %#v", caps, roundTrip)
	}
}

func TestAgentCapabilitiesOptionalWireStates(t *testing.T) {
	data, err := json.Marshal(AgentCapabilities{})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{}" {
		t.Fatalf("empty declaration = %s, want {}", data)
	}

	emptyTools := []Tool{}
	emptyStrings := []string{}
	emptyAgents := []SubAgentInfo{}
	emptyMap := map[string]any{}
	falseValue := false
	zero := int64(0)
	caps := AgentCapabilities{
		Transport:  &TransportCapabilities{Streaming: &falseValue},
		Tools:      &ToolsCapabilities{Items: &emptyTools},
		Output:     &OutputCapabilities{SupportedMimeTypes: &emptyStrings},
		MultiAgent: &MultiAgentCapabilities{SubAgents: &emptyAgents},
		Execution:  &ExecutionCapabilities{MaxIterations: &zero},
		Custom:     &emptyMap,
	}
	data, err = json.Marshal(caps)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"transport":{"streaming":false},"tools":{"items":[]},"output":{"supportedMimeTypes":[]},"multiAgent":{"subAgents":[]},"execution":{"maxIterations":0},"custom":{}}`
	if string(data) != want {
		t.Fatalf("explicit zero values = %s, want %s", data, want)
	}
}

func TestAgentCapabilitiesPermissiveDecodeAndArbitraryJSON(t *testing.T) {
	payload := `{"identity":{"metadata":{"array":[1,{"deep":null}]}},"custom":{"flag":false,"object":{"n":3}},"futureCategory":{"enabled":true}}`
	var caps AgentCapabilities
	if err := json.Unmarshal([]byte(payload), &caps); err != nil {
		t.Fatal(err)
	}
	if caps.Identity == nil || caps.Identity.Metadata == nil || caps.Custom == nil {
		t.Fatal("arbitrary maps were not decoded")
	}
	if got := (*caps.Identity.Metadata)["array"].([]any)[1].(map[string]any)["deep"]; got != nil {
		t.Fatalf("nested value = %#v, want nil", got)
	}
	if got := (*caps.Custom)["flag"]; got != false {
		t.Fatalf("custom flag = %#v, want false", got)
	}

	data, err := json.Marshal(caps)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "futureCategory") {
		t.Fatalf("unknown field unexpectedly retained: %s", data)
	}
}

func TestExecutionCapabilitiesAcceptExactIntegralNumberSpellings(t *testing.T) {
	tests := []struct {
		payload string
		field   func(ExecutionCapabilities) *int64
		want    int64
	}{
		{`{"maxIterations":1.0}`, func(c ExecutionCapabilities) *int64 { return c.MaxIterations }, 1},
		{`{"maxIterations":1e3}`, func(c ExecutionCapabilities) *int64 { return c.MaxIterations }, 1000},
		{`{"maxIterations":1000e-3}`, func(c ExecutionCapabilities) *int64 { return c.MaxIterations }, 1},
		{`{"maxExecutionTime":0.000e999999999}`, func(c ExecutionCapabilities) *int64 { return c.MaxExecutionTime }, 0},
		{`{"maxExecutionTime":9223372036854775807.0}`, func(c ExecutionCapabilities) *int64 { return c.MaxExecutionTime }, 9223372036854775807},
		{`{"maxExecutionTime":-9223372036854775808e0}`, func(c ExecutionCapabilities) *int64 { return c.MaxExecutionTime }, -9223372036854775808},
	}
	for _, tt := range tests {
		var caps ExecutionCapabilities
		if err := json.Unmarshal([]byte(tt.payload), &caps); err != nil {
			t.Errorf("json.Unmarshal(%s): %v", tt.payload, err)
			continue
		}
		if got := tt.field(caps); got == nil || *got != tt.want {
			t.Errorf("json.Unmarshal(%s) = %v, want %d", tt.payload, got, tt.want)
		}
	}
}

func TestExecutionCapabilitiesRejectInvalidLimits(t *testing.T) {
	for _, payload := range []string{
		`{"maxIterations":1.5}`,
		`{"maxExecutionTime":2.25}`,
		`{"maxIterations":9007199254740991.0000000001}`,
		`{"maxIterations":9223372036854775808}`,
		`{"maxExecutionTime":-9223372036854775809.0}`,
		`{"maxIterations":1e999999999}`,
		`{"maxExecutionTime":1e-999999999}`,
		`{"maxIterations":"1"}`,
		`{"maxExecutionTime":true}`,
	} {
		var caps ExecutionCapabilities
		if err := json.Unmarshal([]byte(payload), &caps); err == nil {
			t.Errorf("json.Unmarshal(%s) succeeded, want error", payload)
		}
	}
}

func TestExecutionCapabilitiesCustomDecodeRemainsPermissive(t *testing.T) {
	var caps ExecutionCapabilities
	if err := json.Unmarshal([]byte(`{"codeExecution":false,"sandboxed":true,"maxIterations":null,"futureLimit":12}`), &caps); err != nil {
		t.Fatal(err)
	}
	if caps.CodeExecution == nil || *caps.CodeExecution || caps.Sandboxed == nil || !*caps.Sandboxed {
		t.Fatalf("boolean fields were not decoded: %#v", caps)
	}
	if caps.MaxIterations != nil {
		t.Fatalf("null maxIterations = %v, want nil", caps.MaxIterations)
	}
}

func TestExecutionCapabilitiesExponentBoundDependsOnMantissa(t *testing.T) {
	longInteger := "1" + strings.Repeat("0", 1000)
	var caps ExecutionCapabilities
	if err := json.Unmarshal([]byte(`{"maxIterations":`+longInteger+`e-1000}`), &caps); err != nil {
		t.Fatalf("exact long-mantissa integer: %v", err)
	}
	if caps.MaxIterations == nil || *caps.MaxIterations != 1 {
		t.Fatalf("long-mantissa integer = %v, want 1", caps.MaxIterations)
	}

	if err := json.Unmarshal([]byte(`{"maxIterations":`+longInteger+`e-999999999}`), &caps); err == nil {
		t.Fatal("huge negative exponent on nonzero long mantissa succeeded, want fractional error")
	}
}

func TestExecutionCapabilitiesDecodeReusesReceiverLikeEncodingJSON(t *testing.T) {
	trueValue := true
	iterations, executionTime := int64(4), int64(500)
	caps := ExecutionCapabilities{
		CodeExecution:    &trueValue,
		MaxIterations:    &iterations,
		MaxExecutionTime: &executionTime,
	}
	if err := json.Unmarshal([]byte(`{"codeExecution":false,"maxIterations":null,"future":true}`), &caps); err != nil {
		t.Fatal(err)
	}
	if caps.CodeExecution == nil || *caps.CodeExecution {
		t.Fatalf("codeExecution = %v, want explicit false", caps.CodeExecution)
	}
	if caps.MaxIterations != nil {
		t.Fatalf("maxIterations = %v, want nil after null", caps.MaxIterations)
	}
	if caps.MaxExecutionTime == nil || *caps.MaxExecutionTime != 500 {
		t.Fatalf("omitted maxExecutionTime = %v, want preserved 500", caps.MaxExecutionTime)
	}

	before := caps
	if err := json.Unmarshal([]byte(`{"sandboxed":true,"maxExecutionTime":1.5}`), &caps); err == nil {
		t.Fatal("invalid limit succeeded")
	}
	if !reflect.DeepEqual(caps, before) {
		t.Fatalf("receiver changed after failed decode\nbefore: %#v\n after: %#v", before, caps)
	}
}
