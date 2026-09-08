package parity_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type routeException struct {
	CaseID   string          `json:"case_id"`
	Route    string          `json:"route"`
	Accepted *bool           `json:"accepted"`
	Value    json.RawMessage `json:"value,omitempty"`
	Reason   string          `json:"reason"`
}

type routeNormalization struct {
	CaseID    string `json:"case_id"`
	Route     string `json:"route"`
	Path      string `json:"path"`
	Operation string `json:"operation"`
	Reason    string `json:"reason"`
}

var producerFor = map[string]string{
	"python.from-go": "go.direct", "typescript.from-go": "go.encoder",
	"go.from-python": "python.produced", "go.from-typescript": "typescript.produced",
}

func validateRouteRules(t *testing.T, corpus parityCorpus, manifest parityManifest) {
	t.Helper()
	seen := map[string]bool{}
	for _, rule := range manifest.RouteExceptions {
		key := rule.Route + "/" + rule.CaseID
		require.False(t, seen[key], "duplicate route exception")
		seen[key] = true
		require.Contains(t, manifest.CaseIDs, rule.CaseID)
		require.Contains(t, manifest.Generated.RequiredRoutes, rule.Route)
		require.NotNil(t, rule.Accepted)
		require.NotEmpty(t, rule.Reason)
		for _, c := range corpus.Cases {
			if c.ID == rule.CaseID {
				accepted, value := producerExpectation(c, rule.Route)
				require.True(t, accepted != *rule.Accepted || (len(rule.Value) > 0 && !bytes.Equal(value, rule.Value)), "redundant route exception %s", key)
			}
		}
	}
	seen = map[string]bool{}
	for _, rule := range manifest.RouteNormalizations {
		key := rule.Route + "/" + rule.CaseID + rule.Path
		require.False(t, seen[key], "duplicate route normalization")
		seen[key] = true
		require.Equal(t, "aggregate.empty_labels", rule.CaseID)
		require.Contains(t, []string{"go.from-python", "go.from-typescript"}, rule.Route)
		require.Contains(t, []string{"/0/provider", "/0/model"}, rule.Path)
		require.Equal(t, "omit-if-empty-string", rule.Operation)
		require.NotEmpty(t, rule.Reason)
	}
}

// Input records require explicit status; a missing flag is not a rejection.
func (r *artifactRecord) UnmarshalJSON(data []byte) error {
	var wire struct {
		ID          string          `json:"id"`
		Accepted    *bool           `json:"accepted"`
		Value       json.RawMessage `json:"value"`
		Error       string          `json:"error"`
		Unsupported bool            `json:"unsupported"`
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&wire); err != nil {
		return err
	}
	if wire.Accepted == nil {
		return fmt.Errorf("record %q has no explicit status", wire.ID)
	}
	if *wire.Accepted && (len(wire.Value) == 0 || wire.Error != "" || wire.Unsupported) {
		return fmt.Errorf("accepted record %q has contradictory fields", wire.ID)
	}
	if !*wire.Accepted && (len(wire.Value) != 0 || wire.Error == "") {
		return fmt.Errorf("rejected record %q requires only an error", wire.ID)
	}
	*r = artifactRecord{wire.ID, *wire.Accepted, wire.Value, wire.Error, wire.Unsupported}
	return nil
}

func producerExpectation(c parityCase, route string) (bool, json.RawMessage) {
	accepted, value := c.Valid, c.Expected
	if route == "python.produced" || route == "typescript.produced" {
		language := strings.SplitN(route, ".", 2)[0]
		if override, ok := c.PeerValidity[language]; ok {
			accepted = override
		}
		if override, ok := c.ExpectedByLanguage[language]; ok {
			value = override
		}
	}
	return accepted, value
}

