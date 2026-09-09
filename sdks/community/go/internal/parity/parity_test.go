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
	"regexp"
	"sort"
	"strconv"
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
	PeerValidity         map[string]bool            `json:"peer_validity"`
	ExpectedByLanguage   map[string]json.RawMessage `json:"expected_by_language"`
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
	RouteExceptions     []routeException                        `json:"route_exceptions"`
	RouteNormalizations []routeNormalization                    `json:"route_normalizations"`
	SchemaFields        map[string]any                          `json:"schema_fields"`
	SourceFiles         map[string]string                       `json:"source_files"`
	Version             int                                     `json:"version"`
	PinnedUpstream      string                                  `json:"pinned_upstream"`
	ImplementationBase  string                                  `json:"implementation_base"`
	Events              []string                                `json:"events"`
	Roles               []string                                `json:"roles"`
	GoSentinel          string                                  `json:"go_sentinel"`
	CaseIDs             []string                                `json:"case_ids"`
	ExpectedFailure     []manifestFailure                       `json:"expected_failures"`
	Normalizations      []roleNormalization                     `json:"normalizations"`
	Helpers             map[string]manifestHelper               `json:"helpers"`
	Nonshared           []manifestDifference                    `json:"nonshared"`
	Generated           manifestGenerated                       `json:"generated_artifacts"`
	Coverage            map[string]map[string]coverageReference `json:"coverage"`
}

type manifestDifference struct {
	Scope      string `json:"scope"`
	Difference string `json:"difference"`
}
type coverageReference struct {
	CaseID   string `json:"case_id"`
	Document string `json:"document"`
	Path     string `json:"path"`
}

type manifestHelper struct {
	Go         string   `json:"go"`
	Python     string   `json:"python"`
	TypeScript string   `json:"typescript"`
	Owner      string   `json:"owner"`
	Cases      []string `json:"cases"`
}
type manifestGenerated struct {
	Status            string            `json:"status"`
	Files             map[string]string `json:"files"`
	ImplementedRoutes []string          `json:"implemented_routes"`
	OutputEnv         string            `json:"output_env"`
	RequiredRoutes    []string          `json:"required_routes"`
	ExpectedCaseCount int               `json:"expected_case_count"`
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
	CaseID      string `json:"case_id"`
	Check       string `json:"check"`
	Owner       string `json:"owner"`
	Reason      string `json:"reason"`
	ErrorEquals string `json:"error_equals,omitempty"`
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
	data = read("manifest.json")
	require.True(t, json.Valid(data), "manifest must be one JSON document")
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	require.NoError(t, decoder.Decode(&manifest))
	require.Equal(t, 1, corpus.Version)
	require.Equal(t, 1, manifest.Version)
	validateManifest(t, corpus, manifest)
	validateRouteRules(t, corpus, manifest)
	return corpus, manifest
}

