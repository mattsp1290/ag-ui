package events

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/internal/jsonnumber"
)

// maxInteroperableTokenCount bounds the interoperable safe-integer domain,
// including TypeScript's number type.
const maxInteroperableTokenCount uint64 = 1<<53 - 1

type tokenUsageKey struct {
	provider string
	model    string
}

// AggregateTokenUsage sums entries by their provider/model pair while
// preserving the order in which each pair first appears. Empty labels use the
// existing TokenUsage representation and therefore group together.
//
// Counts above JavaScript's exact-integer range remain valid here because
// TokenUsage uses int64. Callers that need cross-binding exactness should keep
// values at or below 2^53-1.
func AggregateTokenUsage(entries []TokenUsage) ([]TokenUsage, error) {
	result := make([]TokenUsage, 0)
	groups := make(map[tokenUsageKey]int, len(entries))

	for entryIndex, entry := range entries {
		if err := entry.Validate(); err != nil {
			return nil, fmt.Errorf("usage[%d]: %w", entryIndex, err)
		}

		key := tokenUsageKey{provider: entry.Provider, model: entry.Model}
		groupIndex, ok := groups[key]
		if !ok {
			groupIndex = len(result)
			groups[key] = groupIndex
			result = append(result, TokenUsage{Provider: entry.Provider, Model: entry.Model})
		}

		target := &result[groupIndex]
		fields := []struct {
			name   string
			source *int64
			target **int64
		}{
			{"inputTokens", entry.InputTokens, &target.InputTokens},
			{"outputTokens", entry.OutputTokens, &target.OutputTokens},
			{"totalTokens", entry.TotalTokens, &target.TotalTokens},
			{"reasoningTokens", entry.ReasoningTokens, &target.ReasoningTokens},
			{"cachedInputTokens", entry.CachedInputTokens, &target.CachedInputTokens},
		}
		for _, field := range fields {
			if field.source == nil {
				continue
			}
			if *field.target == nil {
				*field.target = TokenCount(*field.source)
				continue
			}
			if *field.source > math.MaxInt64-**field.target {
				return nil, fmt.Errorf("usage[%d].%s: int64 overflow", entryIndex, field.name)
			}
			**field.target += *field.source
		}
	}

	return result, nil
}

// TokenUsageFromLangChainMetadata maps LangChain's JSON-like usage metadata
// into TokenUsage. Only the five documented numeric count locations are read;
// labels alone do not create an entry. Counts are capped at 2^53-1 so their
// values survive an exact round trip through TypeScript.
func TokenUsageFromLangChainMetadata(metadata any, provider, model string) *TokenUsage {
	root, ok := metadata.(map[string]any)
	if !ok {
		return nil
	}

	usage := &TokenUsage{Provider: provider, Model: model}
	usage.InputTokens = interoperableTokenCount(root["input_tokens"])
	usage.OutputTokens = interoperableTokenCount(root["output_tokens"])
	usage.TotalTokens = interoperableTokenCount(root["total_tokens"])
	if details, ok := root["input_token_details"].(map[string]any); ok {
		usage.CachedInputTokens = interoperableTokenCount(details["cache_read"])
	}
	if details, ok := root["output_token_details"].(map[string]any); ok {
		usage.ReasoningTokens = interoperableTokenCount(details["reasoning"])
	}

	if usage.InputTokens == nil && usage.OutputTokens == nil && usage.TotalTokens == nil &&
		usage.ReasoningTokens == nil && usage.CachedInputTokens == nil {
		return nil
	}
	return usage
}

func interoperableTokenCount(value any) *int64 {
	var count uint64
	switch number := value.(type) {
	case int:
		if number < 0 {
			return nil
		}
		count = uint64(number)
	case int8:
		if number < 0 {
			return nil
		}
		count = uint64(number)
	case int16:
		if number < 0 {
			return nil
		}
		count = uint64(number)
	case int32:
		if number < 0 {
			return nil
		}
		count = uint64(number)
	case int64:
		if number < 0 {
			return nil
		}
		count = uint64(number)
	case uint:
		count = uint64(number)
	case uint8:
		count = uint64(number)
	case uint16:
		count = uint64(number)
	case uint32:
		count = uint64(number)
	case uint64:
		count = number
	case uintptr:
		count = uint64(number)
	case float32:
		return interoperableFloatCount(float64(number))
	case float64:
		return interoperableFloatCount(number)
	case json.Number:
		return interoperableJSONNumber(number)
	default:
		return nil
	}
	if count > maxInteroperableTokenCount {
		return nil
	}
	converted := int64(count)
	return &converted
}

func interoperableFloatCount(value float64) *int64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 ||
		value > float64(maxInteroperableTokenCount) || math.Trunc(value) != value {
		return nil
	}
	count := int64(value)
	return &count
}

func interoperableJSONNumber(number json.Number) *int64 {
	count, err := jsonnumber.Int64([]byte(number))
	if err != nil || count < 0 || uint64(count) > maxInteroperableTokenCount {
		return nil
	}
	return &count
}
