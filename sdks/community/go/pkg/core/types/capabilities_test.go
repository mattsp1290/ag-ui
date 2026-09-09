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

func TestExecutionCapabilitiesRejectFractionalLimits(t *testing.T) {
	for _, payload := range []string{
		`{"maxIterations":1.5}`,
		`{"maxExecutionTime":2.25}`,
	} {
		var caps ExecutionCapabilities
		if err := json.Unmarshal([]byte(payload), &caps); err == nil {
			t.Fatalf("json.Unmarshal(%s) succeeded, want fractional int64 error", payload)
		}
	}
}
