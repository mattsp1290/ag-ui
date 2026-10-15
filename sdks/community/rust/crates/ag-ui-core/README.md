# AG-UI Core 0.2.0

Canonical Rust types for AG-UI 1.0: seven message roles, multimodal content,
current event families, typed JSON Patch, run outcomes, usage, interrupts,
resume entries, opaque reasoning artifacts, and subagent attribution.

## Processing boundaries

Use [`processing::normalize_run_input`] on raw JSON before accepting a producer
request. It returns a canonical [`types::RunAgentInput`], `effective_state`, and
structured diagnostics. An absent or whole-null optional state has an effective
`{}` baseline. Required snapshots and nested nulls are preserved.

Use [`processing::normalize_event`] for tolerant consumer ingress. Unknown
closed members/union material are diagnosed and stripped where permitted;
malformed known values fail. Unknown events may produce no canonical event.
Open application JSON and legal patch-operation extension members survive.

Use [`processing::prepare_run_input`] for the non-mutating outgoing request
projection: validate known values, omit optional whole-null fields, exclude
activity history, and declare the consumer's own 1.0 version. Use
[`processing::normalize_producer_event`] for strict producer-output validation;
`ag-ui-encoder` invokes it before producing any bytes.

`TextMessageStartEvent.role` is optional, preserving an omitted wire role;
application reducers default omission to assistant. Each known JSON Patch
operation has an `extra` map for legal extension members. The serializer rejects
extras that would shadow that operation's typed reserved fields.

Serde `Deserialize` is a representation operation, not a full conformance claim.
The strict JSON Schema oracle, tolerant ingress, and producer egress are separate
test lanes. Diagnostics have stable categories and RFC 6901 paths, with bounded
sanitized details rather than payload logging.

Versions are numeric `MAJOR.MINOR`: absent/current declarations proceed; newer
minor and uninterpretable declarations warn; unsupported request majors fail
before admission. Retired 0.x/`THINKING_*` translation is outside this current-1.0
processing path.

## Limited replay

[`Replay`] is a convenience reducer, not a lifecycle verifier, canonical server
history, or persistence engine. Its supported subset is:

- Text start/content/end and explicitly identified text chunks
- Tool results and full message snapshot replacement
- State snapshots (including null) and atomic JSON Patch application
- Run/step boundaries as no-ops

Activity, reasoning, tool-call assembly, subagents, and custom/raw events return
an explicit unsupported-event error. Full client history uses role-aware
snapshot reconciliation; this reducer intentionally replaces the entire message
collection. `message_texts` is lossy display text, never a provider projection.