func TestGeneratedArtifacts(t *testing.T) {
	dir, provided := os.LookupEnv("AG_UI_PARITY_OUTPUT_DIR")
	phase := os.Getenv("AG_UI_PARITY_PHASE")
	if !provided {
		require.Empty(t, phase, "artifact phase requires an output directory")
		t.Skip("ordinary corpus mode")
	}
	require.NotEmpty(t, dir, "output directory must not be empty")
	require.Equal(t, "verify", phase, "Go generated artifacts require verify mode")
	corpus, manifest := loadParityFiles(t)
	data, err := os.ReadFile(parityDataPath("fixtures.json"))
	require.NoError(t, err)
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	artifacts := map[string]map[string]artifactRecord{}
	for _, route := range manifest.Generated.RequiredRoutes {
		data, err := os.ReadFile(filepath.Join(dir, manifest.Generated.Files[route]))
		require.NoError(t, err, "mandatory artifact %s", route)
		require.True(t, json.Valid(data))
		var document artifactDocument
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		require.NoError(t, decoder.Decode(&document), route)
		require.Equal(t, 1, document.Version)
		require.Equal(t, route, document.Route)
		require.Equal(t, digest, document.CorpusSHA256, "stale artifact %s", route)
		records := map[string]artifactRecord{}
		for _, record := range document.Cases {
			_, duplicate := records[record.ID]
			require.False(t, duplicate, "duplicate artifact case %s", record.ID)
			records[record.ID] = record
		}
		var ids []string
		for id := range records {
			ids = append(ids, id)
		}
		require.ElementsMatch(t, manifest.CaseIDs, ids, "artifact case inventory: %s", route)
		artifacts[route] = records
	}
	usedRules := map[string]bool{}
	for _, route := range manifest.Generated.RequiredRoutes {
		for _, c := range corpus.Cases {
			t.Run(route+"/"+c.ID, func(t *testing.T) {
				record := artifacts[route][c.ID]
				accepted, expected := producerExpectation(c, route)
				if source, consumer := producerFor[route]; consumer {
					parent := artifacts[source][c.ID]
					if !parent.Accepted {
						require.False(t, record.Accepted, "source rejection must propagate")
						require.Equal(t, parent.Unsupported, record.Unsupported)
						return
					}
					expected = parent.Value
				}
				for _, rule := range manifest.RouteExceptions {
					if rule.CaseID == c.ID && rule.Route == route {
						accepted = *rule.Accepted
						if len(rule.Value) > 0 {
							expected = rule.Value
						}
					}
				}
				require.False(t, record.Unsupported, "unimplemented route: %s", record.Error)
				require.Equal(t, accepted, record.Accepted, "unexpected acceptance: %s", record.Error)
				if !accepted {
					return
				}
				want := normalizeArtifact(t, expected, c.ID, route, manifest, true, usedRules)
				got := normalizeArtifact(t, record.Value, c.ID, route, manifest, false, usedRules)
				require.NoError(t, compareJSON(want, got, false))
			})
		}
	}
	for _, rule := range manifest.RouteNormalizations {
		if !usedRules[rule.Route+"/"+rule.CaseID+rule.Path] {
			t.Errorf("unused route normalization: %+v", rule)
		}
	}
	if len(manifest.ExpectedFailure) > 0 {
		t.Errorf("parity incomplete: %d expected failures remain", len(manifest.ExpectedFailure))
	}
}

func normalizeArtifact(t *testing.T, raw json.RawMessage, caseID, route string, manifest parityManifest, expected bool, used map[string]bool) json.RawMessage {
	t.Helper()
	var value any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	require.NoError(t, d.Decode(&value))
	for _, rule := range manifest.Normalizations {
		if rule.CaseID == caseID {
			object, ok := value.(map[string]any)
			require.True(t, ok)
			if _, present := object["role"]; !present {
				object["role"] = rule.Value
			}
		}
	}
	if expected {
		for _, rule := range manifest.RouteNormalizations {
			if rule.CaseID != caseID || rule.Route != route {
				continue
			}
			parts := strings.Split(strings.TrimPrefix(rule.Path, "/"), "/")
			index, err := strconv.Atoi(parts[0])
			require.NoError(t, err)
			items, ok := value.([]any)
			if !ok || index >= len(items) {
				continue
			}
			object, ok := items[index].(map[string]any)
			if ok && object[parts[1]] == "" {
				delete(object, parts[1])
				used[route+"/"+caseID+rule.Path] = true
			}
		}
	}
	out, err := json.Marshal(value)
	require.NoError(t, err)
	return out
}
