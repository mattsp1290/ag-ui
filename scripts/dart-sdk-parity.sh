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
  rm -f -- "$repo_root/sdks/community/dart/.dart-parity-runner.dart"
  if [[ "$owns_output" == true ]]; then rm -rf -- "$output_dir"; fi
}
trap cleanup EXIT

export NX_DAEMON=false NX_SKIP_NX_CACHE=true
corpus_env="${AG_UI_DART_PARITY_CORPUS:-}"
if [[ -n "$corpus_env" ]]; then
  uses_supplemental=false
  corpus="$corpus_env"
  if [[ "$corpus" != /* ]]; then corpus="$repo_root/$corpus"; fi
  [[ -f "$corpus" ]] || { echo "parity corpus does not exist: $corpus" >&2; exit 1; }
else
  uses_supplemental=true
  corpus="$output_dir/dart-parity-corpus.json"
  if [[ ! -f "$corpus" ]]; then
    FROZEN="$repo_root/sdks/community/go/testdata/parity/fixtures.json" \
      SUPPLEMENTAL="$repo_root/sdks/community/dart/test/fixtures/parity_cases.json" \
      DEST="$corpus" python3 - <<'PY'
import json, os
from pathlib import Path

frozen = json.loads(Path(os.environ["FROZEN"]).read_text())
supplemental = json.loads(Path(os.environ["SUPPLEMENTAL"]).read_text())
destination = Path(os.environ["DEST"])
if frozen.get("version") != 1 or supplemental.get("version") != 1:
    raise SystemExit("parity corpora must use version 1")
cases = list(frozen.get("cases", []))
ids = {case["id"] for case in cases}
for case in supplemental.get("cases", []):
    if case.get("id") in ids:
        raise SystemExit(f"duplicate supplemental case {case.get('id')}")
    event_type = case.get("input", {}).get("type")
    if not isinstance(event_type, str) or not event_type:
        raise SystemExit(f"supplemental case {case.get('id')} lacks an event type")
    cases.append({**case, "kind": "event", "valid": True, "event_type": event_type})
    ids.add(case["id"])
if len(cases) != len(frozen.get("cases", [])) + len(supplemental.get("cases", [])):
    raise SystemExit("supplemental corpus contains duplicate IDs")
destination.write_text(json.dumps({"version": 1, "cases": cases}, separators=(",", ":")) + "\n")
PY
  fi
fi

python_oracle() (
  cd "$repo_root/sdks/python"
  env -u AG_UI_DART_PARITY_MODE -u AG_UI_DART_PARITY_OUTPUT_DIR \
    -u AG_UI_DART_PARITY_PHASE -u AG_UI_DART_PARITY_CORPUS \
    AG_UI_PARITY_OUTPUT_DIR="$output_dir" AG_UI_PARITY_PHASE="$1" \
    uv run --locked python -m unittest discover -s tests -p test_go_parity.py -v
)
python_dart_consumer() (
  cd "$repo_root/sdks/python"
  env -u AG_UI_PARITY_OUTPUT_DIR -u AG_UI_PARITY_PHASE \
    AG_UI_DART_PARITY_MODE=1 AG_UI_DART_PARITY_OUTPUT_DIR="$output_dir" \
    AG_UI_DART_PARITY_PHASE="$1" AG_UI_DART_PARITY_CORPUS="$corpus" \
    uv run --locked python -m unittest discover -s tests -p test_go_parity.py -v
)
typescript_oracle() (
  cd "$repo_root"
  env -u AG_UI_DART_PARITY_MODE -u AG_UI_DART_PARITY_OUTPUT_DIR \
    -u AG_UI_DART_PARITY_PHASE -u AG_UI_DART_PARITY_CORPUS \
    AG_UI_PARITY_OUTPUT_DIR="$output_dir" AG_UI_PARITY_PHASE="$1" \
    pnpm nx run @ag-ui/core:test -- src/__tests__/go-parity.test.ts
)
typescript_dart_consumer() (
  cd "$repo_root"
  env -u AG_UI_PARITY_OUTPUT_DIR -u AG_UI_PARITY_PHASE \
    AG_UI_DART_PARITY_MODE=1 AG_UI_DART_PARITY_OUTPUT_DIR="$output_dir" \
    AG_UI_DART_PARITY_PHASE="$1" AG_UI_DART_PARITY_CORPUS="$corpus" \
    pnpm nx run @ag-ui/core:test -- src/__tests__/go-parity.test.ts
)

dart_runner="$repo_root/sdks/community/dart/.dart-parity-runner.dart"
cat > "$dart_runner" <<'DART'
import 'dart:convert';
import 'dart:io';

import 'test/parity/artifact_test.dart' as parity;

Map<String, dynamic> readObject(String path) =>
    (jsonDecode(File(path).readAsStringSync()) as Map).cast<String, dynamic>();

void main(List<String> args) {
  if (args.length != 3) {
    throw ArgumentError('usage: runner.dart phase corpus output');
  }
  final phase = args[0];
  final corpus = parity.ParityCorpus(File(args[1]).readAsBytesSync());
  final output = Directory(args[2]);
  if (!output.existsSync()) throw StateError('output directory does not exist');
  switch (phase) {
    case 'produce':
      parity.writeArtifact(output, parity.produceDartArtifact(corpus, 'dart.direct', useEncoder: false));
      parity.writeArtifact(output, parity.produceDartArtifact(corpus, 'dart.encoder', useEncoder: true));
    case 'consume':
      for (final source in ['go', 'python', 'typescript']) {
        final route = source == 'go' ? 'go.direct' : '$source.produced';
        final peer = parity.parseArtifact(readObject('${output.path}/$route.json'), corpus, route);
        parity.writeArtifact(output, parity.consumeDartArtifact(corpus, peer, source));
      }
    case 'verify':
      for (final route in [
        'dart.direct', 'dart.encoder', 'dart.from-go',
        'dart.from-python', 'dart.from-typescript',
      ]) {
        parity.parseArtifact(readObject('${output.path}/$route.json'), corpus, route);
      }
    default:
      throw ArgumentError('unknown phase $phase');
  }
}
DART

dart_parity() (
  cd "$repo_root/sdks/community/dart"
  dart run "$dart_runner" "$1" "$corpus" "$output_dir"
)

build_peer_tools() {
  cd "$repo_root"
  pnpm nx run @ag-ui/core:build
  (
    cd sdks/community/go
    go build -o "$output_dir/go-parity" ./internal/parity/cmd
  )
}

if [[ "$phase" == all || "$phase" == produce || "$phase" == consume || "$phase" == verify ]]; then
  build_peer_tools
fi

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
  if [[ "$uses_supplemental" == false ]]; then
    (
      cd "$repo_root/sdks/community/go"
      AG_UI_PARITY_PHASE=verify AG_UI_PARITY_OUTPUT_DIR="$output_dir" \
        go test -count=1 ./internal/parity -run '^TestGeneratedArtifacts$'
    )
  fi
  CORPUS="$corpus" OUTPUT="$output_dir" python3 - <<'PY'
import hashlib, json, os
from pathlib import Path

corpus_path = Path(os.environ["CORPUS"])
output = Path(os.environ["OUTPUT"])
corpus = json.loads(corpus_path.read_text())
ids = [case["id"] for case in corpus["cases"]]
expected = {
    "go.direct", "go.encoder", "python.produced", "typescript.produced",
    "python.from-go", "typescript.from-go", "go.from-python", "go.from-typescript",
    "dart.direct", "dart.encoder", "dart.from-go", "dart.from-python",
    "dart.from-typescript", "go.from-dart", "python.from-dart", "typescript.from-dart",
}
for route in sorted(expected):
    document = json.loads((output / (route + ".json")).read_text())
    assert set(document) == {"version", "route", "corpus_sha256", "cases"}, route
    assert document["version"] == 1 and document["route"] == route, route
    assert document["corpus_sha256"] == hashlib.sha256(corpus_path.read_bytes()).hexdigest(), route
    records = document["cases"]
    assert [record["id"] for record in records] == ids, route
    for record in records:
        assert set(record) <= {"id", "accepted", "value", "error", "unsupported"}, route
        if record["accepted"]:
            assert set(record) == {"id", "accepted", "value"}, (route, record["id"])
        else:
            assert "error" in record and isinstance(record["error"], str) and record["error"], (route, record["id"])
            assert "value" not in record, (route, record["id"])
print(f"verified {len(expected)} routes x {len(ids)} cases")
PY
fi