func validateManifest(t *testing.T, corpus parityCorpus, m parityManifest) {
	t.Helper()
	sha40 := regexp.MustCompile(`^[0-9a-f]{40}$`)
	sha64 := regexp.MustCompile(`^[0-9a-f]{64}$`)
	require.Regexp(t, sha40, m.PinnedUpstream)
	require.Regexp(t, sha40, m.ImplementationBase)
	require.NotEmpty(t, m.SchemaFields)
	for path, sum := range m.SourceFiles {
		require.NotEmpty(t, path)
		require.Regexp(t, sha64, sum)
	}
	require.Equal(t, "UNKNOWN", m.GoSentinel)
	require.NotEmpty(t, m.Nonshared)
	for _, n := range m.Nonshared {
		require.NotEmpty(t, n.Scope)
		require.NotEmpty(t, n.Difference)
	}
	require.Equal(t, "bidirectional", m.Generated.Status)
	require.Equal(t, "AG_UI_PARITY_OUTPUT_DIR", m.Generated.OutputEnv)
	require.Equal(t, len(corpus.Cases), m.Generated.ExpectedCaseCount, "generated case count must match corpus")
	require.Len(t, m.Generated.RequiredRoutes, 8)
	require.ElementsMatch(t, []string{"go.direct", "go.encoder", "python.produced", "typescript.produced", "python.from-go", "typescript.from-go", "go.from-python", "go.from-typescript"}, m.Generated.RequiredRoutes)
	require.Len(t, m.Generated.Files, len(m.Generated.RequiredRoutes))
	for _, route := range m.Generated.RequiredRoutes {
		require.Equal(t, route+".json", m.Generated.Files[route])
	}
	require.ElementsMatch(t, m.Generated.RequiredRoutes, m.Generated.ImplementedRoutes)
	byID := make(map[string]parityCase, len(corpus.Cases))
	for _, c := range corpus.Cases {
		byID[c.ID] = c
	}
	for name, h := range m.Helpers {
		var expectedIDs []string
		for _, c := range corpus.Cases {
			if c.Kind == name {
				expectedIDs = append(expectedIDs, c.ID)
			}
		}
		require.ElementsMatch(t, expectedIDs, h.Cases, "helper case inventory changed")
		require.Contains(t, []string{"aggregate", "mapper"}, name)
		require.Equal(t, "W4", h.Owner)
		names := map[string][3]string{
			"aggregate": {"AggregateTokenUsage", "aggregate_token_usage", "aggregateTokenUsage"},
			"mapper":    {"TokenUsageFromLangChainMetadata", "token_usage_from_langchain_metadata", "tokenUsageFromLangChainMetadata"},
		}
		require.Equal(t, names[name], [3]string{h.Go, h.Python, h.TypeScript})
		for _, id := range h.Cases {
			c, ok := byID[id]
			require.True(t, ok, "helper %s references missing case %s", name, id)
			require.Equal(t, name, c.Kind)
			require.Equal(t, "W4", c.Owner)
		}
	}
	require.Len(t, m.Helpers, 2)
	require.NotEmpty(t, m.Coverage, "coverage must be declared")
	for model, fields := range m.Coverage {
		require.NotEmpty(t, fields, "coverage for %s", model)
		for field, ref := range fields {
			require.NotEmpty(t, field)
			require.True(t, coverageReferenceExists(ref, byID), "coverage %s.%s references missing %+v", model, field, ref)
		}
	}
}

func coverageReferenceExists(ref coverageReference, byID map[string]parityCase) bool {
	c, ok := byID[ref.CaseID]
	if !ok || !c.Valid || !strings.HasPrefix(ref.Path, "/") {
		return false
	}
	var raw json.RawMessage
	switch ref.Document {
	case "input":
		raw = c.Input
	case "expected":
		raw = c.Expected
	default:
		return false
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return false
	}
	for _, part := range strings.Split(ref.Path[1:], "/") {
		token := strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		switch object := value.(type) {
		case map[string]any:
			value, ok = object[token]
			if !ok {
				return false
			}
		case []any:
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(object) {
				return false
			}
			value = object[index]
		default:
			return false
		}
	}
	return true
}

