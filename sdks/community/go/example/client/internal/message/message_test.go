package message

import (
	"strings"
	"testing"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

func TestNewMessageRendersModernEvents(t *testing.T) {
	delta := "working through it"
	tests := []struct {
		name     string
		event    events.Event
		contains string
		excludes string
	}{
		{"reasoning content", events.NewReasoningMessageContentEvent("m1", delta), delta, ""},
		{"encrypted reasoning", events.NewReasoningEncryptedValueEvent(events.ReasoningEncryptedValueSubtypeMessage, "m1", "opaque-secret"), "Encrypted reasoning received", "opaque-secret"},
		{"subagent", events.NewSubagentStartedEvent("sub1", "researcher"), "researcher", ""},
		{"activity", events.NewActivitySnapshotEvent("m2", "progress", map[string]any{"percent": 50}), `"percent":50`, ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			msg := NewMessage(test.event)
			if msg == nil || len(msg.Strings()) == 0 || !strings.Contains(strings.Join(msg.Strings(), " "), test.contains) {
				t.Fatalf("unexpected rendering: %#v", msg)
			}
			if test.excludes != "" && strings.Contains(strings.Join(msg.Strings(), " "), test.excludes) {
				t.Fatalf("rendering exposed opaque content: %#v", msg.Strings())
			}
		})
	}
}
