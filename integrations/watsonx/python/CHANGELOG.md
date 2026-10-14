# Changelog

## 0.0.2 — 2026-10-14

- Content parts are flattened to their text parts before sending to watsonx orchestrate; non-text parts are dropped with a warning instead of leaking base64 data and file handles via `json.dumps`.
- Tool message content passes through to `TOOL_CALL_RESULT` unchanged (string or content-part content allowed).
- `RUN_STARTED` now carries `protocol_version=PROTOCOL_VERSION`.
- FastAPI endpoint helpers now forward route kwargs (name, tags, summary, operation_id, dependencies, include_in_schema).
- FastAPI helpers imported lazily only when used.
- Added Changelog project URL to published package metadata.
- Added MIT LICENSE file and license metadata.

### Breaking changes

- Requires `ag-ui-protocol >=1.0.0`.
- Prompt content serialization changed: non-text content parts are now dropped rather than JSON-encoded; verify prompts that relied on prior behavior.
