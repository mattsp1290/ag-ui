#!/usr/bin/env bash
# Build and verify the shared parity corpus with Dart as a peer producer.
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
for runtime in dart go node pnpm uv python3; do
  command -v "$runtime" >/dev/null || {
    echo "Missing required runtime: $runtime" >&2
    exit 1
  }
done

mode="dart"
if [[ "${1:-}" == "--mode" ]]; then
  mode="${2:?--mode requires dart}"
  shift 2
fi
if [[ $# -ne 0 || "$mode" != "dart" ]]; then
  echo "usage: $0 [--mode dart]" >&2
  exit 2
fi

phase="${AG_UI_DART_PARITY_PHASE:-all}"
case "$phase" in
  all|produce|consume|verify) ;;
  *) echo "AG_UI_DART_PARITY_PHASE must be all, produce, consume, or verify" >&2; exit 2 ;;
esac

if [[ -n "${AG_UI_DART_PARITY_OUTPUT_DIR:-}" ]]; then
  output_dir="$AG_UI_DART_PARITY_OUTPUT_DIR"
  if [[ "$output_dir" != /* ]]; then output_dir="$repo_root/$output_dir"; fi
  [[ -d "$output_dir" ]] || { echo "caller-owned output directory does not exist: $output_dir" >&2; exit 1; }
  owns_output=false
else
  output_dir="$(mktemp -d "${TMPDIR:-/tmp}/ag-ui-dart-parity.XXXXXX")"
  owns_output=true
fi
cleanup() {
  if [[ "$owns_output" == true ]]; then rm -rf -- "$output_dir"; fi
}
trap cleanup EXIT

export NX_DAEMON=false NX_SKIP_NX_CACHE=true
corpus_env="${AG_UI_DART_PARITY_CORPUS:-}"
if [[ -n "$corpus_env" ]]; then
  corpus="$corpus_env"
  if [[ "$corpus" != /* ]]; then corpus="$repo_root/$corpus"; fi
  [[ -f "$corpus" ]] || { echo "parity corpus does not exist: $corpus" >&2; exit 1; }
else
  corpus="$output_dir/dart-parity-corpus.json"
  if [[ ! -f "$corpus" ]]; then
    (
      cd "$repo_root/sdks/community/dart"
      dart run tool/dart_parity.dart resolve \
        "$repo_root/sdks/community/go/testdata/parity/fixtures.json" \
        "$repo_root/sdks/community/dart/test/fixtures/parity_cases.json" \
        "$corpus"
    )
  fi
fi
frozen_corpus="$repo_root/sdks/community/go/testdata/parity/fixtures.json"
is_frozen_corpus=false
if cmp -s -- "$corpus" "$frozen_corpus"; then is_frozen_corpus=true; fi

python_dart_consumer() (
  cd "$repo_root/sdks/python"
  env -u AG_UI_PARITY_OUTPUT_DIR -u AG_UI_PARITY_PHASE \
    AG_UI_DART_PARITY_MODE=1 AG_UI_DART_PARITY_OUTPUT_DIR="$output_dir" \
    AG_UI_DART_PARITY_PHASE="$1" AG_UI_DART_PARITY_CORPUS="$corpus" \
    uv run --locked python -m unittest discover -s tests -p test_go_parity.py -v
)
typescript_dart_consumer() (
  cd "$repo_root"
  env -u AG_UI_PARITY_OUTPUT_DIR -u AG_UI_PARITY_PHASE \
    AG_UI_DART_PARITY_MODE=1 AG_UI_DART_PARITY_OUTPUT_DIR="$output_dir" \
    AG_UI_DART_PARITY_PHASE="$1" AG_UI_DART_PARITY_CORPUS="$corpus" \
    pnpm nx run @ag-ui/core:test -- src/__tests__/go-parity.test.ts
)

dart_parity() (
  cd "$repo_root/sdks/community/dart"
  dart run tool/dart_parity.dart "$1" "$corpus" "$output_dir"
)

build_peer_tools() {
  cd "$repo_root"
  pnpm nx run @ag-ui/core:build
  (
    cd sdks/community/go
    go build -o "$output_dir/go-parity" ./internal/parity/cmd
  )
}

build_peer_tools

if [[ "$phase" == all || "$phase" == produce ]]; then
  "$output_dir/go-parity" --corpus "$corpus" --output-dir "$output_dir"
  python_dart_consumer produce
  typescript_dart_consumer produce
  dart_parity produce
fi

if [[ "$phase" == all || "$phase" == consume ]]; then
  dart_parity consume
  "$output_dir/go-parity" --corpus "$corpus" --output-dir "$output_dir" --source python
  "$output_dir/go-parity" --corpus "$corpus" --output-dir "$output_dir" --source typescript
  "$output_dir/go-parity" --corpus "$corpus" --output-dir "$output_dir" --source dart
  python_dart_consumer consume
  typescript_dart_consumer consume
fi

if [[ "$phase" == all || "$phase" == verify ]]; then
  python_dart_consumer verify
  typescript_dart_consumer verify
  dart_parity verify
  if [[ "$is_frozen_corpus" == true ]]; then
    (
      cd "$repo_root/sdks/community/go"
      AG_UI_PARITY_PHASE=verify AG_UI_PARITY_OUTPUT_DIR="$output_dir" \
        go test -count=1 ./internal/parity -run '^TestGeneratedArtifacts$'
    )
  fi
  (
    replay_dir="$(mktemp -d "${TMPDIR:-/tmp}/ag-ui-dart-go-replay.XXXXXX")"
    trap 'rm -rf -- "$replay_dir"' EXIT
    cp -- "$output_dir/dart.encoder.json" "$replay_dir/dart.encoder.json"
    "$output_dir/go-parity" --corpus "$corpus" --output-dir "$replay_dir" --source dart
    cmp -s -- "$output_dir/go.from-dart.json" "$replay_dir/go.from-dart.json" || {
      echo "go.from-dart does not match a fresh Go consumer replay" >&2
      exit 1
    }
  )
  sse_dir="$(mktemp -d "${TMPDIR:-/tmp}/ag-ui-dart-go-sse.XXXXXX")"
  (
    trap 'rm -rf -- "$sse_dir"' EXIT
    cd "$repo_root/sdks/community/go"
    AG_UI_SSE_PARITY_OUTPUT_DIR="$sse_dir" \
      go test -count=1 ./pkg/encoding/sse -run '^TestCrossLanguageSSEWrite$'
    cd "$repo_root/sdks/community/dart"
    AG_UI_DART_PARITY_GO_SSE_DIR="$sse_dir" \
      dart test test/parity/live_sse_test.dart \
        --name 'actual Go-produced SSE matches the pinned scenarios'
  )
  CORPUS="$corpus" OUTPUT="$output_dir" \
    MANIFEST="$repo_root/sdks/community/dart/test/fixtures/parity_manifest.json" \
    python3 - <<'PY'
import hashlib, json, os
from pathlib import Path

corpus_path = Path(os.environ["CORPUS"])
output = Path(os.environ["OUTPUT"])
manifest = json.loads(Path(os.environ["MANIFEST"]).read_text())
corpus = json.loads(corpus_path.read_text())
ids = [case["id"] for case in corpus["cases"]]
kind_by_id = {case["id"]: case["kind"] for case in corpus["cases"]}
route_files = manifest["dart_interop"]["routes"]
expected = set(route_files)
assert len(route_files) == 16 and len(set(route_files.values())) == 16
documents = {}
for route in sorted(expected):
    document = json.loads((output / route_files[route]).read_text())
    documents[route] = document
    assert set(document) == {"version", "route", "corpus_sha256", "cases"}, route
    assert document["version"] == 1 and document["route"] == route, route
    assert document["corpus_sha256"] == hashlib.sha256(corpus_path.read_bytes()).hexdigest(), route
    records = document["cases"]
    assert [record["id"] for record in records] == ids, route
    for record in records:
        assert set(record) <= {"id", "accepted", "value", "error", "unsupported"}, route
        assert type(record.get("accepted")) is bool, (route, record.get("id"))
        if "unsupported" in record:
            assert type(record["unsupported"]) is bool, (route, record.get("id"))
        if record["accepted"]:
            assert set(record) == {"id", "accepted", "value"}, (route, record["id"])
            if record["value"] is None:
                assert kind_by_id[record["id"]] == "mapper", (route, record["id"], "null value")
        else:
            assert "error" in record and isinstance(record["error"], str) and record["error"], (route, record["id"])
            assert "value" not in record, (route, record["id"])

consumer_sources = {
    "dart.from-go": "go.direct",
    "dart.from-python": "python.produced",
    "dart.from-typescript": "typescript.produced",
    "go.from-dart": "dart.encoder",
    "python.from-dart": "dart.encoder",
    "typescript.from-dart": "dart.encoder",
}
overrides = {
    (entry["route"], entry["case_id"]): entry
    for entry in manifest["dart_interop"].get("consumer_expectations", [])
}
for route, source_route in consumer_sources.items():
    source = {record["id"]: record for record in documents[source_route]["cases"]}
    for record in documents[route]["cases"]:
        parent = source[record["id"]]
        override = overrides.get((route, record["id"]))
        expected_accepted = override.get("accepted") if override else parent["accepted"]
        assert record["accepted"] == expected_accepted, (route, record["id"], "accepted")
        if record["accepted"]:
            expected_value = override["expected"] if override else parent["value"]
            assert record["value"] == expected_value, (route, record["id"], "value")
print(f"verified {len(expected)} routes x {len(ids)} cases")
PY
fi
