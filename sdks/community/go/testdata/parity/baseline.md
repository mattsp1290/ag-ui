# Go SDK compatibility baseline

This fixture records the public API at pinned upstream `87e9d964dd2f1278cd90614ffbe170ac383208f5` and compares it with the older checkout `30a2a43653fa024ec7cb611c43c6a90dcfa0c368`. The three requested upstream changes are already ancestors of baseline `7ea5fccb28a9d6fc1dd297e351094da9a239f3fb`; no integration is required.

The historical source breaks are inherited from upstream metadata work: `ToolCall.Metadata` changes `ToolCall` from comparable to non-comparable, so old `==` and `map[types.ToolCall]` callers must migrate. Adding that field also breaks old unkeyed `ToolCall` literals. Disposable archive probes demonstrate both: old compile succeeds; pinned compile fails with `struct containing types.Metadata cannot be compared`, `invalid map key type`, and `too few values in struct literal`.

Baseline gates passed on the checked-out fork: `go test ./...` in `sdks/community/go` and `GOWORK=off go test ./...` in `sdks/community/go/example/server`. The fixture intentionally exercises constructor/options and public codec/client/event surfaces, camel-case precedence, snake-case aliases, legacy THINKING through `EventDecoder`, encrypted content, metadata nulls, usage zero, outcomes, and omission of absent optional fields. It contains no production changes.

The broader baseline gates have pre-existing failures. `go test -race ./...` fails in `pkg/client/sse` (`TestStream/logger_output`) with a data race between the test reading a `bytes.Buffer` and the client/logrus stream goroutine writing it; the run also reports an httptest server blocked in `Close` while a connection remains active. `go vet ./...` fails in `pkg/client/sse/client_stream_test.go:704,723,740,752` because context cancel functions are not used on every return path. These failures are assigned to the later streaming-lifecycle slice and must pass before final parity acceptance.

The fixture passes unchanged on disposable archives of both `7ea5fccb28a9d6fc1dd297e351094da9a239f3fb` and `87e9d964dd2f1278cd90614ffbe170ac383208f5`. Run it with `go test ./internal/parity` from the SDK module. These selected public-contract checks protect external SDK users; they do not prove compatibility with every unidentified application or arbitrary unkeyed struct literal. Go examples and user-owned Eino consumers may adapt their APIs; no feature flags or downstream pin changes are introduced.

## Reproduce the historical compile comparison

Run this from the repository root with both Git revisions available. The old revision exits 0; the pinned upstream revision intentionally exits 1. This probes source compatibility separately from the valid wire corpus.

```bash
probe_root=$(mktemp -d)
trap 'rm -rf "$probe_root"' EXIT
for revision in 30a2a43653fa024ec7cb611c43c6a90dcfa0c368 87e9d964dd2f1278cd90614ffbe170ac383208f5; do
  mkdir "$probe_root/$revision"
  git archive "$revision" sdks/community/go | tar -x -C "$probe_root/$revision"
  cat > "$probe_root/$revision/sdks/community/go/compatibility_probe_test.go" <<'GO'
package compatibility_test

import "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/types"

var a, b types.ToolCall
var _ = a == b
var _ = map[types.ToolCall]struct{}{}
var _ = types.ToolCall{"id", types.ToolCallTypeFunction, types.FunctionCall{}}
GO
  (cd "$probe_root/$revision/sdks/community/go" && GOWORK=off go test .)
done
```

The pinned compiler reports these diagnostic classes (paths and line numbers omitted):

```text
invalid operation: a == b (struct containing types.Metadata cannot be compared)
invalid map key type types.ToolCall
too few values in struct literal of type types.ToolCall
```

The merged API also adds `Metadata`/`MergeMetadata`, metadata and subagent attribution fields, three subagent event types with constructors/options/outcomes, and `TokenUsage`/`TokenCount` with terminal usage options. It adds an outcome marshaler that prevents success outcomes from emitting interrupt fields. The `pkg/` sources on the implementation base match the pinned upstream tree; the fork's 26 differing example files remain intact.
