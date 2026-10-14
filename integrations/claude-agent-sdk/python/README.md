# ag-ui-claude-agent-sdk

Implementation of the AG-UI protocol for the Anthropic Claude Agent SDK (Python).

## Installation

```bash
pip install -e .
```

## Usage

The adapter manages the SDK lifecycle internally — just call `adapter.run(input_data)`:

```python
from ag_ui_claude_sdk import ClaudeAgentAdapter, add_claude_fastapi_endpoint

adapter = ClaudeAgentAdapter(name="my_agent", options={"model": "claude-haiku-4-5"})
add_claude_fastapi_endpoint(app=app, adapter=adapter, path="/my_agent")
```

## Features

- **Full lifecycle management** - Handles client pooling, message extraction, and event translation internally
- **Interrupt support** - Call `adapter.interrupt()` to stop a running query
- **Dynamic frontend tools** - Client-provided tools automatically added as MCP server with auto-granted permissions
- **Frontend tool halting** - Streams pause after frontend tool calls for client-side execution (human-in-the-loop)
- **Streaming tool arguments** - Real-time TOOL_CALL_ARGS emission as JSON arguments stream in
- **Bidirectional state sync** - Shared state management via ag_ui_update_state tool
- **Context injection** - Context and state injected into prompts for agent awareness
- **Event cleanup** - Hanging events (tool calls, reasoning blocks) automatically closed on stream end
- **Custom tools via MCP** - Define custom tools using Claude SDK's @tool decorator
- **Forwarded props** - Per-run option overrides with security whitelist

## Examples

The integration includes 5 example agents:

| Route                       | Description                    | Features                                         |
| --------------------------- | ------------------------------ | ------------------------------------------------ |
| `/agentic_chat`             | Basic conversational assistant | Simple chat                                      |
| `/backend_tool_rendering`   | Weather tool (backend MCP)     | Backend tool execution, tool rendering           |
| `/shared_state`             | Recipe collaboration           | Bidirectional state sync, ag_ui_update_state     |
| `/human_in_the_loop`        | Task planning with approval    | Frontend tools, step tracking, approval workflow |
| `/tool_based_generative_ui` | Frontend tool rendering        | Dynamic frontend tools, generative UI            |

## Running the Examples

```bash
# Install dependencies
cd integrations/claude-agent-sdk/python
pip install -e .

# Start server (port 8019)
cd examples
ANTHROPIC_API_KEY=sk-ant-xxx python server.py

# Start Dojo (in another terminal)
cd apps/dojo
pnpm dev
```

Visit **http://localhost:3000** and select **"Claude Agent SDK (Python)"**

## Session Persistence

Claude SDK maintains conversation state in the `.claude/` directory. For production deployments:

- **Development**: Sessions persist locally in `.claude/{session_id}/`
- **Production**: Mount `.claude/` as a persistent volume in your container
- **Resumption**: Pass `resume=<session_id>` via the options dict or `forwarded_props`

See [Claude SDK Hosting Guide](https://platform.claude.com/docs/en/agent-sdk/hosting) for deployment patterns.

## Links

- [Claude Agent SDK](https://platform.claude.com/docs/en/agent-sdk/python)
- [AG-UI Documentation](https://docs.ag-ui.com/)
- [AG-UI State Management](https://docs.ag-ui.com/concepts/state)

## Content and state preservation

The adapter emits AG-UI 1.0 content parts for supported tool-result media. It preserves every block in order, including bytes and MIME type; each projected part retains the original SDK block in `metadata.claude.block`. Parts without an AG-UI equivalent retain their JSON payload and an explicit `metadata.claude.unsupportedType`. Tool-result events and final message snapshots use the same projection and error indication. Single plain-text results remain strings, including their original whitespace.

The `claude:assistant_content` custom event retains complete assistant blocks received from the SDK, including reasoning/signatures and richer blocks that the text transcript cannot represent. Its value contains `messageId`, the SDK record `uuid` when available, `parentToolUseId`, and `content`. Streamed message IDs use the native message ID when supplied. Only `mcp__ag_ui__` tool names map to frontend names; external MCP names remain qualified to avoid collisions.

Input uploads remain in `RUN_STARTED.input` with their original metadata. Image and PDF data/URLs are forwarded to Claude; inline UTF-8 `text/plain` documents are decoded from base64, and document `metadata.filename` is forwarded as the document title. Audio/video inputs and unresolved provider file handles emit `claude:unsupported_input` diagnostics and are omitted from the model prompt. Recording those uploads is separate from model comprehension. Native SDK transcripts do not necessarily retain every AG-UI metadata field.

`input.state` is the complete state at the start of a run. The `ag_ui_update_state` tool shallow-merges `state_updates` object keys; nested objects and arrays replace the previous value at that key. The adapter emits the full merged `STATE_SNAPSHOT`, and invalid updates leave the prior state intact. The native state tool result is an acknowledgement, not a saved complete state snapshot. A native transcript with only partial updates cannot establish the missing initial state.

Frontend tool proxies require a result resolver armed for the original tool call by the deferred-tool/session recovery integration. The resolver returns the real frontend result to that call; absent resolution fails explicitly instead of saving a placeholder. Strings retain their bytes; supported structured results become MCP content. This content change must ship with the PNI-659 lifecycle companion. It does not repair historical placeholder results or make arbitrary historical approvals resumable.

Protocol regression tests exercise synthetic SDK streams and proxy callbacks without model calls. They do not establish released support, native persistence across restart, or end-to-end Intelligence replay; those require the lifecycle companion and product validation.
