package agent

import (
	"bufio"
	"bytes"
	"context"
	"strings"
	"testing"

	aguitypes "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/types"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/encoding/sse"
	"github.com/cloudwego/eino/schema"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/example/server/internal/runstore"
)

func TestCloneWireMessagesKeepsCallerIdentityAndAssignsMissingIDOnce(t *testing.T) {
	in := []aguitypes.Message{
		{ID: "caller-user", Role: aguitypes.RoleUser, Content: "one"},
		{Role: aguitypes.RoleTool, Content: "result", ToolCallID: "call-1"},
	}
	first := cloneWireMessages(in)
	if first[0].ID != "caller-user" || first[1].ID == "" {
		t.Fatalf("unexpected cloned IDs: %#v", first)
	}
	second := cloneWireMessages(first)
	if second[1].ID != first[1].ID {
		t.Fatalf("assigned ID changed across a later turn: %q -> %q", first[1].ID, second[1].ID)
	}
}

func TestStreamTurnWireIdentityPreservesInterleavedSpansAndToolOwner(t *testing.T) {
	idx := 0
	fm := &fakeModel{chunks: []*schema.Message{
		{Role: schema.Assistant, Content: "first"},
		{Role: schema.Assistant, ReasoningContent: "think"},
		{Role: schema.Assistant, Content: "second"},
		{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{Index: &idx, ID: "call-1", Function: schema.FunctionCall{Name: "calculate", Arguments: `{}`}}}},
	}}
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	emit := NewEmitter(context.Background(), w, sse.NewSSEWriter(), "thread", "run", nil)
	turn, err := streamTurn(context.Background(), emit, fm, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	_ = w.Flush()
	if len(turn.WireMessages) != 4 {
		t.Fatalf("wire messages = %#v", turn.WireMessages)
	}
	wantRoles := []aguitypes.Role{aguitypes.RoleAssistant, aguitypes.RoleReasoning, aguitypes.RoleAssistant, aguitypes.RoleAssistant}
	seen := map[string]bool{}
	for i, msg := range turn.WireMessages {
		if msg.Role != wantRoles[i] || msg.ID == "" || seen[msg.ID] {
			t.Fatalf("message %d lost ordered distinct identity: %#v", i, msg)
		}
		seen[msg.ID] = true
	}
	owner := turn.WireMessages[3]
	if owner.ID != turn.ToolOwnerID || len(owner.ToolCalls) != 1 || owner.ToolCalls[0].ID != "call-1" {
		t.Fatalf("tool owner mismatch: %#v", owner)
	}
	if !strings.Contains(buf.String(), `"parentMessageId":"`+owner.ID+`"`) {
		t.Fatalf("TOOL_CALL_START did not reference owner %q:\n%s", owner.ID, buf.String())
	}
}

func TestRunKeepsCallerIDsAcrossTwoToolTurnsAndLaterUserTurn(t *testing.T) {
	m := &scriptedModel{turns: [][]*schema.Message{
		{toolCallChunk("call-1", "file_read", `{"path":"missing-one"}`)},
		{toolCallChunk("call-2", "file_read", `{"path":"missing-two"}`)},
		{textChunk("finished")},
	}}
	in := &aguitypes.RunAgentInput{
		ThreadID: "thread", RunID: "run",
		Messages: []aguitypes.Message{
			{ID: "user-original", Role: aguitypes.RoleUser, Content: "start"},
			{ID: "assistant-earlier", Role: aguitypes.RoleAssistant, Content: "ready"},
			{ID: "user-later", Role: aguitypes.RoleUser, Content: "continue"},
		},
	}
	out := runWithModel(t, m, in, runstore.New(), true, 8)
	for _, stableID := range []string{"user-original", "assistant-earlier", "user-later", "call-1", "call-2"} {
		if !strings.Contains(out, `"`+stableID+`"`) {
			t.Errorf("final stream lost stable identity %q:\n%s", stableID, out)
		}
	}
	if strings.Count(out, `"toolCallId":"call-1"`) < 2 || strings.Count(out, `"toolCallId":"call-2"`) < 2 {
		t.Fatalf("each tool call should retain one owner and matching result:\n%s", out)
	}
}
