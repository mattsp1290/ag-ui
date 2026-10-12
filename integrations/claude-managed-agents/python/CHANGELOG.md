# Changelog

## 0.2.0 — 2026-10-12

- `RUN_STARTED` now carries `protocol_version`; raises the `ag-ui-protocol` floor to `>=1.0.0`.
- Tool-result file sources are skipped with a warning instead of sent as URLs; dropped audio/video/user-media parts now warn via a module logger.
- Added optional `on_error` hook to the agent constructor for best-effort failures that were previously swallowed.
- Endpoint helpers forward FastAPI route kwargs (name, tags, summary, operation_id, dependencies, include_in_schema).
- Rejects empty/whitespace-only `threadId` with `invalid_thread_id` before creating a session.
- Session store and busy gate now share one scoped `managedAgentId:threadId` key, preventing cross-agent record bleed.
- Tool input schemas preserved in full (`$defs`, `additionalProperties`, `anyOf`/`oneOf`/`allOf`, `$ref`); previously only `properties`/`required` survived.
- `TOOL_CALL_RESULT` emitted only after the session accepts the result; split persistence into result/follow-up callbacks.
- Parked tool-call ids recorded before confirmation and forgotten on interrupt, avoiding permanently parked sessions.
- At most one terminal event per run; failure messages replaced with stable codes to avoid leaking session ids, paths, or credentials.
- Gated outbound calls now cancellable via the run signal; entity decoding resolves each entity once and substitutes U+FFFD for lone surrogates.
- Session tool-list fingerprint covers the full held tool set, so Console-side tool edits re-sync.
- Dynamic tool definition changes (removals, schema/description edits) now update the Managed Agents override.

### Breaking changes

- `ag-ui-protocol` floor raised to `>=1.0.0`; validators now sourced from `@ag-ui/core/schemas` with part renames.
- Tool-result file sources are no longer sent as URLs; they are dropped with a warning.
- Runs with empty/whitespace-only `threadId` now fail with `invalid_thread_id` instead of sharing a session.
- Session store key changed to a scoped `managedAgentId:threadId`; `SessionStore`/`ISessionStore` signatures changed.
- Failure events now carry stable messages plus machine-readable codes instead of underlying exception text.
- `TOOL_CALL_RESULT` is emitted only after delivery is confirmed, changing event ordering on failure paths.
- Default in-memory store now copies records on read and write; re-verify custom stores.
