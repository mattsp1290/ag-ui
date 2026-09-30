# AG-UI Rust Client 0.2.0

`ag-ui-client` connects to AG-UI 1.0 agents over HTTP SSE. It depends on `ag-ui-core` 0.2.0. This is a breaking upgrade from 0.1.0; see [CHANGELOG.md](CHANGELOG.md).

`Agent::run_agent` returns `RunAgentResult` with `new_messages`, `new_state`, a JSON `result` (null when absent), and optional `outcome` and `usage`. `RunAgentParams::with_resume` supplies interrupt responses. Subscribers can observe lifecycle, text, tool, state, activity, reasoning, and subagent events. Legacy `THINKING_*` wire events are translated to reasoning events with warnings.

## Live examples

From `integrations/server-starter-all-features/python/examples`, install dependencies and start the Python starter at port 3001:

```bash
PORT=3001 uv run --no-sync dev
```

Then, from `sdks/community/rust`:

```bash
cargo run --example basic_agent
cargo run --example shared_state
```

`basic_agent` posts to `/agentic_chat` and prints the assistant message and its string ID. `shared_state` posts to `/shared_state` and prints its state snapshot, JSON Patch delta, and final patched recipe. Both examples default to `http://127.0.0.1:3001`; set `AG_UI_BASE_URL` to use another host or port.

See [examples](examples) and the [Rust SDK reference](/sdk/rust/overview).
