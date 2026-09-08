package parity_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type artifactRecord struct {
	ID          string          `json:"id"`
	Accepted    bool            `json:"accepted"`
	Value       json.RawMessage `json:"value,omitempty"`
	Error       string          `json:"error,omitempty"`
	Unsupported bool            `json:"unsupported,omitempty"`
}

type artifactDocument struct {
	Version      int              `json:"version"`
	Route        string           `json:"route"`
	CorpusSHA256 string           `json:"corpus_sha256"`
	Cases        []artifactRecord `json:"cases"`
}

// Exercise the binary boundary and its actual serialized output. Peer runtime
// oracles are a separate gate; these tests require only Go and the corpus.
func TestGoArtifactCLI(t *testing.T) {
	corpus, manifest := loadParityFiles(t)
	dir := t.TempDir()
	binary := filepath.Join(dir, "parity")
	output, err := exec.Command("go", "build", "-o", binary, "./cmd").CombinedOutput()
	require.NoError(t, err, "%s", output)
	fixturePath, err := filepath.Abs(parityDataPath("fixtures.json"))
	require.NoError(t, err)
	fixtureBytes, err := os.ReadFile(fixturePath)
	require.NoError(t, err)
	run := func(extra ...string) ([]byte, error) {
		args := append([]string{"--corpus", fixturePath, "--output-dir", dir}, extra...)
		return exec.Command(binary, args...).CombinedOutput()
	}
	output, err = run()
	require.NoError(t, err, "%s", output)
	read := func(route string) artifactDocument {
		data, err := os.ReadFile(filepath.Join(dir, manifest.Generated.Files[route]))
		require.NoError(t, err)
		var document artifactDocument
		require.NoError(t, json.Unmarshal(data, &document))
		require.Equal(t, 1, document.Version)
		require.Equal(t, route, document.Route)
		require.Equal(t, fmt.Sprintf("%x", sha256.Sum256(fixtureBytes)), document.CorpusSHA256)
		var ids []string
		for _, record := range document.Cases {
			ids = append(ids, record.ID)
			if record.Accepted {
				require.NotEmpty(t, record.Value, "accepted record requires serialized value: %s", record.ID)
				require.Empty(t, record.Error)
			} else {
				require.NotEmpty(t, record.Error, "rejected record requires reason: %s", record.ID)
			}
		}
		require.ElementsMatch(t, manifest.CaseIDs, ids)
		return document
	}
	direct := read("go.direct")
	read("go.encoder")
	for _, id := range []string{"event.TEXT_MESSAGE_START.full", "event.TOOL_CALL_CHUNK.full", "event.CUSTOM.nested_null"} {
		var expected json.RawMessage
		for _, c := range corpus.Cases {
			if c.ID == id {
				expected = c.Expected
			}
		}
		for _, record := range direct.Cases {
			if record.ID == id {
				require.True(t, record.Accepted, "%s: %s", id, record.Error)
				require.NoError(t, compareJSON(expected, record.Value, false), id)
			}
		}
	}
	// Simulate a peer envelope solely to exercise the import boundary. The
	// next oracle slice supplies real Python/TypeScript producer artifacts.
	peer := direct
	peer.Route = "python.produced"
	var importedValue json.RawMessage
	for i := range peer.Cases {
		if peer.Cases[i].ID == "event.TEXT_MESSAGE_START.full" {
			var value map[string]any
			require.NoError(t, json.Unmarshal(peer.Cases[i].Value, &value))
			value["messageId"] = "from-peer-artifact"
			importedValue, err = json.Marshal(value)
			require.NoError(t, err)
			peer.Cases[i].Value = importedValue
		}
	}
	writePeer := func(document artifactDocument) {
		data, err := json.Marshal(document)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "python.produced.json"), data, 0600))
	}
	writePeer(peer)
	output, err = run("--source", "python")
	require.NoError(t, err, "%s", output)
	imported := read("go.from-python")
	for _, record := range imported.Cases {
		if record.ID == "event.TEXT_MESSAGE_START.full" {
			require.True(t, record.Accepted, record.Error)
			require.NoError(t, compareJSON(importedValue, record.Value, false), "import must consume peer bytes")
		}
	}
	for _, mutation := range []struct {
		name string
		edit func(*artifactDocument)
	}{
		{"missing case", func(d *artifactDocument) { d.Cases = d.Cases[:len(d.Cases)-1] }},
		{"duplicate case", func(d *artifactDocument) { d.Cases = append(d.Cases, d.Cases[0]) }},
		{"wrong digest", func(d *artifactDocument) { d.CorpusSHA256 = "wrong" }},
		{"wrong route", func(d *artifactDocument) { d.Route = "typescript.produced" }},
		{"accepted without value", func(d *artifactDocument) { d.Cases[0].Accepted = true; d.Cases[0].Value = nil }},
		{"accepted with error", func(d *artifactDocument) { d.Cases[0].Error = "contradictory status" }},
		{"accepted but unsupported", func(d *artifactDocument) { d.Cases[0].Unsupported = true }},
		{"rejected without reason", func(d *artifactDocument) {
			d.Cases[0].Accepted = false
			d.Cases[0].Value = nil
			d.Cases[0].Error = ""
		}},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			changed := peer
			changed.Cases = append([]artifactRecord(nil), peer.Cases...)
			mutation.edit(&changed)
			writePeer(changed)
			output, err := run("--source", "python")
			require.Error(t, err, "corrupt envelope accepted: %s", output)
		})
	}
	writePeer(peer)
	peerPath := filepath.Join(dir, "python.produced.json")
	data, err := os.ReadFile(peerPath)
	require.NoError(t, err)
	var missingStatus map[string]any
	require.NoError(t, json.Unmarshal(data, &missingStatus))
	delete(missingStatus["cases"].([]any)[0].(map[string]any), "accepted")
	data, err = json.Marshal(missingStatus)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(peerPath, data, 0600))
	output, err = run("--source", "python")
	require.Error(t, err, "missing status accepted: %s", output)
	require.NoError(t, os.Remove(peerPath))
	output, err = run("--source", "python")
	require.Error(t, err, "missing artifact accepted: %s", output)
	peer.Route = "typescript.produced"
	data, err = json.Marshal(peer)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "typescript.produced.json"), data, 0600))
	output, err = run("--source", "typescript")
	require.NoError(t, err, "%s", output)
	read("go.from-typescript")
}
