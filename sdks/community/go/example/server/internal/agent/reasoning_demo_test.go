package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	aguijson "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/encoding/json"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/encoding/sse"
)

func TestReasoningDemoBalancedStableSnapshot(t *testing.T) {
	var raw bytes.Buffer
	w := bufio.NewWriter(&raw)
	emit := NewEmitter(context.Background(), w, sse.NewSSEWriter(), "thread", "run", nil)
	ReasoningDemo{}.Run(context.Background(), emit, nil, "thread", "run")
	_ = w.Flush()
	if emit.Err() != nil || emit.EncErr() != nil {
		t.Fatalf("emitter errors: %v / %v", emit.Err(), emit.EncErr())
	}

	var frames []map[string]any
	for _, line := range strings.Split(raw.String(), "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var frame map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &frame); err != nil {
			t.Fatal(err)
		}
		frames = append(frames, frame)
	}
	want := []string{"RUN_STARTED", "REASONING_START", "REASONING_MESSAGE_START", "REASONING_MESSAGE_CONTENT", "REASONING_MESSAGE_CONTENT", "REASONING_MESSAGE_END", "REASONING_END", "TEXT_MESSAGE_START", "TEXT_MESSAGE_CONTENT", "TEXT_MESSAGE_END", "MESSAGES_SNAPSHOT", "RUN_FINISHED"}
	if len(frames) != len(want) {
		t.Fatalf("event count=%d want=%d: %s", len(frames), len(want), raw.String())
	}
	for i := range want {
		if frames[i]["type"] != want[i] {
			t.Fatalf("event %d type=%v want=%s", i, frames[i]["type"], want[i])
		}
	}
	if frames[0]["threadId"] != "thread" || frames[0]["runId"] != "run" || frames[11]["threadId"] != "thread" || frames[11]["runId"] != "run" {
		t.Fatalf("run lifecycle IDs do not match: start=%v finish=%v", frames[0], frames[11])
	}
	if frames[1]["messageId"] != frames[2]["messageId"] {
		t.Fatal("reasoning lifecycle IDs differ")
	}
	if frames[2]["role"] != "reasoning" {
		t.Fatalf("reasoning message role=%v want=reasoning", frames[2]["role"])
	}
	messages := frames[10]["messages"].([]any)
	reasoning := messages[0].(map[string]any)
	answer := messages[1].(map[string]any)
	if reasoning["id"] != frames[1]["messageId"] || answer["id"] != frames[7]["messageId"] {
		t.Fatal("snapshot IDs do not match streamed IDs")
	}
	decoder := aguijson.NewDecoder()
	encoded, _ := json.Marshal(frames[10])
	if _, err := decoder.Decode(context.Background(), encoded); err != nil {
		t.Fatalf("decoder rejected snapshot: %v", err)
	}
}

func TestReasoningDemoCancellationStopsWithoutTerminal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	raw := &cancelWriter{cancel: cancel, marker: `"type":"REASONING_MESSAGE_CONTENT"`}
	w := bufio.NewWriter(raw)
	emit := NewEmitter(ctx, w, sse.NewSSEWriter(), "thread", "run", cancel)
	ReasoningDemo{Pace: time.Hour}.Run(ctx, emit, nil, "thread", "run")
	_ = w.Flush()
	out := raw.String()
	if !strings.Contains(out, raw.marker) {
		t.Fatalf("demo did not emit partial stream: %s", out)
	}
	for _, forbidden := range []string{`"type":"REASONING_MESSAGE_END"`, `"type":"REASONING_END"`, `"type":"TEXT_MESSAGE_START"`, `"type":"MESSAGES_SNAPSHOT"`, `"type":"RUN_FINISHED"`} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("cancelled demo emitted trailing event %s: %s", forbidden, out)
		}
	}
}

type cancelWriter struct {
	bytes.Buffer
	cancel    context.CancelFunc
	marker    string
	cancelled bool
}

func (w *cancelWriter) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	if !w.cancelled && strings.Contains(w.Buffer.String(), w.marker) {
		w.cancelled = true
		w.cancel()
	}
	return n, err
}
