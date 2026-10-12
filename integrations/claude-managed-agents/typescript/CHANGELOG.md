# Changelog

## 0.0.2 — 2026-10-12

- RUN_STARTED now carries `protocolVersion=PROTOCOL_VERSION`.
- Warns when user-message media or audio/video tool-result parts are dropped, including media-only newest user messages.
- Adds an optional `onError` hook for best-effort failures that were previously swallowed; the hook is now safe for async/rejecting handlers.
- Tool input schema conversion preserves the full schema (`$defs`, `additionalProperties`, `description`, `anyOf`/`oneOf`/`allOf`, top-level `$ref`).
- Dynamic custom-tool definitions sync to the Managed Agents override, including removals and schema/description changes; session tool-list fingerprint now covers the base tools a session actually holds.
- Tool-result delivery is confirmed into the session before emitting `TOOL_CALL_RESULT`; failed delivery is reported instead of falsely reporting success.
- Parked tool-call ids are preserved when a turn later fails and forgotten when a session is interrupted.
- Emits at most one terminal event per run; the first `RUN_ERROR`/`RUN_FINISHED` wins.
- Rejects blank/whitespace-only `threadId` with `invalid_thread_id` before creating a session.
- Session store and busy gate now share one scoped key, preventing agents that share a store from reading each other's sessions.
- Gated outbound calls take the run signal so a stalled API call no longer pins the gate open; Python turn timeout now distinguishes inner send bounds from the turn deadline.
- Error messages from SDK/session-store/API exceptions are no longer relayed to clients; stable codes are reported instead.
- Entity decoding resolves each match once and only in escaping contexts; lone surrogate entities become U+FFFD.
- `clone()` now preserves AbstractAgent state (thread id, messages, state, subscribers, middleware).
- Default in-memory session store copies records on read and write, no longer aliasing mutable state.
- Adds `abandoned_backend_tool` reporting for backend tool handlers abandoned on teardown.
- `@ag-ui/core` and `@ag-ui/client` peer dependencies raised to `>=1.0.0`; adapter updated to 1.0 core schemas and part renames.

### Breaking changes

- `@ag-ui/core` and `@ag-ui/client` peers now require `>=1.0.0`; adapter follows 1.0 part renames and `@ag-ui/core/schemas` validators.
- Runs with empty/whitespace-only `threadId` now fail with `invalid_thread_id` instead of sharing one session key.
- Client-facing error messages for run/tool-result/tool-confirmation failures are replaced with stable messages plus machine-readable codes.
- Session store and busy gate are rekeyed by a scoped session-store identity, changing which runs serialize and which records are shared.
- `TOOL_CALL_RESULT` is now emitted only after the session accepts the result, changing event ordering on delivery failure.
- `SessionRecord` is copied on read/write; code relying on in-place aliasing of stored records must be re-verified.
- Example server entry points now run only as the process entry point, changing import side effects.
