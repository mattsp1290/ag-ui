package events

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestTokenUsageFromLangChainMetadataMapsOnlyCounts(t *testing.T) {
	usage := TokenUsageFromLangChainMetadata(map[string]any{
		"input_tokens": 100, "output_tokens": int64(50), "total_tokens": json.Number("150"),
		"input_token_details":  map[string]any{"cache_read": float64(10), "prompt": "secret"},
		"output_token_details": map[string]any{"reasoning": uint8(20), "completion": "secret"},
		"messages":             []any{"secret"},
	}, "anthropic", "claude")
	if usage == nil || usage.Provider != "anthropic" || usage.Model != "claude" ||
		*usage.InputTokens != 100 || *usage.OutputTokens != 50 || *usage.TotalTokens != 150 ||
		*usage.CachedInputTokens != 10 || *usage.ReasoningTokens != 20 {
		t.Fatalf("unexpected mapped usage: %#v", usage)
	}
}

func TestTokenUsageFromLangChainMetadataRequiresAValidCount(t *testing.T) {
	for name, metadata := range map[string]any{
		"nil": nil, "non-map": struct{}{}, "empty": map[string]any{},
		"labels only":  map[string]any{"prompt": "secret"},
		"invalid only": map[string]any{"input_tokens": "1", "output_tokens": true},
	} {
		t.Run(name, func(t *testing.T) {
			if got := TokenUsageFromLangChainMetadata(metadata, "p", "m"); got != nil {
				t.Fatalf("got %#v, want nil", got)
			}
		})
	}
	usage := TokenUsageFromLangChainMetadata(map[string]any{"input_tokens": 0}, "", "")
	if usage == nil || usage.InputTokens == nil || *usage.InputTokens != 0 {
		t.Fatalf("reported zero was lost: %#v", usage)
	}
}

func TestTokenUsageFromLangChainMetadataAcceptsAllGoIntegerWidths(t *testing.T) {
	values := []any{int(7), int8(7), int16(7), int32(7), int64(7), uint(7), uint8(7), uint16(7), uint32(7), uint64(7), uintptr(7)}
	for _, value := range values {
		usage := TokenUsageFromLangChainMetadata(map[string]any{"input_tokens": value}, "", "")
		if usage == nil || usage.InputTokens == nil || *usage.InputTokens != 7 {
			t.Errorf("%T(%v) was not accepted", value, value)
		}
	}
}

func TestTokenUsageFromLangChainMetadataNumericGuards(t *testing.T) {
	invalid := []any{-1, int64(-1), uint64(1 << 53), 1.5, float32(1.5), math.NaN(), math.Inf(1), "12", true,
		json.Number("-1"), json.Number("1.5"), json.Number("9007199254740992"), json.Number("NaN"), json.Number("1e999999999"), json.Number("1e-999999999")}
	for _, value := range invalid {
		if got := TokenUsageFromLangChainMetadata(map[string]any{"input_tokens": value}, "", ""); got != nil {
			t.Errorf("accepted invalid %T(%v): %#v", value, value, got)
		}
	}
	for _, value := range []any{float32(12), float64(12), json.Number("9007199254740991"), json.Number("12.000"), json.Number("12e2"), json.Number("0e999999999")} {
		if got := TokenUsageFromLangChainMetadata(map[string]any{"input_tokens": value}, "", ""); got == nil || got.InputTokens == nil {
			t.Errorf("rejected valid %T(%v)", value, value)
		}
	}
}

func TestTokenUsageFromLangChainMetadataKeepsFieldsIndependent(t *testing.T) {
	usage := TokenUsageFromLangChainMetadata(map[string]any{
		"input_tokens": "bad", "output_tokens": 2,
		"input_token_details":  5,
		"output_token_details": map[string]any{"reasoning": 3, "cache_read": 99},
	}, "", "")
	if usage == nil || usage.InputTokens != nil || usage.OutputTokens == nil || *usage.OutputTokens != 2 ||
		usage.CachedInputTokens != nil || usage.ReasoningTokens == nil || *usage.ReasoningTokens != 3 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
}

func TestAggregateTokenUsageGroupsInFirstSeenOrder(t *testing.T) {
	entries := []TokenUsage{
		{Provider: "a b", Model: "c", InputTokens: TokenCount(1), OutputTokens: TokenCount(0)},
		{Provider: "a", Model: "b c", InputTokens: TokenCount(2)},
		{Provider: "a b", Model: "c", InputTokens: TokenCount(3), ReasoningTokens: TokenCount(4)},
		{InputTokens: TokenCount(5)}, {InputTokens: TokenCount(6)},
	}
	got, err := AggregateTokenUsage(entries)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Provider != "a b" || got[1].Provider != "a" || got[2].Provider != "" {
		t.Fatalf("unexpected groups/order: %#v", got)
	}
	if *got[0].InputTokens != 4 || got[0].OutputTokens == nil || *got[0].OutputTokens != 0 || *got[0].ReasoningTokens != 4 || got[0].TotalTokens != nil || *got[2].InputTokens != 11 {
		t.Fatalf("unexpected sums/presence: %#v", got)
	}
	if empty, err := AggregateTokenUsage(nil); err != nil || len(empty) != 0 {
		t.Fatalf("empty aggregation = %#v, %v", empty, err)
	}
}

func TestAggregateTokenUsageValidatesEntries(t *testing.T) {
	_, err := AggregateTokenUsage([]TokenUsage{{InputTokens: TokenCount(1)}, {OutputTokens: TokenCount(-1)}})
	if err == nil || !strings.Contains(err.Error(), "usage[1]") || !strings.Contains(err.Error(), "outputTokens") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestAggregateTokenUsageDetectsEveryFieldOverflow(t *testing.T) {
	fields := []struct {
		name string
		set  func(*TokenUsage, *int64)
	}{
		{"inputTokens", func(u *TokenUsage, v *int64) { u.InputTokens = v }},
		{"outputTokens", func(u *TokenUsage, v *int64) { u.OutputTokens = v }},
		{"totalTokens", func(u *TokenUsage, v *int64) { u.TotalTokens = v }},
		{"reasoningTokens", func(u *TokenUsage, v *int64) { u.ReasoningTokens = v }},
		{"cachedInputTokens", func(u *TokenUsage, v *int64) { u.CachedInputTokens = v }},
	}
	for _, field := range fields {
		t.Run(field.name, func(t *testing.T) {
			first, second := TokenUsage{Provider: "p"}, TokenUsage{Provider: "p"}
			field.set(&first, TokenCount(math.MaxInt64))
			field.set(&second, TokenCount(1))
			_, err := AggregateTokenUsage([]TokenUsage{first, second})
			if err == nil || !strings.Contains(err.Error(), "usage[1]."+field.name) {
				t.Fatalf("unexpected overflow error: %v", err)
			}
		})
	}
}

func TestAggregateTokenUsageDoesNotAliasOrMutateInputs(t *testing.T) {
	count := int64(7)
	entries := []TokenUsage{{InputTokens: &count}}
	got, err := AggregateTokenUsage(entries)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].InputTokens == entries[0].InputTokens {
		t.Fatal("result aliases input pointer")
	}
	*got[0].InputTokens = 99
	if count != 7 {
		t.Fatalf("input mutated to %d", count)
	}
	count = 8
	if *got[0].InputTokens != 99 {
		t.Fatal("result changed with input")
	}
	if got[0].TotalTokens != nil {
		t.Fatal("totalTokens was synthesized")
	}
}
