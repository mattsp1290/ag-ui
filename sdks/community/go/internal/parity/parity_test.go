package parity_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/types"
	jsoncodec "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/encoding/json"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/encoding/sse"
	"github.com/stretchr/testify/require"
)

// These structures intentionally use RawMessage. Unmarshalling through any
// interface{} without UseNumber would turn large wire integers into float64.
type parityCorpus struct {
	Version int          `json:"version"`
	Cases   []parityCase `json:"cases"`
}

type parityCase struct {
	normalizeDefaultRole bool
	ID                   string          `json:"id"`
	Kind                 string          `json:"kind"`
	Check                string          `json:"check,omitempty"`
	Input                json.RawMessage `json:"input"`
	Expected             json.RawMessage `json:"expected"`
	Valid                bool            `json:"valid"`
	EventType            string          `json:"event_type,omitempty"`
	GoType               string          `json:"go_type,omitempty"`
	Owner                string          `json:"owner"`
	Notes                string          `json:"notes,omitempty"`
}

type parityManifest struct {
	SourceFiles     map[string]string   `json:"source_files"`
	Version         int                 `json:"version"`
	Events          []string            `json:"events"`
	Roles           []string            `json:"roles"`
	CaseIDs         []string            `json:"case_ids"`
	ExpectedFailure []manifestFailure   `json:"expected_failures"`
	Normalizations  []roleNormalization `json:"normalizations"`
}

// The only baseline value insertion is the peers' TEXT_MESSAGE_START role
// default. Other field loss must remain a visible preservation failure.
type roleNormalization struct {
	CaseID    string `json:"case_id"`
	Path      string `json:"path"`
	Operation string `json:"operation"`
	Value     string `json:"value"`
	Reason    string `json:"reason"`
}

type manifestFailure struct {
	CaseID        string `json:"case_id"`
	Check         string `json:"check"`
	Owner         string `json:"owner"`
	Reason        string `json:"reason"`
	ErrorContains string `json:"error_contains,omitempty"`
}

type gapCandidate struct {
	CaseID string `json:"case_id"`
	Check  string `json:"check"`
	Owner  string `json:"owner"`
	Reason string `json:"reason"`
}

var protocolEvents = []string{
	"TEXT_MESSAGE_START", "TEXT_MESSAGE_CONTENT", "TEXT_MESSAGE_END", "TEXT_MESSAGE_CHUNK",
	"TOOL_CALL_START", "TOOL_CALL_ARGS", "TOOL_CALL_END", "TOOL_CALL_CHUNK", "TOOL_CALL_RESULT",
	"STATE_SNAPSHOT", "STATE_DELTA", "MESSAGES_SNAPSHOT", "ACTIVITY_SNAPSHOT", "ACTIVITY_DELTA",
	"RAW", "CUSTOM", "RUN_STARTED", "RUN_FINISHED", "RUN_ERROR", "STEP_STARTED", "STEP_FINISHED",
	"THINKING_START", "THINKING_END", "THINKING_TEXT_MESSAGE_START", "THINKING_TEXT_MESSAGE_CONTENT",
	"THINKING_TEXT_MESSAGE_END", "REASONING_START", "REASONING_MESSAGE_START", "REASONING_MESSAGE_CONTENT",
	"REASONING_MESSAGE_END", "REASONING_MESSAGE_CHUNK", "REASONING_END", "REASONING_ENCRYPTED_VALUE",
	"SUBAGENT_STARTED", "SUBAGENT_FINISHED", "SUBAGENT_ERROR",
}

var protocolRoles = []string{"developer", "system", "assistant", "user", "tool", "activity", "reasoning"}

func parityDataPath(name string) string {
	return filepath.Join("..", "..", "testdata", "parity", name)
}

func loadParityFiles(t *testing.T) (parityCorpus, parityManifest) {
	t.Helper()
	read := func(name string) []byte {
		b, err := os.ReadFile(parityDataPath(name))
		require.NoError(t, err)
		return b
	}
	var corpus parityCorpus
	data := read("fixtures.json")
	require.NoError(t, json.Unmarshal(data, &corpus))
	var presence struct {
		Cases []map[string]json.RawMessage `json:"cases"`
	}
	require.NoError(t, json.Unmarshal(data, &presence))
	for i, fields := range presence.Cases {
		require.Contains(t, []string{"true", "false"}, string(fields["valid"]), "case %d must declare valid", i)
		require.NotEmpty(t, fields["expected"], "case %d must declare expected", i)
	}
	var manifest parityManifest
	require.NoError(t, json.Unmarshal(read("manifest.json"), &manifest))
	require.Equal(t, 1, corpus.Version)
	require.Equal(t, 1, manifest.Version)
	return corpus, manifest
}

