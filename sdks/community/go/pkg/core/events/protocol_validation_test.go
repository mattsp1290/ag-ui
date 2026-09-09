package events

import (
	"encoding/json"
	"testing"

	coretypes "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProtocolValidationAllowsPlainEmptyStringsAndChunks(t *testing.T) {
	events := []Event{
		NewRunStartedEvent("", ""),
		NewRunFinishedEvent("", ""),
		NewRunErrorEvent(""),
		NewStepStartedEvent(""),
		NewStepFinishedEvent(""),
		NewTextMessageStartEvent(""),
		NewTextMessageContentEvent("", ""),
		NewTextMessageEndEvent(""),
		NewToolCallStartEvent("", ""),
		NewToolCallArgsEvent("", ""),
		NewToolCallEndEvent(""),
		NewToolCallResultEvent("", "", ""),
		NewReasoningStartEvent(""),
		NewReasoningMessageContentEvent("", ""),
		NewReasoningMessageEndEvent(""),
		NewReasoningEndEvent(""),
		NewReasoningEncryptedValueEvent(ReasoningEncryptedValueSubtypeMessage, "", ""),
		NewSubagentStartedEvent("", ""),
		NewSubagentFinishedEvent(""),
		NewSubagentErrorEvent("", ""),
		NewActivityDeltaEvent("", "", nil),
		NewCustomEvent(""),
		NewTextMessageChunkEvent(nil, nil, nil),
		NewToolCallChunkEvent(),
		NewReasoningMessageChunkEvent(nil, nil),
	}
	for _, event := range events {
		require.NoError(t, event.Validate(), event.Type())
	}
}

func TestEventRoleLiterals(t *testing.T) {
	for _, role := range []string{"developer", "system", "assistant", "user"} {
		require.NoError(t, NewTextMessageStartEvent("", WithRole(role)).Validate())
		require.NoError(t, NewTextMessageChunkEvent(nil, &role, nil).Validate())
	}
	for _, role := range []string{"", "tool", "reasoning", "other"} {
		assert.Error(t, NewTextMessageStartEvent("", WithRole(role)).Validate())
		assert.Error(t, NewTextMessageChunkEvent(nil, &role, nil).Validate())
	}

	require.NoError(t, NewTextMessageStartEvent("").Validate(), "omitted role uses the peer default")
	require.NoError(t, NewReasoningMessageStartEvent("", "reasoning").Validate())
	assert.Error(t, NewReasoningMessageStartEvent("", "assistant").Validate())

	toolResult := NewToolCallResultEvent("", "", "")
	require.NoError(t, toolResult.Validate())
	badRole := "assistant"
	toolResult.Role = &badRole
	assert.Error(t, toolResult.Validate())
}

func TestActivitySnapshotRequiresObjectContent(t *testing.T) {
	for _, content := range []any{nil, "text", []any{}, true, 1} {
		assert.Error(t, NewActivitySnapshotEvent("", "", content).Validate())
	}
	require.NoError(t, NewActivitySnapshotEvent("", "", map[string]any{}).Validate())
}

func TestTerminalOutcomeValidation(t *testing.T) {
	run := NewRunFinishedEvent("", "")
	run.Outcome = &RunFinishedOutcome{Type: "unknown"}
	assert.Error(t, run.Validate())
	run.Outcome = &RunFinishedOutcome{Type: RunFinishedOutcomeTypeInterrupt}
	assert.Error(t, run.Validate())
	run.Outcome.Interrupts = []coretypes.Interrupt{{ID: "", Reason: ""}}
	require.NoError(t, run.Validate())

	subagent := NewSubagentFinishedEvent("")
	subagent.Outcome = &SubagentFinishedOutcome{Type: "unknown"}
	assert.Error(t, subagent.Validate())
	for _, ids := range [][]string{nil, {}} {
		subagent.Outcome = &SubagentFinishedOutcome{Type: SubagentFinishedOutcomeTypeSuspended, InterruptIDs: ids}
		require.NoError(t, subagent.Validate())
	}
}

func TestRunFinishedDecodeValidatesNestedInterruptPresence(t *testing.T) {
	for _, outcome := range []string{
		`{"type":"interrupt","interrupts":[{}]}`,
		`{"type":"interrupt","interrupts":[{"id":"i"}]}`,
		`{"type":"interrupt","interrupts":[{"reason":"r"}]}`,
		`{"type":"interrupt","interrupts":[{"id":null,"reason":"r"}]}`,
		`{"type":"interrupt","interrupts":[{"id":"i","reason":false}]}`,
	} {
		_, err := EventFromJSON([]byte(`{"type":"RUN_FINISHED","threadId":"","runId":"","outcome":` + outcome + `}`))
		assert.Error(t, err, outcome)
	}

	event, err := EventFromJSON([]byte(`{"type":"RUN_FINISHED","threadId":"","runId":"","outcome":{"type":"interrupt","interrupts":[{"id":"","reason":""}]}}`))
	require.NoError(t, err)
	require.NoError(t, event.Validate())
}

func TestEmptyOptionalArraysRemainPresent(t *testing.T) {
	run := NewRunFinishedEvent("", "")
	run.TimestampMs = nil
	run.Usage = []TokenUsage{}
	data, err := json.Marshal(run)
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"RUN_FINISHED","threadId":"","runId":"","usage":[]}`, string(data))

	subagent := NewSubagentFinishedEvent("", WithSubagentSuspendedOutcome([]string{}))
	data, err = json.Marshal(subagent)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"interruptIds":[]`)

	subagent.Outcome.InterruptIDs = nil
	data, err = json.Marshal(subagent)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "interruptIds")
}

func TestOptionalPayloadArraysPreservePresence(t *testing.T) {
	runError := NewRunErrorEvent("")
	runError.TimestampMs = nil
	runError.Usage = []TokenUsage{}
	data, err := json.Marshal(runError)
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"RUN_ERROR","message":"","usage":[]}`, string(data))
	runError.Usage = nil
	data, err = json.Marshal(runError)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "usage")
	message := Message{Role: coretypes.RoleAssistant, ToolCalls: []ToolCall{}}
	snapshot := NewMessagesSnapshotEvent([]Message{message})
	require.NoError(t, snapshot.Validate())
	data, err = json.Marshal(snapshot)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"toolCalls":[]`)
	snapshot.Messages[0].ToolCalls = nil
	data, err = json.Marshal(snapshot)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "toolCalls")
}
