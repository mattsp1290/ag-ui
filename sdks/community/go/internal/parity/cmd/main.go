// Command parity-fixtures produces test-only cross-SDK parity artifacts.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/types"
	jsoncodec "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/encoding/json"
)

type corpus struct {
	Version int        `json:"version"`
	Cases   []testCase `json:"cases"`
}

type testCase struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	Check     string          `json:"check,omitempty"`
	Valid     *bool           `json:"valid"`
	Input     json.RawMessage `json:"input"`
	EventType string          `json:"event_type,omitempty"`
}

type envelope struct {
	Version      int      `json:"version"`
	Route        string   `json:"route"`
	CorpusSHA256 string   `json:"corpus_sha256"`
	Cases        []record `json:"cases"`
}

type record struct {
	ID          string          `json:"id"`
	Accepted    bool            `json:"accepted"`
	Value       json.RawMessage `json:"value,omitempty"`
	Error       string          `json:"error,omitempty"`
	Unsupported bool            `json:"unsupported,omitempty"`
}

type peerEnvelope struct {
	Version      int          `json:"version"`
	Route        string       `json:"route"`
	CorpusSHA256 string       `json:"corpus_sha256"`
	Cases        []peerRecord `json:"cases"`
}

type peerRecord struct {
	ID          string          `json:"id"`
	Accepted    *bool           `json:"accepted"`
	Value       json.RawMessage `json:"value,omitempty"`
	Error       string          `json:"error,omitempty"`
	Unsupported bool            `json:"unsupported,omitempty"`
}

type eventFactory func() events.Event

