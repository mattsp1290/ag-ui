package event

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

func TestParsePreservesModernEventFields(t *testing.T) {
	t.Run("encrypted reasoning and metadata", func(t *testing.T) {
		event, err := Parse([]byte(`{"type":"REASONING_ENCRYPTED_VALUE","subtype":"message","entityId":"m1","encryptedValue":"opaque-secret","metadata":{"trace":"abc"}}`))
		if err != nil {
			t.Fatal(err)
		}
		got, ok := event.(*events.ReasoningEncryptedValueEvent)
		if !ok {
			t.Fatalf("got %T", event)
		}
		if got.EncryptedValue != "opaque-secret" || got.GetBaseEvent().Metadata["trace"] != "abc" {
			t.Fatalf("fields were not preserved: %#v", got)
		}
	})

	t.Run("usage", func(t *testing.T) {
		event, err := Parse([]byte(`{"type":"RUN_FINISHED","threadId":"t1","runId":"r1","usage":[{"provider":"openai","model":"gpt","inputTokens":3,"outputTokens":2,"totalTokens":5}]}`))
		if err != nil {
			t.Fatal(err)
		}
		got, ok := event.(*events.RunFinishedEvent)
		if !ok || len(got.Usage) != 1 || got.Usage[0].TotalTokens == nil || *got.Usage[0].TotalTokens != 5 {
			t.Fatalf("usage was not preserved: %#v", event)
		}
	})
}

func TestParseWrapsCanonicalDecoderErrors(t *testing.T) {
	_, err := Parse([]byte(`{"type":`))
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) || !strings.Contains(err.Error(), "failed to decode SSE event") {
		t.Fatalf("expected a wrapped JSON syntax error, got %v", err)
	}
}
