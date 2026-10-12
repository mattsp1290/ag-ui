# Changelog

## 0.0.2 — 2026-10-12

- RUN_STARTED now sets `ProtocolVersion` to `AGUIProtocol.Version` directly.
- Emits trace warnings for each non-text part dropped from a user message or flattened from a tool result.
- Media-only user messages now warn about dropped parts instead of failing silently with `empty_run`.
- Validators sourced from `@ag-ui/core/schemas`; aligned with 1.0 model part renames.
- File sources with provider handles are dropped with a warning rather than sent.

### Breaking changes

- Adapter aligned with 1.0 models: part renames and tool-result content flattening change emitted output; re-verify message handling.
- Dropped parts are now named by wire type and warned; previously silent in some paths.