var eventFactories = map[string]eventFactory{
	"TEXT_MESSAGE_START":            func() events.Event { return &events.TextMessageStartEvent{} },
	"TEXT_MESSAGE_CONTENT":          func() events.Event { return &events.TextMessageContentEvent{} },
	"TEXT_MESSAGE_END":              func() events.Event { return &events.TextMessageEndEvent{} },
	"TEXT_MESSAGE_CHUNK":            func() events.Event { return &events.TextMessageChunkEvent{} },
	"TOOL_CALL_START":               func() events.Event { return &events.ToolCallStartEvent{} },
	"TOOL_CALL_ARGS":                func() events.Event { return &events.ToolCallArgsEvent{} },
	"TOOL_CALL_END":                 func() events.Event { return &events.ToolCallEndEvent{} },
	"TOOL_CALL_CHUNK":               func() events.Event { return &events.ToolCallChunkEvent{} },
	"TOOL_CALL_RESULT":              func() events.Event { return &events.ToolCallResultEvent{} },
	"STATE_SNAPSHOT":                func() events.Event { return &events.StateSnapshotEvent{} },
	"STATE_DELTA":                   func() events.Event { return &events.StateDeltaEvent{} },
	"MESSAGES_SNAPSHOT":             func() events.Event { return &events.MessagesSnapshotEvent{} },
	"ACTIVITY_SNAPSHOT":             func() events.Event { return &events.ActivitySnapshotEvent{} },
	"ACTIVITY_DELTA":                func() events.Event { return &events.ActivityDeltaEvent{} },
	"RAW":                           func() events.Event { return &events.RawEvent{} },
	"CUSTOM":                        func() events.Event { return &events.CustomEvent{} },
	"RUN_STARTED":                   func() events.Event { return &events.RunStartedEvent{} },
	"RUN_FINISHED":                  func() events.Event { return &events.RunFinishedEvent{} },
	"RUN_ERROR":                     func() events.Event { return &events.RunErrorEvent{} },
	"STEP_STARTED":                  func() events.Event { return &events.StepStartedEvent{} },
	"STEP_FINISHED":                 func() events.Event { return &events.StepFinishedEvent{} },
	"THINKING_START":                func() events.Event { return &events.ThinkingStartEvent{} },
	"THINKING_END":                  func() events.Event { return &events.ThinkingEndEvent{} },
	"THINKING_TEXT_MESSAGE_START":   func() events.Event { return &events.ThinkingTextMessageStartEvent{} },
	"THINKING_TEXT_MESSAGE_CONTENT": func() events.Event { return &events.ThinkingTextMessageContentEvent{} },
	"THINKING_TEXT_MESSAGE_END":     func() events.Event { return &events.ThinkingTextMessageEndEvent{} },
	"REASONING_START":               func() events.Event { return &events.ReasoningStartEvent{} },
	"REASONING_MESSAGE_START":       func() events.Event { return &events.ReasoningMessageStartEvent{} },
	"REASONING_MESSAGE_CONTENT":     func() events.Event { return &events.ReasoningMessageContentEvent{} },
	"REASONING_MESSAGE_END":         func() events.Event { return &events.ReasoningMessageEndEvent{} },
	"REASONING_MESSAGE_CHUNK":       func() events.Event { return &events.ReasoningMessageChunkEvent{} },
	"REASONING_END":                 func() events.Event { return &events.ReasoningEndEvent{} },
	"REASONING_ENCRYPTED_VALUE":     func() events.Event { return &events.ReasoningEncryptedValueEvent{} },
	"SUBAGENT_STARTED":              func() events.Event { return &events.SubagentStartedEvent{} },
	"SUBAGENT_FINISHED":             func() events.Event { return &events.SubagentFinishedEvent{} },
	"SUBAGENT_ERROR":                func() events.Event { return &events.SubagentErrorEvent{} },
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	corpusPath := flag.String("corpus", "", "path to parity fixtures.json")
	outputDir := flag.String("output-dir", "", "existing caller-owned output directory")
	source := flag.String("source", "", "peer producer to consume: python or typescript")
	flag.Parse()
	if *corpusPath == "" || *outputDir == "" || flag.NArg() != 0 {
		return errors.New("--corpus and --output-dir are required and positional arguments are not accepted")
	}
	if *source != "" && *source != "python" && *source != "typescript" {
		return fmt.Errorf("invalid --source %q: want python or typescript", *source)
	}
	info, err := os.Stat(*outputDir)
	if err != nil {
		return fmt.Errorf("output directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("output path %q is not a directory", *outputDir)
	}

	raw, err := os.ReadFile(*corpusPath)
	if err != nil {
		return fmt.Errorf("read corpus: %w", err)
	}
	var c corpus
	if err := json.Unmarshal(raw, &c); err != nil {
		return fmt.Errorf("decode corpus: %w", err)
	}
	if err := validateCorpus(c); err != nil {
		return err
	}
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	if *source != "" {
		return consumePeer(c, digest, *outputDir, *source)
	}
	if err := produce(c, digest, *outputDir, "go.direct", false); err != nil {
		return err
	}
	return produce(c, digest, *outputDir, "go.encoder", true)
}

func validateCorpus(c corpus) error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported corpus version %d", c.Version)
	}
	if len(c.Cases) == 0 {
		return errors.New("corpus has no cases")
	}
	seen := make(map[string]bool, len(c.Cases))
	for i, tc := range c.Cases {
		if tc.ID == "" || tc.Valid == nil {
			return fmt.Errorf("corpus case %d requires id and explicit validity", i)
		}
		if seen[tc.ID] {
			return fmt.Errorf("duplicate corpus case id %q", tc.ID)
		}
		seen[tc.ID] = true
		if !isObject(tc.Input) {
			return fmt.Errorf("corpus case %q input must be a JSON object", tc.ID)
		}
		switch tc.Kind {
		case "event":
			// UNKNOWN is the deliberately invalid Go sentinel fixture, never
			// an additional protocol event or a concrete factory entry.
			invalidSentinel := tc.EventType == "UNKNOWN" && !*tc.Valid && tc.Check == "jsonDecoder"
			if eventFactories[tc.EventType] == nil && !invalidSentinel {
				return fmt.Errorf("corpus case %q has unknown event_type %q", tc.ID, tc.EventType)
			}
			var wire struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(tc.Input, &wire); err != nil || wire.Type != tc.EventType {
				return fmt.Errorf("corpus case %q discriminator does not match event_type %q", tc.ID, tc.EventType)
			}
		case "message", "content", "request", "usage", "aggregate", "mapper", "capabilities":
			if tc.EventType != "" {
				return fmt.Errorf("non-event corpus case %q declares event_type", tc.ID)
			}
		default:
			return fmt.Errorf("corpus case %q has unknown kind %q", tc.ID, tc.Kind)
		}
		if tc.Check != "" && tc.Check != "validate" && tc.Check != "jsonDecoder" {
			return fmt.Errorf("corpus case %q has unknown check %q", tc.ID, tc.Check)
		}
		if tc.Check == "jsonDecoder" && tc.Kind != "event" {
			return fmt.Errorf("corpus case %q uses event decoding for %s", tc.ID, tc.Kind)
		}
	}
	return nil
}

