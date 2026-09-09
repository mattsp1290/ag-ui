#!/usr/bin/env bash
# Encode fresh SSE streams in all three SDKs and exercise the actual consumers.
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
for runtime in go node pnpm uv; do
  command -v "$runtime" >/dev/null || { echo "Missing required runtime: $runtime" >&2; exit 1; }
done
sse_dir="$(mktemp -d "${TMPDIR:-/tmp}/ag-ui-sse-parity.XXXXXX")"
trap 'rm -rf -- "$sse_dir"' EXIT
export AG_UI_SSE_PARITY_OUTPUT_DIR="$sse_dir"
export NX_DAEMON=false NX_SKIP_NX_CACHE=true
printf 'SSE corpus: %s\n' "$repo_root/sdks/community/go/testdata/parity/sse-scenarios.json"
printf 'Go SDK source: %s\n' "$repo_root/sdks/community/go"
(
  cd "$repo_root/sdks/community/go"
  go test -count=1 -v ./pkg/encoding/sse -run '^TestCrossLanguageSSEWrite$'
)
(
  cd "$repo_root/sdks/python"
  uv run --locked python -m unittest discover -s tests -p test_go_sse.py -v
)
(
  cd "$repo_root"
  pnpm nx run @ag-ui/client:test -- src/transform/__tests__/go-sse.test.ts
)
(
  cd "$repo_root/sdks/community/go"
  go test -count=1 -v ./pkg/client/sse -run '^TestCrossLanguageSSERead$'
)
