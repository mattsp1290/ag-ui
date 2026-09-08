#!/usr/bin/env bash
# Build fresh artifacts from all three SDKs and verify every directed route.
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
for runtime in go node pnpm uv; do
  command -v "$runtime" >/dev/null || { echo "Missing required runtime: $runtime" >&2; exit 1; }
done

parity_dir="$(mktemp -d "${TMPDIR:-/tmp}/ag-ui-parity.XXXXXX")"
trap 'rm -rf -- "$parity_dir"' EXIT
export AG_UI_PARITY_OUTPUT_DIR="$parity_dir"
export NX_DAEMON=false NX_SKIP_NX_CACHE=true
unset AG_UI_PARITY_PHASE

python_oracle() (
  cd "$repo_root/sdks/python"
  AG_UI_PARITY_PHASE="$1" uv run --locked python -m unittest discover -s tests -p test_go_parity.py -v
)

typescript_oracle() (
  cd "$repo_root"
  AG_UI_PARITY_PHASE="$1" pnpm nx run @ag-ui/core:test -- src/__tests__/go-parity.test.ts
)

cd "$repo_root"
pnpm nx run @ag-ui/core:build
(
  cd sdks/community/go
  go build -o "$parity_dir/go-parity" ./internal/parity/cmd
)
corpus="$repo_root/sdks/community/go/testdata/parity/fixtures.json"
"$parity_dir/go-parity" --corpus "$corpus" --output-dir "$parity_dir"
python_oracle produce
typescript_oracle produce
python_oracle consume
typescript_oracle consume
"$parity_dir/go-parity" --corpus "$corpus" --output-dir "$parity_dir" --source python
"$parity_dir/go-parity" --corpus "$corpus" --output-dir "$parity_dir" --source typescript
python_oracle verify
typescript_oracle verify
cd "$repo_root/sdks/community/go"
AG_UI_PARITY_PHASE=verify go test -count=1 ./internal/parity -run '^TestGeneratedArtifacts$'
