# Parallel flight/dashboard regression traces (PNI-490)

The original three traces below are recorded protocol traces, not hand-authored
tool sequences. Both tools
belong to the same native assistant message and retain its `parentMessageId`.
Only the run boundaries and `search_flights` / `render_a2ui` events are retained;
transport debug metadata, import provenance, timestamps and unrelated tools are
removed. Call IDs, parent message IDs, argument chunks, result payloads and event
order are preserved.

- `langgraph-live.json`: a fresh provider-backed synthetic demo, generated through
  an aimock recorder on 2026-09-24 with LangGraph 1.1.6, LangChain 1.2.15,
  langchain-openai 1.1.9, copilotkit Python 0.1.94 and ag-ui-langgraph 0.0.41.
  The native SQLite checkpoint contains one assistant message with both parallel
  calls. The graph exposes the existing Beautiful Chat `search_flights` and a
  direct `render_a2ui` tool, with `parallel_tool_calls=True`. Two model requests
  were recorded (parallel tool turn, then completion).
- `langgraph-import.json`: that same saved native history, passed through the
  Intelligence LangGraph normalizer and shared conversation-event builder.
- `strands-ts-import.json`: the retained original Strands SDK 1.19.0 native
  SessionManager snapshot, passed through Intelligence PR1366's normalizer and
  the same event builder. This original source used a deterministic model and
  real Agent/FunctionTool/SessionManager execution; it is not a provider-generated
  Strands conversation.

The LangGraph model emitted exactly the original Strands fixture's flight and
render argument values. Both transformations use the custom
`copilotkit://app-dashboard-catalog`. The original Strands capture used generic
FunctionTool callbacks, so its tool-schema validation is not claimed equivalent
to the typed LangGraph tools. The regression concerns preserved sibling call
identity and result ownership, downstream of those schema differences.

Published CopilotKit 1.73.3 with A2UI middleware 0.0.10 maps the dashboard to the
flight call in all three traces, then suppresses the flight result. Each trace
must instead produce the dashboard on its render call and flight cards on their
own flight call. Existing recovery tests cover the legacy nested-render path
where adapters omit parent message IDs. Separate synthetic unit events exercise
out-of-order ordinary results and failure envelopes.

## Live sibling calls with distinct message IDs (PNI-679)

`strands-ts-live.json` retains all tool start/args/end/result events, in order,
from the October 16 deterministic Strands TypeScript SDK 1.20.0 capture used to
reproduce the failure in middleware 0.0.12. Run boundaries are retained; message
and state snapshots are omitted. The five sibling calls each have a distinct
`parentMessageId`, and all calls finish emitting arguments before results arrive.
The flight envelope includes both DL123 and AA456. No AWS access is needed to
replay this synthetic capture.

A parent message ID identifies an assistant message, not a parent tool call.
Calls with message identities remain independent. Legacy nested renders that
omit message identity still use the active outer call, even when its arguments
have ended. Without message identity, parallel versus nested calls remain
ambiguous; the legacy fallback is retained for compatibility with existing
recovery adapters. Nested calls with message identities need an explicit nesting
contract; different message IDs alone cannot distinguish them from siblings.
