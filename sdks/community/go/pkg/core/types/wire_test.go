package types

import (
	"encoding/json"
	"reflect"
	"testing"
)

type wireState string

func (s wireState) MarshalJSON() ([]byte, error) { return []byte(s), nil }

func decodeWireObject(t *testing.T, value any) map[string]any {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("decode marshaled JSON: %v", err)
	}
	return got
}

func TestToolCallEncryptedValueWirePresence(t *testing.T) {
	empty := ""
	tests := []struct {
		name    string
		value   *string
		present bool
	}{
		{name: "absent", value: nil, present: false},
		{name: "explicit empty", value: &empty, present: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeWireObject(t, ToolCall{EncryptedValue: tt.value})
			value, present := got["encryptedValue"]
			if present != tt.present {
				t.Fatalf("encryptedValue presence = %v, want %v; JSON = %#v", present, tt.present, got)
			}
			if present && value != "" {
				t.Fatalf("encryptedValue = %#v, want empty string", value)
			}
		})
	}
}

func TestInputContentMarshalRequiredEmptyFields(t *testing.T) {
	tests := []struct {
		name  string
		input InputContent
		want  map[string]any
	}{
		{name: "text", input: InputContent{Type: InputContentTypeText}, want: map[string]any{"type": "text", "text": ""}},
		{name: "binary", input: InputContent{Type: InputContentTypeBinary}, want: map[string]any{"type": "binary", "mimeType": ""}},
		{name: "image", input: InputContent{Type: InputContentTypeImage}, want: map[string]any{"type": "image", "source": nil}},
		{name: "audio", input: InputContent{Type: InputContentTypeAudio}, want: map[string]any{"type": "audio", "source": nil}},
		{name: "video", input: InputContent{Type: InputContentTypeVideo}, want: map[string]any{"type": "video", "source": nil}},
		{name: "document", input: InputContent{Type: InputContentTypeDocument}, want: map[string]any{"type": "document", "source": nil}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decodeWireObject(t, tt.input); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("JSON = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestInputContentSourceMarshalByDiscriminator(t *testing.T) {
	data := decodeWireObject(t, InputContentSource{Type: InputContentSourceTypeData})
	if got, ok := data["mimeType"]; !ok || got != "" {
		t.Fatalf("data mimeType = %#v, present %v; want required empty string", got, ok)
	}
	url := decodeWireObject(t, InputContentSource{Type: InputContentSourceTypeURL})
	if _, ok := url["mimeType"]; ok {
		t.Fatalf("URL source unexpectedly contains empty mimeType: %#v", url)
	}
}

func TestInputContentMarshalDoesNotMutate(t *testing.T) {
	source := &InputContentSource{Type: InputContentSourceTypeData}
	input := InputContent{
		Type: InputContentTypeImage, Text: "legacy", ID: "legacy-id",
		Source: source, Metadata: map[string]any{"kept": true},
	}
	want := input
	wire := decodeWireObject(t, input)
	if wire["text"] != "legacy" || wire["id"] != "legacy-id" || wire["metadata"] == nil {
		t.Fatalf("marshal dropped populated compatibility fields: %#v", wire)
	}
	if !reflect.DeepEqual(input, want) || input.Source != source {
		t.Fatalf("marshal mutated input: got %#v, want %#v", input, want)
	}
}

func TestRunAgentInputMarshalWireDistinctions(t *testing.T) {
	nestedState := map[string]any{
		"false": false, "zero": 0, "empty": "", "array": []any{}, "nestedNull": nil,
	}
	tests := []struct {
		name          string
		input         RunAgentInput
		statePresent  bool
		resumePresent bool
	}{
		{name: "nil state and resume", input: RunAgentInput{}, statePresent: false, resumePresent: false},
		{name: "explicit empty resume", input: RunAgentInput{Resume: []ResumeEntry{}}, statePresent: false, resumePresent: true},
		{name: "false state", input: RunAgentInput{State: false}, statePresent: true, resumePresent: false},
		{name: "zero state", input: RunAgentInput{State: 0}, statePresent: true, resumePresent: false},
		{name: "empty string state", input: RunAgentInput{State: ""}, statePresent: true, resumePresent: false},
		{name: "empty object state", input: RunAgentInput{State: map[string]any{}}, statePresent: true, resumePresent: false},
		{name: "nested null state", input: RunAgentInput{State: nestedState}, statePresent: true, resumePresent: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeWireObject(t, tt.input)
			if _, ok := got["state"]; ok != tt.statePresent {
				t.Fatalf("state presence = %v, want %v; JSON = %#v", ok, tt.statePresent, got)
			}
			if _, ok := got["resume"]; ok != tt.resumePresent {
				t.Fatalf("resume presence = %v, want %v; JSON = %#v", ok, tt.resumePresent, got)
			}
			for _, key := range []string{"messages", "tools", "context"} {
				value, ok := got[key].([]any)
				if !ok || len(value) != 0 {
					t.Fatalf("%s = %#v, want []", key, got[key])
				}
			}
		})
	}
}

func TestRunAgentInputMarshalDoesNotMutateNilCollections(t *testing.T) {
	input := RunAgentInput{}
	if _, err := json.Marshal(input); err != nil {
		t.Fatal(err)
	}
	if input.Messages != nil || input.Tools != nil || input.Context != nil || input.Resume != nil {
		t.Fatalf("marshal mutated input collections: %#v", input)
	}
}

func TestRunAgentInputOuterNullStateNormalizesToOmission(t *testing.T) {
	var input RunAgentInput
	if err := json.Unmarshal([]byte(`{"state":null,"messages":[],"tools":[],"context":[]}`), &input); err != nil {
		t.Fatal(err)
	}
	if got := decodeWireObject(t, input); func() bool { _, ok := got["state"]; return ok }() {
		t.Fatalf("state should be omitted after outer null decode: %#v", got)
	}
}

func TestRunAgentInputStateUsesJSONRepresentation(t *testing.T) {
	t.Run("raw null omitted", func(t *testing.T) {
		got := decodeWireObject(t, RunAgentInput{State: json.RawMessage(" \n null \t")})
		if _, ok := got["state"]; ok {
			t.Fatalf("state should be omitted: %#v", got)
		}
	})
	t.Run("custom non-null retained", func(t *testing.T) {
		got := decodeWireObject(t, RunAgentInput{State: wireState(`{"nested":null}`)})
		if !reflect.DeepEqual(got["state"], map[string]any{"nested": nil}) {
			t.Fatalf("state = %#v", got["state"])
		}
	})
	t.Run("unsupported state still errors", func(t *testing.T) {
		if _, err := json.Marshal(RunAgentInput{State: func() {}}); err == nil {
			t.Fatal("marshal succeeded for unsupported state")
		}
	})
}