func produce(c corpus, digest, outputDir, route string, useEncoder bool) error {
	out := envelope{Version: 1, Route: route, CorpusSHA256: digest, Cases: make([]record, 0, len(c.Cases))}
	for _, tc := range c.Cases {
		value, unsupported, err := decodeAndEncode(tc, tc.Input, useEncoder, false)
		out.Cases = append(out.Cases, makeRecord(tc.ID, value, unsupported, err))
	}
	return writeEnvelope(outputDir, route+".json", out)
}

func consumePeer(c corpus, digest, outputDir, source string) error {
	path := filepath.Join(outputDir, source+".produced.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read peer artifact: %w", err)
	}
	var peer peerEnvelope
	if err := unmarshalStrict(raw, &peer); err != nil {
		return fmt.Errorf("decode peer artifact: %w", err)
	}
	if peer.Version != 1 || peer.Route != source+".produced" || peer.CorpusSHA256 != digest {
		return errors.New("peer artifact version, route, or corpus digest does not match")
	}
	byID := make(map[string]peerRecord, len(peer.Cases))
	for _, rec := range peer.Cases {
		if rec.ID == "" || byID[rec.ID].ID != "" {
			return fmt.Errorf("peer artifact contains empty or duplicate case id %q", rec.ID)
		}
		if rec.Accepted == nil {
			return fmt.Errorf("peer case %q must explicitly declare accepted", rec.ID)
		}
		if *rec.Accepted {
			if len(rec.Value) == 0 || rec.Error != "" || rec.Unsupported {
				return fmt.Errorf("accepted peer case %q requires value and forbids error or unsupported", rec.ID)
			}
		} else if len(rec.Value) != 0 || rec.Error == "" {
			return fmt.Errorf("rejected peer case %q requires error and forbids value", rec.ID)
		}
		byID[rec.ID] = rec
	}
	if len(byID) != len(c.Cases) {
		return fmt.Errorf("peer artifact has %d cases; want %d", len(byID), len(c.Cases))
	}

	route := "go.from-" + source
	out := envelope{Version: 1, Route: route, CorpusSHA256: digest, Cases: make([]record, 0, len(c.Cases))}
	for _, tc := range c.Cases {
		peerRecord, ok := byID[tc.ID]
		if !ok {
			return fmt.Errorf("peer artifact is missing case %q", tc.ID)
		}
		if !*peerRecord.Accepted {
			out.Cases = append(out.Cases, record{ID: tc.ID, Error: "not round-tripped: " + peerRecord.Error, Unsupported: peerRecord.Unsupported})
			continue
		}
		if !validResultShape(tc.Kind, peerRecord.Value) {
			return fmt.Errorf("accepted peer case %q has invalid %s result shape", tc.ID, tc.Kind)
		}
		value, unsupported, decodeErr := decodeAndEncode(tc, peerRecord.Value, false, true)
		out.Cases = append(out.Cases, makeRecord(tc.ID, value, unsupported, decodeErr))
	}
	return writeEnvelope(outputDir, route+".json", out)
}