func TestParityInventoryIsExact(t *testing.T) {
	corpus, manifest := loadParityFiles(t)
	// This digest binds the independently audited live peer inventory to its
	// exact source inputs. Updating it requires rerunning both peer inventory
	// comparisons. Ordinary Go tests remain independent of peer runtimes.
	snapshot, err := json.Marshal(map[string]any{"schema_fields": manifest.SchemaFields, "source_files": manifest.SourceFiles, "coverage": manifest.Coverage})
	require.NoError(t, err)
	require.Equal(t, "808afc0fb8233ffd959b45ca1ed2b860b92a877cfb14d6bd0bd00ba9fb80b0d6", fmt.Sprintf("%x", sha256.Sum256(snapshot)), "audited peer inventory changed")
	requiredSources := []string{
		"sdks/typescript/packages/core/src/events.ts",
		"sdks/typescript/packages/core/src/types.ts",
		"sdks/typescript/packages/core/src/metadata.ts",
		"sdks/typescript/packages/core/src/capabilities.ts",
		"sdks/typescript/packages/core/src/token-usage.ts",
		"sdks/python/ag_ui/core/events.py",
		"sdks/python/ag_ui/core/types.py",
		"sdks/python/ag_ui/core/capabilities.py",
		"sdks/python/ag_ui/core/token_usage.py",
		"sdks/typescript/packages/core/src/index.ts",
		"sdks/python/ag_ui/core/__init__.py",
	}
	var sourcePaths []string
	for path := range manifest.SourceFiles {
		sourcePaths = append(sourcePaths, path)
	}
	require.ElementsMatch(t, requiredSources, sourcePaths, "exact peer source set required")
	for path, expected := range manifest.SourceFiles {
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
		require.NotEmpty(t, x.ErrorEquals)
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
					} else if x.ErrorEquals != "" && err.Error() != x.ErrorEquals {
						t.Errorf("expected failure did not match error_equals %q: %v", x.ErrorEquals, err)
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
	case "capabilities":
		var value types.AgentCapabilities
		return runValueCase(c, "marshal", &value)
	case "aggregate", "mapper":
		return runUsageHelperCase(c)
	default:
		return fmt.Errorf("unsupported corpus kind %q", c.Kind)
	}
}

func runUsageHelperCase(c parityCase) error {
	var value any
	var err error
	if c.Kind == "aggregate" {
		var input struct {
			Entries []events.TokenUsage `json:"entries"`
		}
		if err := json.Unmarshal(c.Input, &input); err != nil {
			return decodeResult(c, err)
		}
		value, err = events.AggregateTokenUsage(input.Entries)
	} else {
		var input struct {
			Metadata any    `json:"metadata"`
			Provider string `json:"provider"`
			Model    string `json:"model"`
		}
		decoder := json.NewDecoder(bytes.NewReader(c.Input))
		decoder.UseNumber()
		if err := decoder.Decode(&input); err != nil {
			return decodeResult(c, err)
		}
		value = events.TokenUsageFromLangChainMetadata(input.Metadata, input.Provider, input.Model)
	}
	if err != nil || !c.Valid {
		return decodeResult(c, err)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return compareJSON(c.Expected, encoded, false)
}

func runValueCase(c parityCase, check string, value any) error {
	if check == "validate" {
		switch value.(type) {
		case *types.RunAgentInput:
			return decodeResult(c, types.ValidateRunAgentInputJSON(c.Input))
		case *types.Message:
			return decodeResult(c, types.ValidateMessageJSON(c.Input))
		case *types.InputContent:
			return decodeResult(c, types.ValidateInputContentJSON(c.Input))
		}
	}
	if err := json.Unmarshal(c.Input, value); err != nil {
		return decodeResult(c, err)
	}
	if check == "validate" {
		if usage, ok := value.(*events.TokenUsage); ok {
			return decodeResult(c, usage.Validate())
		}
		return fmt.Errorf("unsupported validation target for %s", c.Kind)
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
		return fmt.Errorf("JSON mismatch at %s", strings.Join(jsonDifferences(a, b, ""), ", "))
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

// Report all differing locations so a registered gap cannot mask another
// field regression. Missing containers use their own pointer as the boundary.
func jsonDifferences(want, got any, path string) []string {
	if reflect.DeepEqual(want, got) {
		return nil
	}
	var differences []string
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
					differences = append(differences, pointer)
				} else {
					differences = append(differences, jsonDifferences(av, bv, pointer)...)
				}
			}
			return differences
		}
	}
	if a, ok := want.([]any); ok {
		if b, ok := got.([]any); ok {
			for i := 0; i < max(len(a), len(b)); i++ {
				pointer := fmt.Sprintf("%s/%d", path, i)
				if i >= len(a) || i >= len(b) {
					differences = append(differences, pointer)
				} else {
					differences = append(differences, jsonDifferences(a[i], b[i], pointer)...)
				}
			}
			return differences
		}
	}
	if path == "" {
		path = "/ (document)"
	}
	return []string{path}
}