func TestParityInventoryIsExact(t *testing.T) {
	corpus, manifest := loadParityFiles(t)
	// Ordinary Go tests need no peer runtimes. Source fingerprints fence the
	// pinned field snapshot until the actual peer inventory/oracles are rerun.
	require.Len(t, manifest.SourceFiles, 11)
	for path, expected := range manifest.SourceFiles {
		require.True(t, strings.HasPrefix(path, "sdks/python/ag_ui/core/") || strings.HasPrefix(path, "sdks/typescript/packages/core/src/"))
		require.Equal(t, path, filepath.ToSlash(filepath.Clean(path)))
		data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", path))
		require.NoError(t, err)
		require.Equal(t, expected, fmt.Sprintf("%x", sha256.Sum256(data)), "peer source changed: rerun the schema inventory and review fixture coverage for %s", path)
	}
	require.ElementsMatch(t, protocolEvents, manifest.Events, "manifest event inventory changed")
	require.Len(t, manifest.Events, len(protocolEvents))
	require.ElementsMatch(t, protocolRoles, manifest.Roles, "manifest role inventory changed")
	require.Len(t, manifest.Roles, len(protocolRoles))
	seen := make(map[string]bool, len(corpus.Cases))
	eventSeen := make(map[string]bool)
	roleSeen := make(map[string]bool)
	for _, c := range corpus.Cases {
		require.NotEmpty(t, c.ID)
		require.False(t, seen[c.ID], "duplicate corpus case %q", c.ID)
		seen[c.ID] = true
		require.Contains(t, []string{"event", "request", "message", "content", "capabilities", "usage", "aggregate", "mapper"}, c.Kind)
		require.Contains(t, []string{"W2", "W3", "W4"}, c.Owner)
		require.NotEmpty(t, c.Input)
		if c.Valid && c.Kind == "event" {
			var wire struct {
				Type string `json:"type"`
			}
			require.NoError(t, json.Unmarshal(c.Input, &wire))
			require.Equal(t, wire.Type, c.EventType)
			require.NotEmpty(t, c.GoType)
			eventSeen[wire.Type] = true
		}
		if c.Valid && c.Kind == "message" {
			var wire struct {
				Role string `json:"role"`
			}
			require.NoError(t, json.Unmarshal(c.Input, &wire))
			roleSeen[wire.Role] = true
		}
	}
	require.ElementsMatch(t, protocolEvents, keys(eventSeen), "positive corpus must cover every event")
	require.ElementsMatch(t, protocolRoles, keys(roleSeen), "positive corpus must cover every message role")
	want := append([]string(nil), manifest.CaseIDs...)
	require.ElementsMatch(t, want, keys(seen), "manifest case_ids must exactly match corpus")
	require.Len(t, want, len(seen), "manifest case_ids contain duplicates")
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestParityCorpus(t *testing.T) {
	corpus, manifest := loadParityFiles(t)
	xfails := make(map[string]manifestFailure, len(manifest.ExpectedFailure))
	for _, x := range manifest.ExpectedFailure {
		key := x.CaseID + "\x00" + x.Check
		require.Empty(t, xfails[key], "duplicate expected failure %s/%s", x.CaseID, x.Check)
		require.NotEmpty(t, x.CaseID)
		require.NotEmpty(t, x.Check)
		require.Contains(t, []string{"W2", "W3", "W4"}, x.Owner)
		require.NotEmpty(t, x.Reason)
		require.NotEmpty(t, x.ErrorContains)
		xfails[key] = x
	}
	declared := make(map[string]bool)
	for _, c := range corpus.Cases {
		for _, check := range parityChecks(c) {
			declared[c.ID+"\x00"+check] = true
		}
	}
	for key := range xfails {
		require.True(t, declared[key], "expected failure references unknown case/check %q", key)
	}
	defaults := make(map[string]bool)
	for _, rule := range manifest.Normalizations {
		require.Equal(t, "/role", rule.Path)
		require.Equal(t, "insert-if-absent", rule.Operation)
		require.Equal(t, "assistant", rule.Value)
		require.NotEmpty(t, rule.Reason)
		require.False(t, defaults[rule.CaseID], "duplicate role normalization")
		require.True(t, declared[rule.CaseID+"\x00eventFromJSON"], "normalization references unknown case")
		defaults[rule.CaseID] = true
	}
	var gaps []gapCandidate
	for _, c := range corpus.Cases {
		c := c
		c.normalizeDefaultRole = defaults[c.ID]
		if c.normalizeDefaultRole {
			require.Equal(t, "TEXT_MESSAGE_START", c.EventType)
		}
		for _, check := range parityChecks(c) {
			check := check
			t.Run(c.ID+"/"+check, func(t *testing.T) {
				err := runParityCase(c, check)
				key := c.ID + "\x00" + check
				if x, ok := xfails[key]; ok {
					if err == nil {
						t.Errorf("unexpected pass for expected failure (%s): %s", x.Owner, x.Reason)
					} else if x.ErrorContains != "" && !strings.Contains(err.Error(), x.ErrorContains) {
						t.Errorf("expected failure did not match error_contains %q: %v", x.ErrorContains, err)
					}
					return
				}
				if err != nil {
					gaps = append(gaps, gapCandidate{CaseID: c.ID, Check: check, Owner: c.Owner, Reason: err.Error()})
					t.Errorf("parity gap: %v", err)
				}
			})
		}
	}
	writeGapCandidates(t, gaps)
}

func defaultParityCheck(c parityCase) string {
	switch c.Kind {
	case "event":
		return "eventFromJSON"
	case "capabilities", "aggregate", "mapper":
		return c.Kind
	default:
		return "marshal"
	}
}

func parityChecks(c parityCase) []string {
	if c.Check != "" {
		return []string{c.Check}
	}
	if c.Kind == "event" {
		return []string{"eventFromJSON", "eventDecoder", "jsonDecoder", "marshal", "jsonEncoder", "sseWriter", "validate"}
	}
	return []string{defaultParityCheck(c)}
}

func runParityCase(c parityCase, check string) error {
	if os.Getenv("AG_UI_PARITY_OUTPUT_DIR") != "" {
		return fmt.Errorf("generated-artifact mode is not supported by the ordinary corpus harness")
	}
	switch c.Kind {
	case "event":
		return runEventCase(c, check)
	case "request":
		var value types.RunAgentInput
		return runValueCase(c, check, &value)
	case "message":
		var value types.Message
		return runValueCase(c, check, &value)
	case "content":
		var value types.InputContent
		return runValueCase(c, check, &value)
	case "usage":
		var value events.TokenUsage
		return runValueCase(c, check, &value)
	case "capabilities", "aggregate", "mapper":
		return fmt.Errorf("unsupported feature: no public Go %s API", c.Kind)
	default:
		return fmt.Errorf("unsupported corpus kind %q", c.Kind)
	}
}

func runValueCase(c parityCase, check string, value any) error {
	if err := json.Unmarshal(c.Input, value); err != nil {
		return decodeResult(c, err)
	}
	if check == "validate" {
		if usage, ok := value.(*events.TokenUsage); ok {
			return decodeResult(c, usage.Validate())
		}
		// Shared payload validation is intentionally owned by W3. Until its
		// public opt-in validator exists, this row is an explicit W3 gap.
		return fmt.Errorf("unsupported feature: no public validation API for %s", c.Kind)
	}
	if check != "marshal" && check != "jsonMarshal" {
		return fmt.Errorf("unsupported check %q for %s", check, c.Kind)
	}
	out, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := compareJSON(c.Expected, out, c.normalizeDefaultRole); err != nil {
		return err
	}
	if !c.Valid {
		return fmt.Errorf("invalid case unexpectedly accepted")
	}
	return nil
}

func runEventCase(c parityCase, check string) error {
	decode := func() (events.Event, error) {
		switch check {
		case "eventFromJSON":
			return events.EventFromJSON(c.Input)
		case "eventDecoder":
			return events.NewEventDecoder(nil).DecodeEvent(c.EventType, c.Input)
		case "jsonDecoder":
			return jsoncodec.NewJSONDecoder(nil).Decode(context.Background(), c.Input)
		default:
			return events.EventFromJSON(c.Input)
		}
	}
	e, err := decode()
	if err != nil {
		return decodeResult(c, err)
	}
	if e == nil {
		return fmt.Errorf("decoder returned nil event")
	}
	if c.GoType != "" && reflect.TypeOf(e).String() != c.GoType {
		return fmt.Errorf("concrete type %s, want %s", reflect.TypeOf(e), c.GoType)
	}
	if c.EventType != "" && string(e.Type()) != c.EventType {
		return fmt.Errorf("event type %q, want %q", e.Type(), c.EventType)
	}
	var out []byte
	switch check {
	case "validate":
		err = e.Validate()
	case "marshal":
		out, err = json.Marshal(e)
	case "jsonEncoder":
		out, err = jsoncodec.NewJSONEncoder(nil).Encode(context.Background(), e)
	case "sseWriter":
		var b bytes.Buffer
		err = sse.NewSSEWriter().WriteEvent(context.Background(), &b, e)
		if err == nil {
			out = sseData(b.Bytes())
		}
	case "eventFromJSON", "eventDecoder", "jsonDecoder":
		out, err = e.ToJSON()
	default:
		return fmt.Errorf("unsupported event check %q", check)
	}
	if err != nil {
		if check == "validate" {
			return decodeResult(c, err)
		}
		return err
	}
	if check != "validate" {
		if err := compareJSON(c.Expected, out, c.normalizeDefaultRole); err != nil {
			return err
		}
	}
	if !c.Valid {
		return fmt.Errorf("invalid case unexpectedly accepted")
	}
	return nil
}

func sseData(frame []byte) []byte {
	for _, line := range bytes.Split(frame, []byte("\n")) {
		if bytes.HasPrefix(line, []byte("data: ")) {
			return bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data: ")))
		}
	}
	return nil
}

func compareJSON(want, got []byte, normalizeRole bool) error {
	var a, b any
	decode := func(data []byte, dst *any) error {
		d := json.NewDecoder(bytes.NewReader(data))
		d.UseNumber()
		return d.Decode(dst)
	}
	if err := decode(want, &a); err != nil {
		return fmt.Errorf("expected JSON: %w", err)
	}
	if err := decode(got, &b); err != nil {
		return fmt.Errorf("produced JSON: %w", err)
	}
	if normalizeRole {
		object, ok := b.(map[string]any)
		if !ok {
			return fmt.Errorf("role default requires event object")
		}
		if _, present := object["role"]; !present {
			object["role"] = "assistant"
		}
	}
	if !reflect.DeepEqual(a, b) {
		return fmt.Errorf("JSON mismatch at %s", jsonDifference(a, b, ""))
	}
	return nil
}

func decodeResult(c parityCase, err error) error {
	if c.Valid {
		return err
	}
	if err == nil {
		return fmt.Errorf("invalid case unexpectedly accepted")
	}
	return nil
}

func writeGapCandidates(t *testing.T, gaps []gapCandidate) {
	t.Helper()
	b, err := json.MarshalIndent(struct {
		Candidates []gapCandidate `json:"candidates"`
	}{gaps}, "", "  ")
	require.NoError(t, err)
	if os.Getenv("AG_UI_PARITY_DISCOVER_GAPS") != "" {
		fmt.Println(string(b))
	}
}

// Return a stable JSON pointer so a known gap cannot hide unrelated field loss.
func jsonDifference(want, got any, path string) string {
	if reflect.DeepEqual(want, got) {
		return ""
	}
	if a, ok := want.(map[string]any); ok {
		if b, ok := got.(map[string]any); ok {
			fields := make(map[string]bool, len(a)+len(b))
			for k := range a {
				fields[k] = true
			}
			for k := range b {
				fields[k] = true
			}
			ordered := keys(fields)
			sort.Strings(ordered)
			for _, k := range ordered {
				pointer := path + "/" + strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1")
				av, aok := a[k]
				bv, bok := b[k]
				if aok != bok {
					return pointer
				}
				if diff := jsonDifference(av, bv, pointer); diff != "" {
					return diff
				}
			}
		}
	}
	if a, ok := want.([]any); ok {
		if b, ok := got.([]any); ok && len(a) == len(b) {
			for i := range a {
				if diff := jsonDifference(a[i], b[i], fmt.Sprintf("%s/%d", path, i)); diff != "" {
					return diff
				}
			}
		}
	}
	if path == "" {
		return "/ (document)"
	}
	return path
}