func isObject(raw json.RawMessage) bool {
	var object map[string]json.RawMessage
	return json.Unmarshal(raw, &object) == nil && object != nil
}

// Helper artifacts contain serialized helper results, not their input objects.
// A mapper can return null; aggregation returns an array of usage objects.
func validResultShape(kind string, raw json.RawMessage) bool {
	switch kind {
	case "mapper":
		return bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || isObject(raw)
	case "aggregate":
		var values []json.RawMessage
		if json.Unmarshal(raw, &values) != nil || values == nil {
			return false
		}
		for _, value := range values {
			if !isObject(value) {
				return false
			}
		}
		return true
	default:
		return isObject(raw)
	}
}

func decodeAndEncode(tc testCase, input json.RawMessage, useEncoder, fromPeer bool) (json.RawMessage, bool, error) {
	if tc.Kind == "aggregate" || tc.Kind == "mapper" || tc.Kind == "capabilities" {
		return nil, true, fmt.Errorf("Go SDK has no public %s API", tc.Kind)
	}
	if tc.Check == "validate" && (tc.Kind == "message" || tc.Kind == "content" || tc.Kind == "request") {
		return nil, true, fmt.Errorf("Go SDK has no public validation API for %s payloads", tc.Kind)
	}

	var value any
	switch tc.Kind {
	case "event":
		var event events.Event
		var err error
		if fromPeer {
			event, err = events.EventFromJSON(input)
		} else {
			factory, ok := eventFactories[tc.EventType]
			if !ok {
				return nil, false, fmt.Errorf("no concrete Go event type for %q", tc.EventType)
			}
			event = factory()
			err = json.Unmarshal(input, event)
		}
		if err != nil {
			return nil, false, err
		}
		if string(event.Type()) != tc.EventType {
			return nil, false, fmt.Errorf("event discriminator %q does not match event_type %q", event.Type(), tc.EventType)
		}
		if tc.Check == "validate" {
			if err := event.Validate(); err != nil {
				return nil, false, err
			}
		}
		if useEncoder {
			encoder := jsoncodec.NewJSONEncoder(nil)
			encoded, err := encoder.Encode(context.Background(), event)
			return encoded, false, err
		}
		encoded, err := json.Marshal(event)
		return encoded, false, err
	case "message":
		value = &types.Message{}
	case "content":
		value = &types.InputContent{}
	case "request":
		value = &types.RunAgentInput{}
	case "usage":
		usage := &events.TokenUsage{}
		if err := json.Unmarshal(input, usage); err != nil {
			return nil, false, err
		}
		if tc.Check == "validate" {
			if err := usage.Validate(); err != nil {
				return nil, false, err
			}
		}
		encoded, err := json.Marshal(usage)
		return encoded, false, err
	default:
		return nil, true, fmt.Errorf("unsupported corpus kind %q", tc.Kind)
	}
	if err := json.Unmarshal(input, value); err != nil {
		return nil, false, err
	}
	encoded, err := json.Marshal(value)
	return encoded, false, err
}

func unmarshalStrict(data []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func makeRecord(id string, value json.RawMessage, unsupported bool, err error) record {
	if err != nil {
		return record{ID: id, Error: err.Error(), Unsupported: unsupported}
	}
	return record{ID: id, Accepted: true, Value: value}
}

func writeEnvelope(outputDir, name string, out envelope) error {
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", name, err)
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(outputDir, "."+name+"-*")
	if err != nil {
		return fmt.Errorf("create temporary output: %w", err)
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		tmp.Close()
		if !ok {
			os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", name, err)
	}
	if err := os.Rename(tmpName, filepath.Join(outputDir, name)); err != nil {
		return fmt.Errorf("install %s: %w", name, err)
	}
	ok = true
	return nil
}
